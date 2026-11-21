package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/mocks"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestDeleteApiKey(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenRevokeBuildSucceeds(t) })
	t.Run("Quando revoga o token.", func(t *testing.T) { whenRevokeSucceeds(t) })
	t.Run("Quando o token já estava revogado.", func(t *testing.T) { whenRevokeIsRepeated(t) })
	t.Run("Quando o token é de outro membro.", func(t *testing.T) { whenRevokeTokenBelongsToAnotherMember(t) })
	t.Run("Quando o token não existe.", func(t *testing.T) { whenRevokeTokenIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenRevokeIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenRevokeFailsInDb(t) })
}

// Functions

type revokeRig struct {
	tokens  *mocks.ApiKeyRepo
	guard   *member_mocks.Guard
	tx      *audit_mocks.Tx
	owner   uuid.UUID
	useCase useCases.DeleteApiKeyByIDUseCase
}

// newRevokeRig monta o cenário em que o guard resolve o dono para owner e o
// token pertence a ele.
func newRevokeRig() *revokeRig {
	rig := &revokeRig{tokens: &mocks.ApiKeyRepo{}, guard: &member_mocks.Guard{}, tx: &audit_mocks.Tx{}, owner: uuid.NewV7()}
	rig.guard.PerformFunc = func(context.Context, entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{Owner: rig.owner}
	}
	rig.tokens.FindByIDFunc = func(_ context.Context, _, id uuid.UUID) (entities.ApiKey, error) {
		token := mocks.NewApiKey()
		token.ID, token.MemberID = id, rig.owner
		return token, nil
	}
	rig.useCase = useCases.NewDeleteApiKeyByIDUseCase(rig.tokens, rig.guard, rig.tx)
	return rig
}

func revokeInput() useCases.DeleteInput {
	return useCases.DeleteInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: entities.SelfOwner, ID: uuid.NewV7()}
}

func whenRevokeBuildSucceeds(t *testing.T) {
	rig := newRevokeRig()

	assert.Equal(t, rig.tokens, rig.useCase.Repository)
	assert.Equal(t, rig.guard, rig.useCase.Guard)
	assert.Equal(t, rig.tx, rig.useCase.Tx)
}

func whenRevokeSucceeds(t *testing.T) {
	rig := newRevokeRig()

	output := rig.useCase.Perform(context.Background(), revokeInput())

	assert.Nil(t, output.Error)
	assert.Len(t, rig.tokens.Deleted, 1)
	assert.Equal(t, []string{entities.AuditApiKeyRevoked}, rig.tx.Actions())
}

func whenRevokeIsRepeated(t *testing.T) {
	rig := newRevokeRig()
	rig.tokens.FindByIDFunc = func(_ context.Context, _, id uuid.UUID) (entities.ApiKey, error) {
		token := mocks.NewApiKey()
		now := time.Now()
		token.ID, token.MemberID, token.DeletedAt = id, rig.owner, &now
		return token, nil
	}

	output := rig.useCase.Perform(context.Background(), revokeInput())

	assert.Nil(t, output.Error, "revogar de novo é idempotente")
	assert.Empty(t, rig.tokens.Deleted)
	assert.Empty(t, rig.tx.Entries, "e não gera outra linha de auditoria")
}

func whenRevokeTokenBelongsToAnotherMember(t *testing.T) {
	rig := newRevokeRig()
	rig.owner = uuid.NewV7()
	rig.tokens.FindByIDFunc = func(_ context.Context, _, id uuid.UUID) (entities.ApiKey, error) {
		token := mocks.NewApiKey()
		token.ID, token.MemberID = id, uuid.NewV7()
		return token, nil
	}

	output := rig.useCase.Perform(context.Background(), revokeInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status, "o id de um token alheio não revela nada")
	}
	assert.Empty(t, rig.tokens.Deleted)
}

func whenRevokeTokenIsMissing(t *testing.T) {
	rig := newRevokeRig()
	rig.tokens.FindByIDFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.ApiKey, error) {
		return entities.ApiKey{}, sql.ErrNoRows
	}

	output := rig.useCase.Perform(context.Background(), revokeInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
}

func whenRevokeIsDenied(t *testing.T) {
	rig := newRevokeRig()
	rig.guard.Deny(entities.ForbiddenError())

	output := rig.useCase.Perform(context.Background(), revokeInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, rig.tokens.Deleted)
}

func whenRevokeFailsInDb(t *testing.T) {
	rig := newRevokeRig()
	rig.tokens.DeleteFunc = func(context.Context, uuid.UUID, uuid.UUID) error { return errors.New("segredo-interno") }

	output := rig.useCase.Perform(context.Background(), revokeInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}
