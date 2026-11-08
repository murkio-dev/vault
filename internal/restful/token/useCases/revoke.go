package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type RevokeTokenUseCase struct {
	Finder  contracts.ITokenFinder
	Revoker contracts.ITokenRevoker
	Guard   contracts.IAccessGuard
	Tx      contracts.IAuditedTx
}

func NewRevokeTokenUseCase(finder contracts.ITokenFinder, revoker contracts.ITokenRevoker, guard contracts.IAccessGuard, tx contracts.IAuditedTx) RevokeTokenUseCase {
	return RevokeTokenUseCase{Finder: finder, Revoker: revoker, Guard: guard, Tx: tx}
}

// DTOS

type RevokeInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Token       uuid.UUID
}

type RevokeOutput struct {
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform revoga o token; vale na hora, porque o runtime consulta o banco a
// cada requisição. O token tem de ser do membro do caminho (senão 404), então
// um developer não revoga o de outro apontando o id. Revogar duas vezes é
// idempotente e não gera outra linha de auditoria.
func (u RevokeTokenUseCase) Perform(ctx context.Context, input RevokeInput) RevokeOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionTokenRevoke)
	if denied != nil {
		return RevokeOutput{Error: denied}
	}

	ctx = entities.WithActor(ctx, input.Requester.Actor)

	token, err := u.Finder.FindById(ctx, input.Application, input.Token)
	if err != nil || token.MemberID != owner {
		notFound := entities.NotFoundError("Resource")
		return RevokeOutput{Error: &notFound}
	}

	if token.RevokedAt != nil {
		return RevokeOutput{}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditTokenRevoked, entities.AuditAllowed,
		entities.AuditTarget{Type: "token", ID: token.ID.String()}, map[string]any{"name": token.Name, "prefix": token.Prefix, "member_id": owner.String()})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		return u.Revoker.Revoke(ctx, input.Application, token.ID)
	})
	if err != nil {
		return RevokeOutput{Error: failure(err)}
	}

	return RevokeOutput{}
}
