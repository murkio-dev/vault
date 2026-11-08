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
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestUpdateApplication(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenUpdateBuildSucceeds(t) })
	t.Run("Quando atualiza uma unica entidade.", func(t *testing.T) { whenUpdateOneEntity(t) })
	t.Run("Quando atualiza multiplas entidades.", func(t *testing.T) { whenUpdateMultipleEntities(t) })
	t.Run("Quando o ID é vazio.", func(t *testing.T) { whenUpdateEmptyID(t) })
	t.Run("Quando o nome é inválido.", func(t *testing.T) { whenUpdateInvalidName(t) })
	t.Run("Quando o ID está duplicado no request.", func(t *testing.T) { whenUpdateDuplicatedID(t) })
	t.Run("Quando a aplicação não existe.", func(t *testing.T) { whenUpdateNotFound(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenUpdateFailsInDb(t) })
	t.Run("Quando a lista de entrada é vazia.", func(t *testing.T) { whenUpdateEmptyInput(t) })
	t.Run("Quando recebe um lote misto com sucesso parcial.", func(t *testing.T) { whenUpdateMixedBatch(t) })
}

// Functions

func whenUpdateBuildSucceeds(t *testing.T) {
	repo := &mocks.ApplicationRepo{}

	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	assert.Equal(t, repo, useCase.Updater)
}

func whenUpdateOneEntity(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	app := mocks.NewApp()

	output := useCase.Perform(context.Background(), useCases.UpdateInput{
		Requester:    mocks.NewRequester(),
		Applications: []entities.Application{app},
	})

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, app.ID, output.Data[0].ID)
	}
	assert.Equal(t, 1, len(repo.Updated))
}

func whenUpdateMultipleEntities(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	apps := []entities.Application{mocks.NewApp(), mocks.NewApp(), mocks.NewApp()}

	output := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewRequester(), Applications: apps})

	assert.Empty(t, output.Errors)
	assert.Len(t, output.Data, 3)
	for i, app := range apps {
		assert.Equal(t, app.ID, output.Data[i].ID, "a ordem do lote deve ser preservada")
	}
	assert.Equal(t, 3, len(repo.Updated))
}

func whenUpdateEmptyID(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	app := mocks.NewApp()
	app.ID = uuid.UUID{}

	output := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewRequester(), Applications: []entities.Application{app}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
		assert.Equal(t, http.StatusBadRequest, output.Errors[0].Status)
	}
	assert.Zero(t, len(repo.Updated))
}

func whenUpdateInvalidName(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	app := mocks.NewApp()
	app.Name = "Nome Inválido!"

	output := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewRequester(), Applications: []entities.Application{app}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
	}
	assert.Zero(t, len(repo.Updated))
}

func whenUpdateDuplicatedID(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	dup := mocks.NewApp()
	other := mocks.NewApp()

	output := useCase.Perform(context.Background(), useCases.UpdateInput{
		Requester:     mocks.NewRequester(),
		Applications:  []entities.Application{dup, other, dup},
		DuplicatedIDs: map[uuid.UUID]bool{dup.ID: true},
	})

	assert.Len(t, output.Errors, 1, "o id duplicado é reportado uma única vez")
	assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
	if assert.Len(t, output.Data, 1, "o id não duplicado segue normalmente") {
		assert.Equal(t, other.ID, output.Data[0].ID)
	}
	assert.Equal(t, 1, len(repo.Updated), "nenhuma ocorrência do id duplicado é processada")
}

func whenUpdateNotFound(t *testing.T) {
	repo := &mocks.ApplicationRepo{UpdateFunc: func(ctx context.Context, input entities.Application) (entities.Application, error) {
		return entities.Application{}, sql.ErrNoRows
	}}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewRequester(), Applications: []entities.Application{mocks.NewApp()}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Errors[0].Type)
		assert.Equal(t, http.StatusNotFound, output.Errors[0].Status)
	}
}

func whenUpdateFailsInDb(t *testing.T) {
	repo := &mocks.ApplicationRepo{UpdateFunc: func(ctx context.Context, input entities.Application) (entities.Application, error) {
		return entities.Application{}, errors.New("db down")
	}}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewRequester(), Applications: []entities.Application{mocks.NewApp()}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type)
		assert.Equal(t, http.StatusInternalServerError, output.Errors[0].Status)
	}
}

func whenUpdateEmptyInput(t *testing.T) {
	useCase := useCases.NewUpdateApplicationUseCase(&mocks.ApplicationRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewRequester()})

	assert.Nil(t, output.Data)
	assert.Empty(t, output.Errors)
}

func whenUpdateMixedBatch(t *testing.T) {
	ok, missing, broken := mocks.NewApp(), mocks.NewApp(), mocks.NewApp()
	invalid := mocks.NewApp()
	invalid.Name = ""

	repo := &mocks.ApplicationRepo{UpdateFunc: func(ctx context.Context, input entities.Application) (entities.Application, error) {
		switch input.ID {
		case missing.ID:
			return entities.Application{}, sql.ErrNoRows
		case broken.ID:
			return entities.Application{}, errors.New("db error")
		}
		return input, nil
	}}
	useCase := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.UpdateInput{
		Requester:    mocks.NewRequester(),
		Applications: []entities.Application{ok, invalid, missing, broken},
	})

	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, ok.ID, output.Data[0].ID)
	}
	if assert.Len(t, output.Errors, 3) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
		assert.Equal(t, entities.ErrTypeNotFound, output.Errors[1].Type)
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[2].Type)
	}
}
