package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	secretcontracts "github.com/murkio-dev/vault/internal/restful/secret/contracts"
)

// Builder

type ListVersionsUseCase struct {
	Repository secretcontracts.ISecretRepository
	Versions   secretcontracts.IVersionRepository
	Guard      contracts.IAccessGuard
}

func NewListVersionsUseCase(repository secretcontracts.ISecretRepository, versions secretcontracts.IVersionRepository, guard contracts.IAccessGuard) ListVersionsUseCase {
	return ListVersionsUseCase{Repository: repository, Versions: versions, Guard: guard}
}

// DTOS

type VersionsInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Secret      uuid.UUID
}

type VersionsOutput struct {
	Data  []entities.SecretVersion `json:"data"`
	Error *entities.Error          `json:"error,omitempty"`
}

// UseCase

// Perform lista as versões (número, autor e data), da mais nova para a mais
// antiga. Nenhuma traz o valor: para vê-lo existe o reveal, que é auditado. O
// ciphertext que o repositório devolve não sai (json:"-").
func (u ListVersionsUseCase) Perform(ctx context.Context, input VersionsInput) VersionsOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretRead)
	if denied != nil {
		return VersionsOutput{Error: denied}
	}

	secret, err := u.Repository.FindByOwner(ctx, input.Application, owner, input.Secret)
	if err != nil {
		return VersionsOutput{Error: failure(err)}
	}

	versions, _, err := u.Versions.List(ctx, input.Application, secret.ID, entities.MaxVersionsListed, 0)
	if err != nil {
		return VersionsOutput{Error: failure(err)}
	}

	return VersionsOutput{Data: versions}
}
