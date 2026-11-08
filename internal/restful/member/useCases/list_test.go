package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestListMembers(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenListBuildSucceeds(t) })
	t.Run("Quando lista com sucesso.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando page e limit são inválidos ou passam do máximo.", func(t *testing.T) { whenListWindowIsClamped(t) })
	t.Run("Quando calcula o offset da página.", func(t *testing.T) { whenListComputesOffset(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenListIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenListFailsInDb(t) })
}

// Functions

func whenListBuildSucceeds(t *testing.T) {
	repo, guard := &mocks.MemberRepo{}, &mocks.Guard{}

	useCase := useCases.NewListMembersUseCase(repo, guard)

	assert.Equal(t, repo, useCase.Lister)
	assert.Equal(t, guard, useCase.Guard)
}

func whenListSucceeds(t *testing.T) {
	members := []entities.Member{mocks.NewMember(entities.RoleAdmin), mocks.NewMember(entities.RoleDeveloper)}
	repo := &mocks.MemberRepo{ListFunc: func(context.Context, uuid.UUID, int, int) ([]entities.Member, int, error) { return members, 2, nil }}
	guard := &mocks.Guard{}

	app := uuid.NewV7()

	output := useCases.NewListMembersUseCase(repo, guard).Perform(context.Background(), useCases.ListInput{Requester: mocks.NewUserRequester(), Application: app})

	assert.Nil(t, output.Error)
	assert.Equal(t, members, output.Data)
	assert.Equal(t, []entities.Action{entities.ActionMemberList}, guard.Actions())
	assert.Equal(t, app, repo.Application(), "a listagem é da aplicação pedida")
	if assert.NotNil(t, output.Page) {
		assert.Equal(t, 2, output.Page.Total)
		assert.False(t, output.Page.HasNext)
	}
}

func whenListWindowIsClamped(t *testing.T) {
	repo := &mocks.MemberRepo{}
	useCase := useCases.NewListMembersUseCase(repo, &mocks.Guard{})

	useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Page: -3, Limit: 0})
	useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Page: 1, Limit: entities.MaxPageSize + 1})

	assert.Equal(t, [][2]int{{entities.MaxPageSize, 0}, {entities.MaxPageSize, 0}}, repo.Windows)
}

func whenListComputesOffset(t *testing.T) {
	repo := &mocks.MemberRepo{}

	useCases.NewListMembersUseCase(repo, &mocks.Guard{}).Perform(context.Background(), useCases.ListInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Page: 3, Limit: 20})

	assert.Equal(t, [][2]int{{20, 40}}, repo.Windows, "offset = (page-1)*limit")
}

func whenListIsDenied(t *testing.T) {
	repo := &mocks.MemberRepo{}
	guard := (&mocks.Guard{}).Deny(entities.ForbiddenError())

	output := useCases.NewListMembersUseCase(repo, guard).Perform(context.Background(), useCases.ListInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, repo.Windows, "negado, o repositório não é chamado")
}

func whenListFailsInDb(t *testing.T) {
	repo := &mocks.MemberRepo{ListFunc: func(context.Context, uuid.UUID, int, int) ([]entities.Member, int, error) {
		return nil, 0, errors.New("segredo-interno")
	}}

	output := useCases.NewListMembersUseCase(repo, &mocks.Guard{}).Perform(context.Background(), useCases.ListInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
	assert.Nil(t, output.Data)
}
