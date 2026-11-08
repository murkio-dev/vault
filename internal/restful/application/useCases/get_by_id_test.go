package useCases_test

import (
	"context"
	"database/sql"
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

func TestGetApplicationByID(t *testing.T) {
	t.Run("Quando a aplicação existe.", func(t *testing.T) { whenGetByIDFindsApplication(t) })
	t.Run("Quando a aplicação não existe.", func(t *testing.T) { whenGetByIDNotFound(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenGetByIDFailsInDb(t) })
}

// Functions

func whenGetByIDFindsApplication(t *testing.T) {
	app := mocks.NewApp()
	repo := &mocks.ApplicationRepo{
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (entities.Application, error) {
			assert.Equal(t, app.ID, id)
			return app, nil
		},
	}
	useCase := useCases.NewGetApplicationByIDUseCase(repo, &member_mocks.Guard{})

	output := useCase.Perform(context.Background(), useCases.GetByIDInput{Requester: mocks.NewRequester(), Application: app.ID})

	assert.Nil(t, output.Error)
	assert.Equal(t, app, output.Data)
}

func whenGetByIDNotFound(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (entities.Application, error) {
			return entities.Application{}, sql.ErrNoRows
		},
	}
	useCase := useCases.NewGetApplicationByIDUseCase(repo, &member_mocks.Guard{})

	output := useCase.Perform(context.Background(), useCases.GetByIDInput{Requester: mocks.NewRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Error.Type)
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Equal(t, entities.Application{}, output.Data)
}

func whenGetByIDFailsInDb(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (entities.Application, error) {
			return entities.Application{}, errors.New("connection lost")
		},
	}
	useCase := useCases.NewGetApplicationByIDUseCase(repo, &member_mocks.Guard{})

	output := useCase.Perform(context.Background(), useCases.GetByIDInput{Requester: mocks.NewRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.Equal(t, http.StatusInternalServerError, output.Error.Status)
	}
}
