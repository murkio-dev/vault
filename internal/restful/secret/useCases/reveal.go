package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type RevealSecretUseCase struct {
	OwnerFinder contracts.ISecretOwnerFinder
	Values      contracts.ISecretValues
	Guard       contracts.IAccessGuard
	Tx          contracts.IAuditedTx
}

func NewRevealSecretUseCase(ownerFinder contracts.ISecretOwnerFinder, values contracts.ISecretValues, guard contracts.IAccessGuard, tx contracts.IAuditedTx) RevealSecretUseCase {
	return RevealSecretUseCase{OwnerFinder: ownerFinder, Values: values, Guard: guard, Tx: tx}
}

// DTOS

type RevealInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Secret      uuid.UUID
	// Version 0 é a atual.
	Version int
}

type RevealOutput struct {
	Data  entities.RevealedSecret `json:"data"`
	Error *entities.Error         `json:"error,omitempty"`
}

// UseCase

// Perform devolve o valor. É a única ação, junto com o runtime, que o faz, e a
// leitura é auditada NA MESMA transação que a produz: se a trilha não puder ser
// gravada, o valor não sai. A auditoria leva nome e versão, nunca o valor.
func (u RevealSecretUseCase) Perform(ctx context.Context, input RevealInput) RevealOutput {

	if input.Version < 0 {
		return RevealOutput{Error: invalid("version", "must be a positive integer")}
	}

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretReveal)
	if denied != nil {
		return RevealOutput{Error: denied}
	}

	secret, err := u.OwnerFinder.FindByOwner(ctx, input.Application, owner, input.Secret)
	if err != nil {
		return RevealOutput{Error: failure(err)}
	}

	metadata := map[string]any{"name": secret.Name, "member_id": owner.String()}
	if input.Version > 0 {
		metadata["version"] = input.Version
	} else {
		metadata["version"] = secret.Version
	}
	if secret.ReferenceSecretID != nil {
		metadata["resolved_from"] = secret.ReferenceSecretID.String()
	}
	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditSecretRevealed, entities.AuditAllowed,
		entities.AuditTarget{Type: "secret", ID: secret.ID.String()}, metadata)

	var revealed entities.RevealedSecret
	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		value, number, err := u.Values.Value(ctx, secret, input.Version)
		if err != nil {
			return err
		}
		revealed = entities.RevealedSecret{ID: secret.ID, Name: secret.Name, Version: number, Value: value}
		return nil
	})
	if err != nil {
		return RevealOutput{Error: failure(err)}
	}

	return RevealOutput{Data: revealed}
}
