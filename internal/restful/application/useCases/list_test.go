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
}

// Functions

func whenListSucceeds(t *testing.T) {
	apps := []entities.Application{mocks.NewApp(), mocks.NewApp()}
	repo := &mocks.ApplicationRepo{
		ListByUserFunc: func(ctx context.Context, _ uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
			return apps, len(apps), nil
		},
	}
	useCase := useCases.NewListApplicationsUseCase(repo, repo)

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
	useCase := useCases.NewListApplicationsUseCase(repo, repo)

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
	useCase := useCases.NewListApplicationsUseCase(repo, repo)

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
	useCase := useCases.NewListApplicationsUseCase(repo, repo)

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
	useCase := useCases.NewListApplicationsUseCase(repo, repo)

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 2, Limit: 10})

	assert.True(t, output.Page.HasNext)
	assert.True(t, output.Page.HasPrev)
	assert.Equal(t, 25, output.Page.Total)
}

func whenListIsEmpty(t *testing.T) {
	useCase := useCases.NewListApplicationsUseCase(&mocks.ApplicationRepo{}, &mocks.ApplicationRepo{})

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
	useCase := useCases.NewListApplicationsUseCase(repo, repo)

	output := useCase.Perform(context.Background(), useCases.ListInput{Requester: mocks.NewRequester(), Page: 1, Limit: 10})

	assert.Nil(t, output.Data)
	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.Equal(t, http.StatusInternalServerError, output.Error.Status)
	}
}
