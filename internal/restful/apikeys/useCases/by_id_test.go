package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/mocks"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestApiKeyByID(t *testing.T) {
	t.Run("Quando lê a chave.", func(t *testing.T) { whenGetSucceeds(t) })
	t.Run("Quando a chave é de outro membro.", func(t *testing.T) { whenKeyBelongsToAnotherMember(t) })
	t.Run("Quando renomeia com PATCH e PUT.", func(t *testing.T) { whenUpdateAndUpgradeRename(t) })
	t.Run("Quando o nome novo é inválido.", func(t *testing.T) { whenUpdateNameIsInvalid(t) })
	t.Run("Quando a chave apagada é alterada.", func(t *testing.T) { whenDeletedKeyIsChanged(t) })
	t.Run("Quando rotaciona a chave.", func(t *testing.T) { whenRotateSucceeds(t) })
	t.Run("Quando a rotação falha no banco.", func(t *testing.T) { whenRotateFailsInDb(t) })
	t.Run("Quando o membro da rotação está suspenso.", func(t *testing.T) { whenRotateMemberIsSuspended(t) })
}

// Functions

type byIDRig struct {
	keys    *mocks.ApiKeyRepo
	members *member_mocks.MemberRepo
	guard   *member_mocks.Guard
	tx      *audit_mocks.Tx
	owner   uuid.UUID
	current entities.ApiKey
}

// newByIDRig monta o cenário em que o guard resolve o dono para owner e a
// chave pedida pertence a ele.
func newByIDRig() *byIDRig {
	rig := &byIDRig{keys: &mocks.ApiKeyRepo{}, members: member_mocks.NewRepo(), guard: &member_mocks.Guard{}, tx: &audit_mocks.Tx{}, owner: uuid.NewV7()}
	rig.current = mocks.NewApiKey()
	rig.current.MemberID = rig.owner
	rig.guard.PerformFunc = func(context.Context, entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{Owner: rig.owner}
	}
	rig.keys.FindByIDFunc = func(_ context.Context, _, id uuid.UUID) (entities.ApiKey, error) {
		key := rig.current
		key.ID = id
		return key, nil
	}
	return rig
}

func requester() entities.ApiHeader { return member_mocks.NewUserRequester() }

func whenGetSucceeds(t *testing.T) {
	rig := newByIDRig()
	id := uuid.NewV7()

	output := useCases.NewGetApiKeyByIDUseCase(rig.keys, rig.guard).Perform(context.Background(),
		useCases.GetInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: id})

	assert.Nil(t, output.Error)
	assert.Equal(t, id, output.Data.ID)
	assert.Equal(t, []entities.Action{entities.ActionApiKeyRead}, rig.guard.Actions())
}

func whenKeyBelongsToAnotherMember(t *testing.T) {
	rig := newByIDRig()
	rig.current.MemberID = uuid.NewV7()
	input := useCases.GetInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7()}

	get := useCases.NewGetApiKeyByIDUseCase(rig.keys, rig.guard).Perform(context.Background(), input)
	update := useCases.NewUpdateApiKeyByIDUseCase(rig.keys, rig.guard, rig.tx).Perform(context.Background(),
		useCases.UpdateInput{Requester: input.Requester, Application: input.Application, Member: input.Member, ID: input.ID, Name: "x"})
	rotate := useCases.NewRotateApiKeyByIDUseCase(rig.keys, rig.members, rig.guard, rig.tx, "https://vault.example.com").Perform(context.Background(),
		useCases.RotateInput{Requester: input.Requester, Application: input.Application, Member: input.Member, ID: input.ID})

	for name, e := range map[string]*entities.Error{"get": get.Error, "update": update.Error, "rotate": rotate.Error} {
		if assert.NotNil(t, e, name) {
			assert.Equal(t, http.StatusNotFound, e.Status, name+": o id de uma chave alheia não revela nada")
		}
	}
	assert.Empty(t, rig.keys.Renamed)
	assert.Empty(t, rig.keys.Created)
}

