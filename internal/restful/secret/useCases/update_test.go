package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestUpdateSecret(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenUpdateBuildSucceeds(t) })
	t.Run("Quando troca o valor.", func(t *testing.T) { whenUpdateSucceeds(t) })
	t.Run("Quando o valor novo é inválido.", func(t *testing.T) { whenUpdateValueIsInvalid(t) })
	t.Run("Quando atualiza uma referência.", func(t *testing.T) { whenUpdateTargetsAReference(t) })
	t.Run("Quando a versão corre com outra gravação.", func(t *testing.T) { whenUpdateLosesTheRace(t) })
	t.Run("Quando o segredo não existe.", func(t *testing.T) { whenUpdateSecretIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenUpdateIsDenied(t) })
	t.Run("Quando a cifragem, o banco ou a auditoria falham.", func(t *testing.T) { whenUpdateInfrastructureFails(t) })
}

// Functions

func whenUpdateBuildSucceeds(t *testing.T) {
	r := mocks.NewRig()

	assert.Equal(t, r.Secrets, updateUseCase(r).Repository)
	assert.Equal(t, r.Values, updateUseCase(r).Values)
	assert.Equal(t, r.Guard, updateUseCase(r).Guard)
	assert.Equal(t, r.Tx, updateUseCase(r).Tx)
	assert.NotPanics(t, func() { useCases.NewUpdateSecretUseCase(nil, nil, nil, nil) })
}

func whenUpdateSucceeds(t *testing.T) {
	r := mocks.NewRig()
	value := "novo-valor-nao-vazar"

	output := updateUseCase(r).Perform(context.Background(), updateInput(value))

	assert.Nil(t, output.Error)
	assert.Len(t, r.Secrets.Added, 1)
	assert.Equal(t, []string{value}, r.Values.Sealed)
	assert.Equal(t, []entities.Action{entities.ActionSecretWrite}, r.Guard.Actions())
	if assert.Len(t, r.Tx.Entries, 1) {
		assert.Equal(t, entities.AuditSecretUpdated, r.Tx.Entries[0].Action)
		assert.Contains(t, string(r.Tx.Entries[0].Metadata), `"version":2`, "a trilha leva a versão nova")
		assert.NotContains(t, string(r.Tx.Entries[0].Metadata), value)
	}
}

func whenUpdateValueIsInvalid(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		secret := mocks.NewSecret()
		secret.Type = entities.SecretURL
		return secret, nil
	}

	for name, value := range map[string]string{"vazio": "", "gigante": strings.Repeat("a", entities.MaxSecretValueBytes+1), "url sem host": "segredo-que-nao-e-url"} {
		output := updateUseCase(r).Perform(context.Background(), updateInput(value))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusBadRequest, output.Error.Status, name)
			assert.NotContains(t, output.Error.Detail, "segredo-que-nao-e-url", name)
			assert.NotContains(t, fmt.Sprint(output.Error.InvalidParams), "segredo-que-nao-e-url", name)
		}
	}
	assert.Empty(t, r.Secrets.Added)
	assert.Empty(t, r.Values.Sealed)
	assert.Zero(t, r.Tx.Runs)
}

func whenUpdateTargetsAReference(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		secret := mocks.NewSecret()
		secret.Source = entities.SourceReference
		return secret, nil
	}

	output := updateUseCase(r).Perform(context.Background(), updateInput("valor"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusBadRequest, output.Error.Status, "uma referência não tem valor próprio")
	}
	assert.Empty(t, r.Secrets.Added)
}

func whenUpdateLosesTheRace(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.AddVersionFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	output := updateUseCase(r).Perform(context.Background(), updateInput("valor"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status, "o UPDATE condicional impede duas versões com o mesmo número")
	}
}

func whenUpdateSecretIsMissing(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	output := updateUseCase(r).Perform(context.Background(), updateInput("valor"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Zero(t, r.Tx.Runs)
	assert.Empty(t, r.Values.Sealed, "sem o segredo, nem cifra")
}

func whenUpdateIsDenied(t *testing.T) {
	r := mocks.NewRig()
	r.Guard.Deny(entities.ForbiddenError())

	output := updateUseCase(r).Perform(context.Background(), updateInput("valor"))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Zero(t, r.Tx.Runs)
	assert.Empty(t, r.Values.Sealed, "negado, nem cifra")
	assert.Empty(t, r.Secrets.Added)
}

func whenUpdateInfrastructureFails(t *testing.T) {
	boom := errors.New("segredo-interno")
	for name, setup := range map[string]func(mocks.Rig){
		"cifragem": func(r mocks.Rig) { r.Values.SealErr = boom },
		"banco": func(r mocks.Rig) {
			r.Secrets.AddVersionFunc = func(context.Context, entities.SecretWrite) (entities.Secret, error) { return entities.Secret{}, boom }
		},
		"auditoria": func(r mocks.Rig) { r.Tx.AuditErr = boom },
	} {
		r := mocks.NewRig()
		setup(r)

		output := updateUseCase(r).Perform(context.Background(), updateInput("valor"))

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Error.Type, name)
			assert.NotContains(t, output.Error.Detail, "segredo-interno", name)
		}
	}
}

// Helpers

func updateUseCase(r mocks.Rig) useCases.UpdateSecretUseCase {
	return useCases.NewUpdateSecretUseCase(r.Secrets, r.Values, r.Guard, r.Tx)
}

func updateInput(value string) useCases.UpdateInput {
	return useCases.UpdateInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: entities.SelfOwner, Secret: uuid.NewV7(), Value: value}
}
