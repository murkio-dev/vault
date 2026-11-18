package useCases_test

import (
	"context"
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

func TestListSecrets(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenListBuildSucceeds(t) })
	t.Run("Quando lista os metadados.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando a lista está vazia.", func(t *testing.T) { whenListIsEmpty(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenListIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenListFailsInDb(t) })
}

// Functions

func whenListBuildSucceeds(t *testing.T) {
	r := mocks.NewRig()

	assert.Equal(t, r.Secrets, listUseCase(r).Repository)
	assert.Equal(t, r.Guard, listUseCase(r).Guard)
	assert.NotPanics(t, func() { useCases.NewListSecretsUseCase(nil, nil) })
}

func whenListSucceeds(t *testing.T) {
	r := mocks.NewRig()
	secrets := []entities.Secret{mocks.NewSecret(), mocks.NewSecret()}
	r.Secrets.ListFunc = func(_ context.Context, _, owner uuid.UUID, _, _ int) ([]entities.Secret, int, error) {
		assert.Equal(t, r.Owner, owner, "lista a tabela do dono resolvido")
		return secrets, len(secrets), nil
	}

	output := listUseCase(r).Perform(context.Background(), listInput(entities.SelfOwner))

	assert.Nil(t, output.Error)
	assert.Equal(t, secrets, output.Data)
	assert.Equal(t, []entities.Action{entities.ActionSecretRead}, r.Guard.Actions())
	assert.True(t, r.Guard.Inputs[0].OwnerMustExist)
}

func whenListIsEmpty(t *testing.T) {
	r := mocks.NewRig()

	output := listUseCase(r).Perform(context.Background(), listInput(entities.SelfOwner))

	assert.Nil(t, output.Error)
	assert.NotNil(t, output.Data, "lista vazia é um array")
	assert.Empty(t, output.Data)
}

func whenListIsDenied(t *testing.T) {
	r := mocks.NewRig()
	r.Guard.Deny(entities.ForbiddenError())
	r.Secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		t.Error("negado, o repositório não é chamado")
		return nil, 0, nil
	}

	output := listUseCase(r).Perform(context.Background(), listInput(uuid.NewV7()))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Nil(t, output.Data)
}

func whenListFailsInDb(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		return nil, 0, errors.New("segredo-interno")
	}

	output := listUseCase(r).Perform(context.Background(), listInput(entities.SelfOwner))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}

// Helpers

func listUseCase(r mocks.Rig) useCases.ListSecretsUseCase {
	return useCases.NewListSecretsUseCase(r.Secrets, r.Guard)
}

func listInput(member uuid.UUID) useCases.ListInput {
	return useCases.ListInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: member}
}
