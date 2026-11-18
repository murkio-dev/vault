package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestDeleteSecret(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenDeleteBuildSucceeds(t) })
	t.Run("Quando apaga.", func(t *testing.T) { whenDeleteSucceeds(t) })
	t.Run("Quando outros segredos apontam para o apagado.", func(t *testing.T) { whenDeleteIsReferenced(t) })
	t.Run("Quando o segredo não existe.", func(t *testing.T) { whenDeleteSecretIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenDeleteIsDenied(t) })
	t.Run("Quando o banco ou a auditoria falham.", func(t *testing.T) { whenDeleteInfrastructureFails(t) })
}

// Functions

func whenDeleteBuildSucceeds(t *testing.T) {
	r := mocks.NewRig()

	assert.Equal(t, r.Secrets, deleteUseCase(r).Repository)
	assert.Equal(t, r.Guard, deleteUseCase(r).Guard)
	assert.Equal(t, r.Tx, deleteUseCase(r).Tx)
	assert.NotPanics(t, func() { useCases.NewDeleteSecretUseCase(nil, nil, nil) })
}

func whenDeleteSucceeds(t *testing.T) {
	r := mocks.NewRig()

	output := deleteUseCase(r).Perform(context.Background(), deleteInput())

	assert.Nil(t, output.Error)
	assert.Len(t, r.Secrets.Deleted, 1)
	assert.Equal(t, []entities.Action{entities.ActionSecretWrite}, r.Guard.Actions())
	assert.Equal(t, []string{entities.AuditSecretDeleted}, r.Tx.Actions())
}

func whenDeleteIsReferenced(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.CountReferencesFunc = func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 2, nil }

	output := deleteUseCase(r).Perform(context.Background(), deleteInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusConflict, output.Error.Status)
	}
	assert.Empty(t, r.Secrets.Deleted, "apagar quebraria o runtime de quem aponta")
}

func whenDeleteSecretIsMissing(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	output := deleteUseCase(r).Perform(context.Background(), deleteInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Zero(t, r.Tx.Runs)
	assert.Empty(t, r.Secrets.Deleted)
}

func whenDeleteIsDenied(t *testing.T) {
	r := mocks.NewRig()
	r.Guard.Deny(entities.ForbiddenError())

	output := deleteUseCase(r).Perform(context.Background(), deleteInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Zero(t, r.Tx.Runs)
	assert.Empty(t, r.Secrets.Deleted)
}

func whenDeleteInfrastructureFails(t *testing.T) {
	boom := errors.New("segredo-interno")
	for name, setup := range map[string]func(mocks.Rig){
		"contagem de referências": func(r mocks.Rig) {
			r.Secrets.CountReferencesFunc = func(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 0, boom }
		},
		"banco": func(r mocks.Rig) {
			r.Secrets.DeleteFunc = func(context.Context, uuid.UUID, uuid.UUID) error { return boom }
		},
		"auditoria": func(r mocks.Rig) { r.Tx.AuditErr = boom },
	} {
		r := mocks.NewRig()
		setup(r)

		output := deleteUseCase(r).Perform(context.Background(), deleteInput())

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Error.Type, name)
			assert.NotContains(t, output.Error.Detail, "segredo-interno", name)
		}
	}
}

// Helpers

func deleteUseCase(r mocks.Rig) useCases.DeleteSecretUseCase {
	return useCases.NewDeleteSecretUseCase(r.Secrets, r.Guard, r.Tx)
}

func deleteInput() useCases.DeleteInput {
	return useCases.DeleteInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7()}
}
