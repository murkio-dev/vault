package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	secretcontracts "github.com/murkio-dev/vault/internal/restful/secret/contracts"
)

// Builder

type GetSecretUseCase struct {
	Repository secretcontracts.ISecretRepository
	Guard      contracts.IAccessGuard
}

func NewGetSecretUseCase(repository secretcontracts.ISecretRepository, guard contracts.IAccessGuard) GetSecretUseCase {
	return GetSecretUseCase{Repository: repository, Guard: guard}
}

// DTOS

type GetInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Secret      uuid.UUID
}

type GetOutput struct {
	Data  entities.Secret `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

func (u GetSecretUseCase) Perform(ctx context.Context, input GetInput) GetOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretRead)
	if denied != nil {
		return GetOutput{Error: denied}
	}

	secret, err := u.Repository.FindByOwner(ctx, input.Application, owner, input.Secret)
	if err != nil {
		return GetOutput{Error: failure(err)}
	}

	return GetOutput{Data: secret}
}
