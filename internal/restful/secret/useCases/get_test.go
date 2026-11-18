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

func TestGetSecret(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenGetBuildSucceeds(t) })
	t.Run("Quando busca um segredo.", func(t *testing.T) { whenGetSucceeds(t) })
	t.Run("Quando o segredo não existe.", func(t *testing.T) { whenGetIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenGetIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenGetFailsInDb(t) })
}

// Functions

func whenGetBuildSucceeds(t *testing.T) {
	r := mocks.NewRig()

	assert.Equal(t, r.Secrets, getUseCase(r).Repository)
	assert.Equal(t, r.Guard, getUseCase(r).Guard)
	assert.NotPanics(t, func() { useCases.NewGetSecretUseCase(nil, nil) })
}

func whenGetSucceeds(t *testing.T) {
	r := mocks.NewRig()
	input := getInput(entities.SelfOwner)

	output := getUseCase(r).Perform(context.Background(), input)

	assert.Nil(t, output.Error)
	assert.Equal(t, input.Secret, output.Data.ID)
	assert.Equal(t, r.Owner, output.Data.MemberID)
	assert.Equal(t, input.Application, r.Secrets.Application(), "busca só na aplicação pedida")
	assert.Equal(t, []entities.Action{entities.ActionSecretRead}, r.Guard.Actions())
}

func whenGetIsMissing(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, sql.ErrNoRows
	}

	output := getUseCase(r).Perform(context.Background(), getInput(entities.SelfOwner))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Error.Type)
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
}

func whenGetIsDenied(t *testing.T) {
	r := mocks.NewRig()
	r.Guard.Deny(entities.ForbiddenError())
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		t.Error("negado, o repositório não é chamado")
		return entities.Secret{}, nil
	}

	output := getUseCase(r).Perform(context.Background(), getInput(uuid.NewV7()))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
}

func whenGetFailsInDb(t *testing.T) {
	r := mocks.NewRig()
	r.Secrets.FindByOwnerFunc = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (entities.Secret, error) {
		return entities.Secret{}, errors.New("segredo-interno")
	}

	output := getUseCase(r).Perform(context.Background(), getInput(entities.SelfOwner))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}

// Helpers

func getUseCase(r mocks.Rig) useCases.GetSecretUseCase {
	return useCases.NewGetSecretUseCase(r.Secrets, r.Guard)
}

func getInput(member uuid.UUID) useCases.GetInput {
	return useCases.GetInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: member, Secret: uuid.NewV7()}
}
