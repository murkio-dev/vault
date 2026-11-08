package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type ListSecretsUseCase struct {
	Lister contracts.ISecretLister
	Guard  contracts.IAccessGuard
}

func NewListSecretsUseCase(lister contracts.ISecretLister, guard contracts.IAccessGuard) ListSecretsUseCase {
	return ListSecretsUseCase{Lister: lister, Guard: guard}
}

// DTOS

type ListInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
}

type ListOutput struct {
	Data  []entities.Secret `json:"data"`
	Error *entities.Error   `json:"error,omitempty"`
}

// UseCase

// Perform lista os METADADOS dos segredos do membro: nome, tipo, origem e
// versão. O valor não existe em entities.Secret.
func (u ListSecretsUseCase) Perform(ctx context.Context, input ListInput) ListOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretRead)
	if denied != nil {
		return ListOutput{Error: denied}
	}

	secrets, _, err := u.Lister.List(ctx, input.Application, owner, entities.MaxSecretsPerTable, 0)
	if err != nil {
		return ListOutput{Error: failure(err)}
	}

	return ListOutput{Data: secrets}
}
