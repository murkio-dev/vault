package useCases

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	applicationcontracts "github.com/murkio-dev/vault/internal/restful/application/contracts"
)

// Builder

type GetApplicationByIDUseCase struct {
	Repository applicationcontracts.IApplicationRepository
	Guard      contracts.IAccessGuard
}

func NewGetApplicationByIDUseCase(repository applicationcontracts.IApplicationRepository, guard contracts.IAccessGuard) GetApplicationByIDUseCase {
	return GetApplicationByIDUseCase{Repository: repository, Guard: guard}
}

// DTOS

type GetByIDInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
}

type GetByIDOutput struct {
	Data  entities.Application `json:"data,omitempty"`
	Error *entities.Error      `json:"error,omitempty"`
}

// UseCase

func (u GetApplicationByIDUseCase) Perform(ctx context.Context, input GetByIDInput) GetByIDOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionApplicationRead}).Error; denied != nil {
		return GetByIDOutput{Error: denied}
	}

	data, err := u.Repository.FindById(ctx, input.Application)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := entities.ApplicationNotFoundError(input.Application)
			return GetByIDOutput{Error: &notFoundErr}
		}

		internalErr := entities.InternalServerError(err, nil)
		return GetByIDOutput{Error: &internalErr}
	}

	return GetByIDOutput{Data: data}
}
