package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/mocks"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestListApiKeys(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenListBuildSucceeds(t) })
	t.Run("Quando lista os tokens do membro.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando não há tokens.", func(t *testing.T) { whenListIsEmpty(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenListIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenListFailsInDb(t) })
}

// Functions

func listInput() useCases.ListInput {
	return useCases.ListInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Member: entities.SelfOwner}
}

func whenListBuildSucceeds(t *testing.T) {
	repo, guard := &mocks.ApiKeyRepo{}, &member_mocks.Guard{}

	useCase := useCases.NewListApiKeysUseCase(repo, guard)

	assert.Equal(t, repo, useCase.Repository)
	assert.Equal(t, guard, useCase.Guard)
}

func whenListSucceeds(t *testing.T) {
	tokens := []entities.ApiKey{mocks.NewApiKey(), mocks.NewApiKey()}
	repo := &mocks.ApiKeyRepo{ListFunc: func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.ApiKey, int, error) {
		return tokens, 2, nil
	}}
	guard := &member_mocks.Guard{}

	output := useCases.NewListApiKeysUseCase(repo, guard).Perform(context.Background(), listInput())

	assert.Nil(t, output.Error)
	assert.Equal(t, tokens, output.Data)
	assert.Equal(t, []entities.Action{entities.ActionApiKeyRead}, guard.Actions())
}

func whenListIsEmpty(t *testing.T) {
	output := useCases.NewListApiKeysUseCase(&mocks.ApiKeyRepo{}, &member_mocks.Guard{}).Perform(context.Background(), listInput())

	assert.Nil(t, output.Error)
	assert.NotNil(t, output.Data)
	assert.Empty(t, output.Data)
}

func whenListIsDenied(t *testing.T) {
	repo := &mocks.ApiKeyRepo{ListFunc: func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.ApiKey, int, error) {
		t.Error("negado, o repositório não é chamado")
		return nil, 0, nil
	}}

	output := useCases.NewListApiKeysUseCase(repo, (&member_mocks.Guard{}).Deny(entities.ForbiddenError())).Perform(context.Background(), listInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
}

func whenListFailsInDb(t *testing.T) {
	repo := &mocks.ApiKeyRepo{ListFunc: func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.ApiKey, int, error) {
		return nil, 0, errors.New("segredo-interno")
	}}

	output := useCases.NewListApiKeysUseCase(repo, &member_mocks.Guard{}).Perform(context.Background(), listInput())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}
