package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	tokencontracts "github.com/murkio-dev/vault/internal/restful/token/contracts"
)

const MaxListed = 200

// Builder

type ListTokensUseCase struct {
	Repository tokencontracts.ITokenRepository
	Guard      contracts.IAccessGuard
}

func NewListTokensUseCase(repository tokencontracts.ITokenRepository, guard contracts.IAccessGuard) ListTokensUseCase {
	return ListTokensUseCase{Repository: repository, Guard: guard}
}

// DTOS

type ListInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
}

type ListOutput struct {
	Data  []entities.AccessToken `json:"data"`
	Error *entities.Error        `json:"error,omitempty"`
}

// UseCase

// Perform lista os tokens do membro (inclusive os revogados, para a pessoa ver
// o histórico). Nenhum traz o token: só o prefixo.
func (u ListTokensUseCase) Perform(ctx context.Context, input ListInput) ListOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionTokenList)
	if denied != nil {
		return ListOutput{Error: denied}
	}

	tokens, _, err := u.Repository.List(ctx, input.Application, owner, MaxListed, 0)
	if err != nil {
		return ListOutput{Error: failure(err)}
	}

	return ListOutput{Data: tokens}
}
