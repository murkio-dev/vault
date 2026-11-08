package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestRevokeInvitation(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenRevokeBuildSucceeds(t) })
	t.Run("Quando revoga um convite pendente.", func(t *testing.T) { whenRevokeSucceeds(t) })
	t.Run("Quando o convite não existe ou já se encerrou.", func(t *testing.T) { whenRevokeFindsNothing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenRevokeIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenRevokeFailsInDb(t) })
}

// Functions

func revokeInput() useCases.RevokeInput {
	return useCases.RevokeInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Invitation: uuid.NewV7()}
}

func whenRevokeBuildSucceeds(t *testing.T) {
	repo, guard, tx := &mocks.InvitationRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{}

	useCase := useCases.NewRevokeInvitationUseCase(repo, guard, tx)

	assert.Equal(t, repo, useCase.Revoker)
	assert.Equal(t, guard, useCase.Guard)
	assert.Equal(t, tx, useCase.Tx)
}

func whenRevokeSucceeds(t *testing.T) {
	repo, tx := &mocks.InvitationRepo{}, &audit_mocks.Tx{}

	output := useCases.NewRevokeInvitationUseCase(repo, &member_mocks.Guard{}, tx).Perform(context.Background(), revokeInput())

	assert.Nil(t, output.Error)
	assert.Len(t, repo.Revoked, 1)
	assert.Equal(t, []string{entities.AuditInvitationRevoked}, tx.Actions())
}

func whenRevokeFindsNothing(t *testing.T) {
	repo := &mocks.InvitationRepo{RevokeFunc: func(context.Context, uuid.UUID, uuid.UUID) error { return sql.ErrNoRows }}
	tx := &audit_mocks.Tx{}

	output := useCases.NewRevokeInvitationUseCase(repo, &member_mocks.Guard{}, tx).Perform(context.Background(), revokeInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Error.Type)
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Empty(t, tx.Entries, "sem revogar nada, nada é auditado")
}

func whenRevokeIsDenied(t *testing.T) {
	repo := &mocks.InvitationRepo{}
	guard := (&member_mocks.Guard{}).Deny(entities.ForbiddenError())

	output := useCases.NewRevokeInvitationUseCase(repo, guard, &audit_mocks.Tx{}).Perform(context.Background(), revokeInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, repo.Revoked)
}

func whenRevokeFailsInDb(t *testing.T) {
	repo := &mocks.InvitationRepo{RevokeFunc: func(context.Context, uuid.UUID, uuid.UUID) error { return errors.New("segredo-interno") }}

	output := useCases.NewRevokeInvitationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{}).Perform(context.Background(), revokeInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}
