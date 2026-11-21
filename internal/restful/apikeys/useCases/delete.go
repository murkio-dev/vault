package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
)

// Builder

type DeleteApiKeyByIDUseCase struct {
	Repository apikeyscontracts.IApiKeyRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
}

func NewDeleteApiKeyByIDUseCase(repository apikeyscontracts.IApiKeyRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx) DeleteApiKeyByIDUseCase {
	return DeleteApiKeyByIDUseCase{Repository: repository, Guard: guard, Tx: tx}
}

// DTOS

type DeleteInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	ID          uuid.UUID
}

type DeleteOutput struct {
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform revoga a chave; vale na hora, porque o runtime consulta o banco a
// cada requisição. Apagar duas vezes é idempotente e não gera outra linha de
// auditoria.
func (u DeleteApiKeyByIDUseCase) Perform(ctx context.Context, input DeleteInput) DeleteOutput {

	key, e := ownedKey(u.Guard, u.Repository, ctx, input.Requester, input.Application, input.Member, input.ID, entities.ActionApiKeyDelete)
	if e != nil {
		return DeleteOutput{Error: e}
	}

	if key.DeletedAt != nil {
		return DeleteOutput{}
	}

	ctx = entities.WithActor(ctx, input.Requester.Actor)

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditApiKeyRevoked, entities.AuditAllowed,
		entities.AuditTarget{Type: "api_key", ID: key.ID.String()}, map[string]any{"name": key.Name, "type": key.Type, "prefix": key.Prefix, "member_id": key.MemberID.String()})

	err := u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		return u.Repository.Delete(ctx, input.Application, key.ID)
	})
	if err != nil {
		return DeleteOutput{Error: failure(err)}
	}

	return DeleteOutput{}
}
