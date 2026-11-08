package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/token/mocks"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
	"github.com/murkio-dev/vault/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestCreateToken(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenCreateBuildSucceeds(t) })
	t.Run("Quando cria o token.", func(t *testing.T) { whenCreateSucceeds(t) })
	t.Run("Quando o nome ou o vencimento são inválidos.", func(t *testing.T) { whenCreateInputIsInvalid(t) })
	t.Run("Quando o membro está suspenso.", func(t *testing.T) { whenCreateMemberIsSuspended(t) })
	t.Run("Quando o membro não existe.", func(t *testing.T) { whenCreateMemberIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenCreateIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenCreateFailsInDb(t) })
	t.Run("Quando a auditoria falha.", func(t *testing.T) { whenCreateAuditFails(t) })
}

// Functions

type createRig struct {
	tokens  *mocks.TokenRepo
	members *member_mocks.MemberRepo
	guard   *member_mocks.Guard
	tx      *audit_mocks.Tx
	useCase useCases.CreateTokenUseCase
}

func newCreateRig() *createRig {
	rig := &createRig{tokens: &mocks.TokenRepo{}, members: member_mocks.NewRepo(), guard: &member_mocks.Guard{}, tx: &audit_mocks.Tx{}}
	rig.useCase = useCases.NewCreateTokenUseCase(rig.tokens, rig.members, rig.guard, rig.tx, "https://vault.example.com/")
	return rig
}

func createInput() useCases.CreateInput {
	return useCases.CreateInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Name: utils.GenRandomTokenName()}
}

func whenCreateBuildSucceeds(t *testing.T) {
	rig := newCreateRig()

	assert.Equal(t, rig.tokens, rig.useCase.Creator)
	assert.Equal(t, rig.members, rig.useCase.MemberFinder)
	assert.Equal(t, rig.guard, rig.useCase.Guard)
	assert.Equal(t, rig.tx, rig.useCase.Tx)
	assert.Equal(t, "https://vault.example.com", rig.useCase.PublicURL, "a barra final sai")
}

func whenCreateSucceeds(t *testing.T) {
	rig := newCreateRig()
	input := createInput()

	output := rig.useCase.Perform(context.Background(), input)

	assert.Nil(t, output.Error)
	assert.True(t, strings.HasPrefix(output.Data.Token, entities.SDKTokenPrefix))
	assert.Equal(t, output.Data.Token[:entities.TokenDisplayLength], output.Data.Prefix)
	assert.Equal(t, "https://vault.example.com/"+output.Data.UriID, output.Data.URI)
	assert.Equal(t, []entities.Action{entities.ActionTokenCreate}, rig.guard.Actions())
	if assert.Len(t, rig.tokens.Created, 1) {
		assert.Equal(t, entities.HashToken(output.Data.Token), rig.tokens.Created[0].TokenHash, "o banco recebe o hash")
		assert.Equal(t, input.Application, rig.tokens.Application(), "grava no escopo da aplicação")
	}
	if assert.Len(t, rig.tx.Entries, 1) {
		assert.Equal(t, entities.AuditTokenCreated, rig.tx.Entries[0].Action)
		assert.NotContains(t, string(rig.tx.Entries[0].Metadata), output.Data.Token, "a trilha nunca leva o token")
	}
}

func whenCreateInputIsInvalid(t *testing.T) {
	rig := newCreateRig()
	past, far := time.Now().Add(-time.Hour), time.Now().Add(20*365*24*time.Hour)

	for name, mutate := range map[string]func(*useCases.CreateInput){
		"nome vazio":        func(i *useCases.CreateInput) { i.Name = "  " },
		"nome com quebra":   func(i *useCases.CreateInput) { i.Name = "a\nb" },
		"nome longo":        func(i *useCases.CreateInput) { i.Name = strings.Repeat("a", 101) },
		"vence no passado":  func(i *useCases.CreateInput) { i.ExpiresAt = &past },
		"vence muito longe": func(i *useCases.CreateInput) { i.ExpiresAt = &far },
	} {
		input := createInput()
		mutate(&input)

		output := rig.useCase.Perform(context.Background(), input)

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeValidation, output.Error.Type, name)
		}
	}
	assert.Empty(t, rig.tokens.Created)
}

func whenCreateMemberIsSuspended(t *testing.T) {
	rig := newCreateRig()
	rig.members.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		member := member_mocks.NewMember(entities.RoleDeveloper)
		member.Status = entities.MemberSuspended
		return member, nil
	}

	output := rig.useCase.Perform(context.Background(), createInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusConflict, output.Error.Status)
	}
	assert.Empty(t, rig.tokens.Created)
}

func whenCreateMemberIsMissing(t *testing.T) {
	rig := newCreateRig()
	rig.members.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		return entities.Member{}, sql.ErrNoRows
	}

	output := rig.useCase.Perform(context.Background(), createInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
}

func whenCreateIsDenied(t *testing.T) {
	rig := newCreateRig()
	rig.guard.Deny(entities.ForbiddenError())

	output := rig.useCase.Perform(context.Background(), createInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, rig.tokens.Created)
	assert.Empty(t, output.Data.Token)
}

func whenCreateFailsInDb(t *testing.T) {
	rig := newCreateRig()
	rig.tokens.CreateFunc = func(context.Context, entities.AccessToken) (entities.AccessToken, error) {
		return entities.AccessToken{}, errors.New("segredo-interno")
	}

	output := rig.useCase.Perform(context.Background(), createInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
	assert.Empty(t, output.Data.Token, "se falhou, o token não vai para o cliente")
}

func whenCreateAuditFails(t *testing.T) {
	rig := newCreateRig()
	rig.tx.AuditErr = errors.New("disk full")

	output := rig.useCase.Perform(context.Background(), createInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
	}
	assert.Empty(t, output.Data.Token, "sem trilha, o token não nasce")
}
