package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/mocks"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestListApplications(t *testing.T) {
	t.Run("Quando lista com sucesso.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando page e limit são inválidos.", func(t *testing.T) { whenListUsesDefaults(t) })
	t.Run("Quando o limit passa do máximo.", func(t *testing.T) { whenListCapsLimit(t) })
	t.Run("Quando calcula o offset da página.", func(t *testing.T) { whenListComputesOffset(t) })
	t.Run("Quando há próxima e página anterior.", func(t *testing.T) { whenListPaginatorFlags(t) })
	t.Run("Quando não há registros.", func(t *testing.T) { whenListIsEmpty(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenListFailsInDb(t) })
	t.Run("Quando a pessoa lista as próprias aplicações.", func(t *testing.T) { whenListPersonSeesOwnApplications(t) })
	t.Run("Quando o platform admin lista.", func(t *testing.T) { whenListPlatformAdminSeesAll(t) })
	t.Run("Quando quem lista não é uma pessoa autenticada.", func(t *testing.T) { whenListIsNotAuthenticated(t) })
}

// Functions

func whenListSucceeds(t *testing.T) {
	apps := []entities.Application{mocks.NewApp(), mocks.NewApp()}
	repo := &mocks.ApplicationRepo{
		ListByUserFunc: func(ctx context.Context, _ uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
			return apps, len(apps), nil
		},
	}
	useCase := useCases.NewListApplicationsUseCase(repo)

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 1, Limit: 10})

	assert.Nil(t, output.Error)
	assert.Equal(t, apps, output.Data)
	assert.Equal(t, 1, output.Page.Page)
	assert.Equal(t, 10, output.Page.Limit)
	assert.Equal(t, 2, output.Page.Total)
	assert.False(t, output.Page.HasNext)
	assert.False(t, output.Page.HasPrev)
}

func whenListUsesDefaults(t *testing.T) {
	var gotLimit, gotOffset int
	repo := &mocks.ApplicationRepo{
		ListByUserFunc: func(ctx context.Context, _ uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
			gotLimit, gotOffset = limit, offset
			return []entities.Application{}, 0, nil
		},
	}
	useCase := useCases.NewListApplicationsUseCase(repo)

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 0, Limit: -5})

	assert.Equal(t, entities.MaxPageSize, gotLimit)
	assert.Equal(t, 0, gotOffset)
	assert.Equal(t, 1, output.Page.Page)
}

func whenListCapsLimit(t *testing.T) {
	var gotLimit int
	repo := &mocks.ApplicationRepo{
		ListByUserFunc: func(ctx context.Context, _ uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
			gotLimit = limit
			return []entities.Application{}, 0, nil
		},
	}
	useCase := useCases.NewListApplicationsUseCase(repo)

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 1, Limit: entities.MaxPageSize + 1})

	assert.Equal(t, entities.MaxPageSize, gotLimit)
	assert.Equal(t, entities.MaxPageSize, output.Page.Limit)
}

func whenListComputesOffset(t *testing.T) {
	var gotLimit, gotOffset int
	repo := &mocks.ApplicationRepo{
		ListByUserFunc: func(ctx context.Context, _ uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
			gotLimit, gotOffset = limit, offset
			return []entities.Application{}, 0, nil
		},
	}
	useCase := useCases.NewListApplicationsUseCase(repo)

	useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 3, Limit: 20})

	assert.Equal(t, 20, gotLimit)
	assert.Equal(t, 40, gotOffset, "offset = (page-1)*limit")
}

func whenListPaginatorFlags(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		ListByUserFunc: func(ctx context.Context, _ uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
			return []entities.Application{mocks.NewApp()}, 25, nil
		},
	}
	useCase := useCases.NewListApplicationsUseCase(repo)

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 2, Limit: 10})

	assert.True(t, output.Page.HasNext)
	assert.True(t, output.Page.HasPrev)
	assert.Equal(t, 25, output.Page.Total)
}

func whenListIsEmpty(t *testing.T) {
	useCase := useCases.NewListApplicationsUseCase(&mocks.ApplicationRepo{})

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 1, Limit: 10})

	assert.Nil(t, output.Error)
	assert.Empty(t, output.Data)
	assert.Equal(t, 0, output.Page.Total)
}

func whenListFailsInDb(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		ListByUserFunc: func(ctx context.Context, _ uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
			return nil, 0, errors.New("db down")
		},
	}
	useCase := useCases.NewListApplicationsUseCase(repo)

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 1, Limit: 10})

	assert.Nil(t, output.Data)
	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.Equal(t, http.StatusInternalServerError, output.Error.Status)
	}
}

func whenListPersonSeesOwnApplications(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	requester := mocks.NewRequester()

	output := useCases.NewListApplicationsUseCase(repo).Perform(context.Background(), useCases.ListInput{Requester: requester})

	assert.Nil(t, output.Error)
	assert.Equal(t, []uuid.UUID{requester.Actor.UserID}, repo.Users, "só as aplicações de que a pessoa é membro")
}

func whenListPlatformAdminSeesAll(t *testing.T) {
	called := false
	repo := &mocks.ApplicationRepo{ListFunc: func(context.Context, int, int) ([]entities.Application, int, error) {
		called = true
		return []entities.Application{}, 0, nil
	}}
	requester := mocks.NewRequester()
	requester.Actor.PlatformAdmin = true

	output := useCases.NewListApplicationsUseCase(repo).Perform(context.Background(), useCases.ListInput{Requester: requester})

	assert.Nil(t, output.Error)
	assert.True(t, called, "o platform admin vê todas")
	assert.Empty(t, repo.Users, "o platform admin lista sem filtro de conta")
}

func whenListIsNotAuthenticated(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	token := member_mocks.NewTokenRequester(member_mocks.NewMember(entities.RoleDeveloper))

	for name, requester := range map[string]entities.ApiHeader{"anônimo": {}, "token do SDK": token} {
		output := useCases.NewListApplicationsUseCase(repo).Perform(context.Background(), useCases.ListInput{Requester: requester})

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusUnauthorized, output.Error.Status, name)
		}
	}
	assert.Empty(t, repo.Users, "quem não está autenticado nem chega ao repositório")
	assert.Empty(t, repo.Windows)
}
