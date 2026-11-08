package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/mocks"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestCreateApplication(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenCreateBuildSucceeds(t) })
	t.Run("Quando o build recebe repositório nulo.", func(t *testing.T) { whenCreateBuildWithNilRepo(t) })
	t.Run("Quando recebe uma unica entidade valida.", func(t *testing.T) { whenCreateOneEntity(t) })
	t.Run("Quando recebe multiplas entidades validas.", func(t *testing.T) { whenCreateMultipleEntities(t) })
	t.Run("Quando recebe uma unica entidade invalida.", func(t *testing.T) { whenCreateOneInvalidEntity(t) })
	t.Run("Quando recebe multiplas entidades invalidas.", func(t *testing.T) { whenCreateMultipleInvalidEntities(t) })
	t.Run("Quando a aplicação já existe no banco.", func(t *testing.T) { whenCreateAppAlreadyExists(t) })
	t.Run("Quando ocorre erro de banco na criação.", func(t *testing.T) { whenCreateFailsInDb(t) })
	t.Run("Quando ocorre erro de banco na verificação de existência.", func(t *testing.T) { whenCreateExistFailsInDb(t) })
	t.Run("Quando a lista de entrada é vazia.", func(t *testing.T) { whenCreateInputIsEmpty(t) })
	t.Run("Quando recebe um lote misto com sucesso parcial.", func(t *testing.T) { whenCreateMixedBatch(t) })
}

// Functions

func whenCreateBuildSucceeds(t *testing.T) {
	repo := &mocks.ApplicationRepo{}

	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	assert.NotNil(t, useCase.Creator)
	assert.Equal(t, repo, useCase.Creator, "o repositório atribuído deve ser o mesmo passado no construtor")
	assert.Equal(t, repo, useCase.Checker)
}

func whenCreateBuildWithNilRepo(t *testing.T) {
	useCase := useCases.NewCreateApplicationUseCase(nil, nil, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	assert.Nil(t, useCase.Creator, "o repositório interno fica nil quando injetado como nil")
}

func whenCreateOneEntity(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	name := utils.GenRandomApplicationName()
	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester:   mocks.NewRequester(),
		Application: []entities.Application{{Name: name}},
	})

	assert.Empty(t, output.Errors)
	assert.Len(t, output.Data, 1)
	assert.Equal(t, name, output.Data[0].Name)
	assert.NotEmpty(t, output.Data[0].ID, "o usecase deve gerar o ID")
	assert.Equal(t, 1, len(repo.Created))
}

func whenCreateMultipleEntities(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	input := useCases.CreateInput{
		Requester: mocks.NewRequester(),
		Application: []entities.Application{
			{Name: utils.GenRandomApplicationName()},
			{Name: utils.GenRandomApplicationName()},
			{Name: utils.GenRandomApplicationName()},
		},
	}
	output := useCase.Perform(context.Background(), input)

	assert.Empty(t, output.Errors)
	assert.Len(t, output.Data, len(input.Application))
	for i, application := range input.Application {
		assert.Equal(t, application.Name, output.Data[i].Name, "a ordem do lote deve ser preservada")
	}
	assert.Equal(t, 3, len(repo.Created))
}

func whenCreateOneInvalidEntity(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester:   mocks.NewRequester(),
		Application: []entities.Application{{Name: ""}},
	})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
		assert.Equal(t, http.StatusBadRequest, output.Errors[0].Status)
	}
	assert.Zero(t, len(repo.Created), "não deve tentar criar entidade inválida")
}

func whenCreateMultipleInvalidEntities(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester:   mocks.NewRequester(),
		Application: []entities.Application{{Name: ""}, {Name: "A B"}, {Name: "-x-"}},
	})

	assert.Nil(t, output.Data)
	assert.Len(t, output.Errors, 3, "um erro por aplicação inválida")
	for _, e := range output.Errors {
		assert.Equal(t, entities.ErrTypeValidation, e.Type)
	}
	assert.Zero(t, len(repo.Created))
}

func whenCreateAppAlreadyExists(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		ExistFunc: func(ctx context.Context, input entities.Application) (bool, error) { return true, nil },
	}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester:   mocks.NewRequester(),
		Application: []entities.Application{{Name: utils.GenRandomApplicationName()}},
	})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeConflict, output.Errors[0].Type)
		assert.Equal(t, http.StatusConflict, output.Errors[0].Status)
	}
	assert.Zero(t, len(repo.Created), "não deve criar quando já existe")
}

func whenCreateFailsInDb(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		CreateFunc: func(ctx context.Context, input entities.Application) (entities.Application, error) {
			return entities.Application{}, errors.New("database connection failure")
		},
	}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester:   mocks.NewRequester(),
		Application: []entities.Application{{Name: utils.GenRandomApplicationName()}},
	})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type)
		assert.Equal(t, http.StatusInternalServerError, output.Errors[0].Status)
		assert.NotContains(t, output.Errors[0].Detail, "database connection failure", "não deve vazar o erro interno")
	}
}

func whenCreateExistFailsInDb(t *testing.T) {
	repo := &mocks.ApplicationRepo{
		ExistFunc: func(ctx context.Context, input entities.Application) (bool, error) {
			return false, errors.New("connection failed during exist check")
		},
	}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester:   mocks.NewRequester(),
		Application: []entities.Application{{Name: utils.GenRandomApplicationName()}},
	})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type)
	}
	assert.Zero(t, len(repo.Created), "não deve criar se a checagem falhou")
}

func whenCreateInputIsEmpty(t *testing.T) {
	useCase := useCases.NewCreateApplicationUseCase(&mocks.ApplicationRepo{}, &mocks.ApplicationRepo{}, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester:   mocks.NewRequester(),
		Application: []entities.Application{},
	})

	assert.Nil(t, output.Data)
	assert.Empty(t, output.Errors)
}

func whenCreateMixedBatch(t *testing.T) {
	validName := utils.GenRandomApplicationName()
	existsName := utils.GenRandomApplicationName()
	dbErrorName := utils.GenRandomApplicationName()

	repo := &mocks.ApplicationRepo{
		ExistFunc: func(ctx context.Context, input entities.Application) (bool, error) {
			return input.Name == existsName, nil
		},
		CreateFunc: func(ctx context.Context, input entities.Application) (entities.Application, error) {
			if input.Name == dbErrorName {
				return entities.Application{}, errors.New("db creation error")
			}
			return input, nil
		},
	}
	useCase := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{})

	output := useCase.Perform(context.Background(), useCases.CreateInput{
		Requester: mocks.NewRequester(),
		Application: []entities.Application{
			{Name: validName},
			{Name: ""},
			{Name: existsName},
			{Name: dbErrorName},
		},
	})

	if assert.Len(t, output.Data, 1, "salva apenas a aplicação válida") {
		assert.Equal(t, validName, output.Data[0].Name)
	}
	if assert.Len(t, output.Errors, 3) {
		assert.Equal(t, entities.ErrTypeValidation, output.Errors[0].Type)
		assert.Equal(t, entities.ErrTypeConflict, output.Errors[1].Type)
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[2].Type)
	}
}
