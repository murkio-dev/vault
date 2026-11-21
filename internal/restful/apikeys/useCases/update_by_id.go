package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
)

// Builder

// UpdateApiKeyByIDUseCase atende o PATCH e o PUT, como no Identity: a única
// coisa que muda numa chave é o nome. No PATCH (Partial) o nome vazio mantém o
// atual; no PUT ele é obrigatório.
type UpdateApiKeyByIDUseCase struct {
	Repository apikeyscontracts.IApiKeyRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
	Partial    bool
}

func NewUpdateApiKeyByIDUseCase(repository apikeyscontracts.IApiKeyRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx) UpdateApiKeyByIDUseCase {
	return UpdateApiKeyByIDUseCase{Repository: repository, Guard: guard, Tx: tx, Partial: true}
}

func NewUpgradeApiKeyByIDUseCase(repository apikeyscontracts.IApiKeyRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx) UpdateApiKeyByIDUseCase {
	return UpdateApiKeyByIDUseCase{Repository: repository, Guard: guard, Tx: tx}
}

// DTOS

type UpdateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	ID          uuid.UUID
	Name        string
}

type UpdateOutput struct {
	Data  entities.ApiKey `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform renomeia a chave do membro. Chave apagada é 404.
func (u UpdateApiKeyByIDUseCase) Perform(ctx context.Context, input UpdateInput) UpdateOutput {

	key, e := ownedKey(u.Guard, u.Repository, ctx, input.Requester, input.Application, input.Member, input.ID, entities.ActionApiKeyUpdate)
	if e != nil {
		return UpdateOutput{Error: e}
	}
	if key.DeletedAt != nil {
		notFound := entities.NotFoundError("Resource")
		return UpdateOutput{Error: &notFound}
	}

	if u.Partial && input.Name == "" {
		return UpdateOutput{Data: key}
	}

	previous := key.Name
	if err := key.Rename(input.Name); err != nil {
		invalid := entities.ValidationError("The api key is invalid.", entities.InvalidParam{Field: "name", Reason: err.Error()})
		return UpdateOutput{Error: &invalid}
	}

	ctx = entities.WithActor(ctx, input.Requester.Actor)

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditApiKeyUpdated, entities.AuditAllowed,
		entities.AuditTarget{Type: "api_key", ID: key.ID.String()},
		map[string]any{"name": key.Name, "previous_name": previous, "prefix": key.Prefix, "member_id": key.MemberID.String()})

	err := u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		renamed, err := u.Repository.Rename(ctx, key)
		key.UpdatedAt = renamed.UpdatedAt
		return err
	})
	if err != nil {
		return UpdateOutput{Error: failure(err)}
	}

	return UpdateOutput{Data: key}
}
