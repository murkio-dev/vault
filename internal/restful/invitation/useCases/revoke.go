package useCases

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type RevokeInvitationUseCase struct {
	Revoker contracts.IInvitationRevoker
	Guard   contracts.IAccessGuard
	Tx      contracts.IAuditedTx
}

func NewRevokeInvitationUseCase(revoker contracts.IInvitationRevoker, guard contracts.IAccessGuard, tx contracts.IAuditedTx) RevokeInvitationUseCase {
	return RevokeInvitationUseCase{Revoker: revoker, Guard: guard, Tx: tx}
}

// DTOS

type RevokeInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Invitation  uuid.UUID
}

type RevokeOutput struct {
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

func (u RevokeInvitationUseCase) Perform(ctx context.Context, input RevokeInput) RevokeOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionInvitationManage}).Error; denied != nil {
		return RevokeOutput{Error: denied}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditInvitationRevoked, entities.AuditAllowed,
		entities.AuditTarget{Type: "invitation", ID: input.Invitation.String()}, nil)

	err := u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		return u.Revoker.Revoke(ctx, input.Application, input.Invitation)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFound := entities.NotFoundError("Resource")
			return RevokeOutput{Error: &notFound}
		}
		internal := entities.InternalServerError(err, nil)
		return RevokeOutput{Error: &internal}
	}

	return RevokeOutput{}
}
