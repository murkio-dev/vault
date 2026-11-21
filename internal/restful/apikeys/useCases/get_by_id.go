package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
)

// Builder

type GetApiKeyByIDUseCase struct {
	Repository apikeyscontracts.IApiKeyRepository
	Guard      contracts.IAccessGuard
}

func NewGetApiKeyByIDUseCase(repository apikeyscontracts.IApiKeyRepository, guard contracts.IAccessGuard) GetApiKeyByIDUseCase {
	return GetApiKeyByIDUseCase{Repository: repository, Guard: guard}
}

// DTOS

type GetInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	ID          uuid.UUID
}

type GetOutput struct {
	Data  entities.ApiKey `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform devolve a chave do membro, mesmo apagada, sem o valor.
func (u GetApiKeyByIDUseCase) Perform(ctx context.Context, input GetInput) GetOutput {

	key, e := ownedKey(u.Guard, u.Repository, ctx, input.Requester, input.Application, input.Member, input.ID, entities.ActionApiKeyRead)
	if e != nil {
		return GetOutput{Error: e}
	}

	return GetOutput{Data: key}
}
