package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type DeleteSecretUseCase struct {
	OwnerFinder      contracts.ISecretOwnerFinder
	ReferenceCounter contracts.ISecretReferenceCounter
	Deleter          contracts.ISecretDeleter
	Guard            contracts.IAccessGuard
	Tx               contracts.IAuditedTx
}

func NewDeleteSecretUseCase(ownerFinder contracts.ISecretOwnerFinder, referenceCounter contracts.ISecretReferenceCounter, deleter contracts.ISecretDeleter, guard contracts.IAccessGuard, tx contracts.IAuditedTx) DeleteSecretUseCase {
	return DeleteSecretUseCase{OwnerFinder: ownerFinder, ReferenceCounter: referenceCounter, Deleter: deleter, Guard: guard, Tx: tx}
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
// quebraria o runtime deles. A contagem (o total do List filtrado por
// ReferenceOf) e o delete estão na mesma transação.
func (u DeleteSecretUseCase) Perform(ctx context.Context, input DeleteInput) DeleteOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretWrite)
	if denied != nil {
		return DeleteOutput{Error: denied}
	}

	secret, err := u.OwnerFinder.FindByOwner(ctx, input.Application, owner, input.Secret)
	if err != nil {
		return DeleteOutput{Error: failure(err)}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditSecretDeleted, entities.AuditAllowed,
		entities.AuditTarget{Type: "secret", ID: secret.ID.String()}, map[string]any{"name": secret.Name, "member_id": owner.String()})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		references, err := u.ReferenceCounter.CountReferences(ctx, input.Application, secret.ID)
		if err != nil {
			return err
		}
		if references > 0 {
			return errHasReferences
		}
		return u.Deleter.Delete(ctx, input.Application, secret.ID)
	})
	if err != nil {
		return DeleteOutput{Error: failure(err)}
	}

	return DeleteOutput{}
}
