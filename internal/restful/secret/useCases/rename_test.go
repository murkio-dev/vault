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

func TestRenameSecret(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenRenameBuildSucceeds(t) })
	t.Run("Quando renomeia.", func(t *testing.T) { whenRenameSucceeds(t) })
	t.Run("Quando o nome novo é inválido.", func(t *testing.T) { whenRenameNameIsInvalid(t) })
	t.Run("Quando o nome novo já existe.", func(t *testing.T) { whenRenameNameExists(t) })
	t.Run("Quando o segredo não existe.", func(t *testing.T) { whenRenameSecretIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenRenameIsDenied(t) })
	t.Run("Quando o banco ou a auditoria falham.", func(t *testing.T) { whenRenameInfrastructureFails(t) })
}

// Functions

func whenRenameBuildSucceeds(t *testing.T) {
	r := mocks.NewRig()

	assert.Equal(t, r.Secrets, renameUseCase(r).Repository)
	assert.Equal(t, r.Guard, renameUseCase(r).Guard)
	assert.Equal(t, r.Tx, renameUseCase(r).Tx)
	assert.NotPanics(t, func() { useCases.NewRenameSecretUseCase(nil, nil, nil) })
}

func whenRenameSucceeds(t *testing.T) {
	r := mocks.NewRig()
	name := "NOVO_NOME"

	output := renameUseCase(r).Perform(context.Background(), renameInput(name))

	assert.Nil(t, output.Error)
	assert.Equal(t, name, output.Data.Name)
	if assert.Len(t, r.Secrets.Renamed, 1) {
		assert.Equal(t, name, r.Secrets.Renamed[0].Name)
	}
	assert.Equal(t, []entities.Action{entities.ActionSecretWrite}, r.Guard.Actions())
	assert.Equal(t, []string{entities.AuditSecretRenamed}, r.Tx.Actions())
}

func whenRenameNameIsInvalid(t *testing.T) {
	for _, name := range []string{"", "minúsculo", "COM ESPAÇO", "1COMECA_COM_NUMERO"} {
		r := mocks.NewRig()

		output := renameUseCase(r).Perform(context.Background(), renameInput(name))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusBadRequest, output.Error.Status, name)
		}
		assert.Empty(t, r.Secrets.Renamed, name)
		assert.Zero(t, r.Tx.Runs, name)
	}
}

func whenRenameNameExists(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.RenameFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
		return entities.Secret{}, mocks.NewUniqueViolation()
	}

	output := renameUseCase(r).Perform(context.Background(), renameInput("JA_EXISTE"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusConflict, output.Error.Status)
	}
}

func whenRenameSecretIsMissing(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	output := renameUseCase(r).Perform(context.Background(), renameInput("OUTRO"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Zero(t, r.Tx.Runs)
}

func whenRenameIsDenied(t *testing.T) {
	r := mocks.NewRig()
	r.Guard.Deny(entities.ForbiddenError())

	output := renameUseCase(r).Perform(context.Background(), renameInput("OUTRO"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Zero(t, r.Tx.Runs)
	assert.Empty(t, r.Secrets.Renamed)
}

func whenRenameInfrastructureFails(t *testing.T) {
	boom := errors.New("segredo-interno")
	for name, setup := range map[string]func(mocks.Rig){
		"banco": func(r mocks.Rig) {
			r.Secrets.RenameFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) { return entities.Secret{}, boom }
		},
		"auditoria": func(r mocks.Rig) { r.Tx.AuditErr = boom },
	} {
		r := mocks.NewRig()
		setup(r)

		output := renameUseCase(r).Perform(context.Background(), renameInput("OUTRO"))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Error.Type, name)
			assert.NotContains(t, output.Error.Detail, "segredo-interno", name)
		}
	}
}

// Helpers

func renameUseCase(r mocks.Rig) useCases.RenameSecretUseCase {
	return useCases.NewRenameSecretUseCase(r.Secrets, r.Guard, r.Tx)
}

func renameInput(name string) useCases.RenameInput {
	return useCases.RenameInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7(), Name: name}
}
