package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	secretcontracts "github.com/murkio-dev/vault/internal/restful/secret/contracts"
)

// Builder

type RenameSecretUseCase struct {
	Repository secretcontracts.ISecretRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
}

func NewRenameSecretUseCase(repository secretcontracts.ISecretRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx) RenameSecretUseCase {
	return RenameSecretUseCase{Repository: repository, Guard: guard, Tx: tx}
}

// DTOS

type RenameInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Secret      uuid.UUID
	Name        string
}

type RenameOutput struct {
	Data  entities.Secret `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform muda o nome (a chave do ambiente). Nome repetido na tabela do membro
// é conflito. O valor e as versões não mudam.
func (u RenameSecretUseCase) Perform(ctx context.Context, input RenameInput) RenameOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretWrite)
	if denied != nil {
		return RenameOutput{Error: denied}
	}

	if err := entities.ValidateSecretName(input.Name); err != nil {
		return RenameOutput{Error: invalid("name", err.Error())}
	}

	secret, err := u.Repository.FindByOwner(ctx, input.Application, owner, input.Secret)
	if err != nil {
		return RenameOutput{Error: failure(err)}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditSecretRenamed, entities.AuditAllowed,
		entities.AuditTarget{Type: "secret", ID: secret.ID.String()}, map[string]any{"from": secret.Name, "to": input.Name})

	var renamed entities.Secret
	err = u.Tx.Run(ctx, entry, func(ctx context.Context) (err error) {
		secret.Name = input.Name
		renamed, err = u.Repository.Rename(ctx, entities.SecretWrite{Secret: secret})
		return err
	})
	if err != nil {
		return RenameOutput{Error: failure(err)}
	}

	return RenameOutput{Data: renamed}
}