func whenUpdateAndUpgradeRename(t *testing.T) {
	rig := newByIDRig()
	input := useCases.UpdateInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7()}

	keep := useCases.NewUpdateApiKeyByIDUseCase(rig.keys, rig.guard, rig.tx).Perform(context.Background(), input)
	required := useCases.NewUpgradeApiKeyByIDUseCase(rig.keys, rig.guard, rig.tx).Perform(context.Background(), input)
	input.Name = " deploy "
	renamed := useCases.NewUpdateApiKeyByIDUseCase(rig.keys, rig.guard, rig.tx).Perform(context.Background(), input)

	assert.Nil(t, keep.Error, "PATCH sem nome mantém o atual")
	assert.Equal(t, rig.current.Name, keep.Data.Name)
	if assert.NotNil(t, required.Error, "PUT exige o nome") {
		assert.Equal(t, entities.ErrTypeValidation, required.Error.Type)
	}
	assert.Nil(t, renamed.Error)
	assert.Equal(t, "deploy", renamed.Data.Name)
	if assert.Len(t, rig.keys.Renamed, 1) {
		assert.Equal(t, "deploy", rig.keys.Renamed[0].Name)
	}
	assert.Equal(t, []string{entities.AuditApiKeyUpdated}, rig.tx.Actions())
}

func whenUpdateNameIsInvalid(t *testing.T) {
	rig := newByIDRig()

	output := useCases.NewUpgradeApiKeyByIDUseCase(rig.keys, rig.guard, rig.tx).Perform(context.Background(),
		useCases.UpdateInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7(), Name: strings.Repeat("a", 101)})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeValidation, output.Error.Type)
	}
	assert.Empty(t, rig.keys.Renamed)
}

func whenDeletedKeyIsChanged(t *testing.T) {
	rig := newByIDRig()
	now := time.Now()
	rig.current.DeletedAt = &now

	update := useCases.NewUpdateApiKeyByIDUseCase(rig.keys, rig.guard, rig.tx).Perform(context.Background(),
		useCases.UpdateInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7(), Name: "x"})
	rotate := useCases.NewRotateApiKeyByIDUseCase(rig.keys, rig.members, rig.guard, rig.tx, "").Perform(context.Background(),
		useCases.RotateInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7()})

	for name, e := range map[string]*entities.Error{"update": update.Error, "rotate": rotate.Error} {
		if assert.NotNil(t, e, name) {
			assert.Equal(t, http.StatusNotFound, e.Status, name)
		}
	}
	assert.Empty(t, rig.tx.Entries)
}

func whenRotateSucceeds(t *testing.T) {
	rig := newByIDRig()
	rig.current.Type = entities.KeyPublishable
	now := time.Now().UTC()
	useCase := useCases.NewRotateApiKeyByIDUseCase(rig.keys, rig.members, rig.guard, rig.tx, "https://vault.example.com/")
	useCase.Now = func() time.Time { return now }
	input := useCases.RotateInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7()}

	output := useCase.Perform(context.Background(), input)

	require.Nil(t, output.Error)
	assert.True(t, strings.HasPrefix(output.Data.Key, entities.PublicKeyPrefix), "a nova chave mantém o tipo")
	assert.Equal(t, rig.current.Name, output.Data.Name)
	assert.NotEqual(t, input.ID, output.Data.ID)
	assert.Equal(t, "https://vault.example.com/"+output.Data.UriID, output.Data.URI)
	assert.Len(t, rig.keys.Created, 1)
	assert.True(t, rig.keys.Expired[input.ID].Equal(now.Add(entities.RotationOverlap)), "a antiga vence em 72h")
	assert.Equal(t, []entities.Action{entities.ActionApiKeyCreate}, rig.guard.Actions())
	if assert.Len(t, rig.tx.Entries, 1) {
		assert.Equal(t, entities.AuditApiKeyRotated, rig.tx.Entries[0].Action)
		assert.NotContains(t, string(rig.tx.Entries[0].Metadata), output.Data.Key, "a trilha nunca leva a chave")
	}
}

func whenRotateFailsInDb(t *testing.T) {
	rig := newByIDRig()
	rig.keys.ExpireFunc = func(context.Context, uuid.UUID, uuid.UUID, time.Time) error { return errors.New("segredo-interno") }

	output := useCases.NewRotateApiKeyByIDUseCase(rig.keys, rig.members, rig.guard, rig.tx, "").Perform(context.Background(),
		useCases.RotateInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
	assert.Empty(t, output.Data.Key, "se falhou, a chave nova não vai para o cliente")
}

func whenRotateMemberIsSuspended(t *testing.T) {
	rig := newByIDRig()
	rig.members.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		member := member_mocks.NewMember(entities.RoleDeveloper)
		member.Status = entities.MemberSuspended
		return member, nil
	}

	output := useCases.NewRotateApiKeyByIDUseCase(rig.keys, rig.members, rig.guard, rig.tx, "").Perform(context.Background(),
		useCases.RotateInput{Requester: requester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusConflict, output.Error.Status)
	}
	assert.Empty(t, rig.keys.Created)
}
