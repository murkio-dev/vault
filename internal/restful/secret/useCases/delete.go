package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	secretcontracts "github.com/murkio-dev/vault/internal/restful/secret/contracts"
)

// Builder

type DeleteSecretUseCase struct {
	Repository secretcontracts.ISecretRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
}

func NewDeleteSecretUseCase(repository secretcontracts.ISecretRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx) DeleteSecretUseCase {
	return DeleteSecretUseCase{Repository: repository, Guard: guard, Tx: tx}
}

// DTOS

type DeleteInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Secret      uuid.UUID
}

type DeleteOutput struct {
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform apaga o segredo (soft delete: as versões cifradas ficam para a
// auditoria). Se outros segredos apontam para ele, é conflito: apagá-lo
// quebraria o runtime deles. A contagem (CountReferences) e o delete estão na mesma transação.
func (u DeleteSecretUseCase) Perform(ctx context.Context, input DeleteInput) DeleteOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretWrite)
	if denied != nil {
		return DeleteOutput{Error: denied}
	}

	secret, err := u.Repository.FindByOwner(ctx, input.Application, owner, input.Secret)
	if err != nil {
		return DeleteOutput{Error: failure(err)}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditSecretDeleted, entities.AuditAllowed,
		entities.AuditTarget{Type: "secret", ID: secret.ID.String()}, map[string]any{"name": secret.Name, "member_id": owner.String()})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		references, err := u.Repository.CountReferences(ctx, input.Application, secret.ID)
		if err != nil {
			return err
		}
		if references > 0 {
			return errHasReferences
		}
		return u.Repository.Delete(ctx, input.Application, secret.ID)
	})
	if err != nil {
		return DeleteOutput{Error: failure(err)}
	}

	return DeleteOutput{}
}
