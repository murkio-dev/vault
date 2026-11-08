package useCases_test

import (
	"context"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestGetMe(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenGetMeBuildSucceeds(t) })
	t.Run("Quando o ator é membro.", func(t *testing.T) { whenGetMeSucceeds(t) })
	t.Run("Quando o ator não tem membro na aplicação.", func(t *testing.T) { whenGetMeHasNoMember(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenGetMeIsDenied(t) })
}

// Functions

func whenGetMeBuildSucceeds(t *testing.T) {
	guard := &mocks.Guard{}

	assert.Equal(t, guard, useCases.NewGetMeUseCase(guard).Guard)
}

func whenGetMeSucceeds(t *testing.T) {
	member := mocks.NewMember(entities.RoleDeveloper)
	guard := &mocks.Guard{Member: member}

	output := useCases.NewGetMeUseCase(guard).Perform(context.Background(), useCases.GetMeInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7()})

	assert.Nil(t, output.Error)
	assert.Equal(t, member, output.Data)
	assert.Equal(t, []entities.Action{entities.ActionApplicationRead}, guard.Actions())
}

func whenGetMeHasNoMember(t *testing.T) {
	guard := &mocks.Guard{PerformFunc: func(context.Context, entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{}
	}}

	output := useCases.NewGetMeUseCase(guard).Perform(context.Background(), useCases.GetMeInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status, "o platform admin sem vínculo não tem membro")
	}
}

func whenGetMeIsDenied(t *testing.T) {
	guard := (&mocks.Guard{}).Deny(entities.NotFoundError("Resource"))

	output := useCases.NewGetMeUseCase(guard).Perform(context.Background(), useCases.GetMeInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
}
