package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
)

const MaxListed = 200

// Builder

type ListApiKeysUseCase struct {
	Repository apikeyscontracts.IApiKeyRepository
	Guard      contracts.IAccessGuard
}

func NewListApiKeysUseCase(repository apikeyscontracts.IApiKeyRepository, guard contracts.IAccessGuard) ListApiKeysUseCase {
	return ListApiKeysUseCase{Repository: repository, Guard: guard}
}

// DTOS

type ListInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
}

type ListOutput struct {
	Data  []entities.ApiKey `json:"data"`
	Error *entities.Error   `json:"error,omitempty"`
}

// UseCase

// Perform lista as chaves do membro (inclusive as apagadas, para a pessoa ver
// o histórico). Nenhuma traz o valor: só o prefixo.
func (u ListApiKeysUseCase) Perform(ctx context.Context, input ListInput) ListOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionApiKeyRead)
	if denied != nil {
		return ListOutput{Error: denied}
	}

	keys, _, err := u.Repository.List(ctx, input.Application, owner, MaxListed, 0)
	if err != nil {
		return ListOutput{Error: failure(err)}
	}

	return ListOutput{Data: keys}
}
