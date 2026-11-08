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

func TestDeleteApplication(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenDeleteBuildSucceeds(t) })
	t.Run("Quando deleta um unico id.", func(t *testing.T) { whenDeleteOneID(t) })
	t.Run("Quando deleta multiplos ids.", func(t *testing.T) { whenDeleteMultipleIDs(t) })
	t.Run("Quando o ID é vazio.", func(t *testing.T) { whenDeleteEmptyID(t) })
	t.Run("Quando o ID está duplicado no request.", func(t *testing.T) { whenDeleteDuplicatedID(t) })
	t.Run("Quando a aplicação não existe.", func(t *testing.T) { whenDeleteNotFound(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenDeleteFailsInDb(t) })
	t.Run("Quando a lista de entrada é vazia.", func(t *testing.T) { whenDeleteEmptyInput(t) })
	t.Run("Quando recebe um lote misto com sucesso parcial.", func(t *testing.T) { whenDeleteMixedBatch(t) })
}

// Functions

func whenDeleteBuildSucceeds(t *testing.T) {
	repo := &mocks.ApplicationRepo{}

	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	assert.Equal(t, repo, useCase.Deleter)
}

func whenDeleteOneID(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	id := uuid.NewV7()

	output := useCase.Perform(context.Background(), useCases.DeleteInput{Requester: mocks.NewRequester(), IDs: []uuid.UUID{id}})

	assert.Empty(t, output.Errors)
	assert.Equal(t, []uuid.UUID{id}, output.Data)
	assert.Equal(t, 1, len(repo.Deleted))
}

func whenDeleteMultipleIDs(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}

	output := useCase.Perform(context.Background(), useCases.DeleteInput{Requester: mocks.NewRequester(), IDs: ids})

	assert.Empty(t, output.Errors)
	assert.Equal(t, ids, output.Data, "a ordem do lote deve ser preservada")
	assert.Equal(t, 3, len(repo.Deleted))
}

func whenDeleteEmptyID(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.DeleteInput{Requester: mocks.NewRequester(), IDs: []uuid.UUID{{}}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
		assert.Equal(t, http.StatusBadRequest, output.Errors[0].Status)
	}
	assert.Zero(t, len(repo.Deleted))
}

func whenDeleteDuplicatedID(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})
	dup, other := uuid.NewV7(), uuid.NewV7()

	output := useCase.Perform(context.Background(), useCases.DeleteInput{
		Requester:     mocks.NewRequester(),
		IDs:           []uuid.UUID{dup, other, dup},
		DuplicatedIDs: map[uuid.UUID]bool{dup: true},
	})

	assert.Len(t, output.Errors, 1, "o id duplicado é reportado uma única vez")
	assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
	assert.Equal(t, []uuid.UUID{other}, output.Data)
	assert.Equal(t, 1, len(repo.Deleted), "nenhuma ocorrência do id duplicado é processada")
}

func whenDeleteNotFound(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		DeleteFunc: func(ctx context.Context, id uuid.UUID) error { return sql.ErrNoRows },
	}
	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.DeleteInput{Requester: mocks.NewRequester(), IDs: []uuid.UUID{uuid.NewV7()}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Errors[0].Type)
		assert.Equal(t, http.StatusNotFound, output.Errors[0].Status)
	}
}

func whenDeleteFailsInDb(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		DeleteFunc: func(ctx context.Context, id uuid.UUID) error { return errors.New("db down") },
	}
	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.DeleteInput{Requester: mocks.NewRequester(), IDs: []uuid.UUID{uuid.NewV7()}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type)
		assert.Equal(t, http.StatusInternalServerError, output.Errors[0].Status)
	}
}

func whenDeleteEmptyInput(t *testing.T) {
	useCase := useCases.NewDeleteApplicationUseCase(&mocks.ApplicationRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.DeleteInput{Requester: mocks.NewRequester()})

	assert.Nil(t, output.Data)
	assert.Empty(t, output.Errors)
}

func whenDeleteMixedBatch(t *testing.T) {
	ok, missing, broken := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	repo := &mocks.ApplicationRepo{
		DeleteFunc: func(ctx context.Context, id uuid.UUID) error {
			switch id {
			case missing:
				return sql.ErrNoRows
			case broken:
				return errors.New("db error")
			}
			return nil
		},
	}
	useCase := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.DeleteInput{
		Requester: mocks.NewRequester(),
		IDs:       []uuid.UUID{ok, {}, missing, broken},
	})

	assert.Equal(t, []uuid.UUID{ok}, output.Data)
	if assert.Len(t, output.Errors, 3) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
		assert.Equal(t, entities.ErrTypeNotFound, output.Errors[1].Type)
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[2].Type)
	}
}
