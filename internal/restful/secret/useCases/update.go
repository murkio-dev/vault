package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	secretcontracts "github.com/murkio-dev/vault/internal/restful/secret/contracts"
)

// Builder

type UpdateSecretUseCase struct {
	Repository secretcontracts.ISecretRepository
	Values     contracts.ISecretValues
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
}

func NewUpdateSecretUseCase(repository secretcontracts.ISecretRepository, values contracts.ISecretValues, guard contracts.IAccessGuard, tx contracts.IAuditedTx) UpdateSecretUseCase {
	return UpdateSecretUseCase{Repository: repository, Values: values, Guard: guard, Tx: tx}
}

// DTOS

type UpdateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Secret      uuid.UUID
	Value       string
}

type UpdateOutput struct {
	Data  entities.Secret `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform troca o valor criando uma NOVA versão; as anteriores ficam. Só
// segredo estático tem valor próprio. Se outra gravação chegar entre a leitura
// e a escrita, o UPDATE condicional falha e a resposta é 404 em vez de duas
// versões com o mesmo número.
func (u UpdateSecretUseCase) Perform(ctx context.Context, input UpdateInput) UpdateOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionSecretWrite)
	if denied != nil {
		return UpdateOutput{Error: denied}
	}

	secret, err := u.Repository.FindByOwner(ctx, input.Application, owner, input.Secret)
	if err != nil {
		return UpdateOutput{Error: failure(err)}
	}

	if secret.Source != entities.SourceStatic {
		return UpdateOutput{Error: invalid("value", "only a static secret has a value of its own")}
	}
	if err := entities.ValidateSecretValue(secret.Type, input.Value); err != nil {
		return UpdateOutput{Error: invalid("value", err.Error())}
	}

	versionID := uuid.NewV7()
	ciphertext, err := u.Values.Seal(versionID, input.Value)
	if err != nil {
		return UpdateOutput{Error: failure(err)}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditSecretUpdated, entities.AuditAllowed,
		entities.AuditTarget{Type: "secret", ID: secret.ID.String()},
		map[string]any{"name": secret.Name, "version": secret.Version + 1, "member_id": owner.String()})

	var updated entities.Secret
	err = u.Tx.Run(ctx, entry, func(ctx context.Context) (err error) {
		updated, err = u.Repository.AddVersion(ctx,
			entities.SecretWrite{Secret: secret, VersionID: versionID, Ciphertext: ciphertext, Author: input.Requester.Actor.UserID})
		return err
	})
	if err != nil {
		return UpdateOutput{Error: failure(err)}
	}

	return UpdateOutput{Data: updated}
}
