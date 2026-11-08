package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type CreateInvitationUseCase struct {
	Creator contracts.IInvitationCreator
	Outbox  contracts.IOutboxCreator
	Guard   contracts.IAccessGuard
	Tx      contracts.IAuditedTx
}

func NewCreateInvitationUseCase(creator contracts.IInvitationCreator, outbox contracts.IOutboxCreator, guard contracts.IAccessGuard, tx contracts.IAuditedTx) CreateInvitationUseCase {
	return CreateInvitationUseCase{Creator: creator, Outbox: outbox, Guard: guard, Tx: tx}
}

// DTOS

type CreateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Email       string
	Role        entities.Role
}

type CreateOutput struct {
	Data  entities.InvitationCreated `json:"data"`
	Error *entities.Error            `json:"error,omitempty"`
}

// UseCase

// Perform cria o convite, o pedido de e-mail na outbox e a auditoria numa
// transação só. O token aparece apenas nesta resposta: o banco guarda o hash e
// o payload da outbox, que também o leva, é apagado depois de publicado.
func (u CreateInvitationUseCase) Perform(ctx context.Context, input CreateInput) CreateOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionInvitationManage}).Error; denied != nil {
		return CreateOutput{Error: denied}
	}

	invitation, token, err := entities.NewInvitation(input.Application, input.Email, input.Role, input.Requester.Actor.UserID)
	if err != nil {
		invalid := entities.ValidationError("The invitation is invalid.", entities.InvalidParam{Field: "email or role", Reason: err.Error()})
		return CreateOutput{Error: &invalid}
	}

	email, err := entities.NewInvitationEmail(invitation, token)
	if err != nil {
		internal := entities.InternalServerError(err, nil)
		return CreateOutput{Error: &internal}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditInvitationCreated, entities.AuditAllowed,
		entities.AuditTarget{Type: "invitation", ID: invitation.ID.String()}, map[string]any{"role": string(invitation.Role)})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		if _, err := u.Creator.Create(ctx, invitation); err != nil {
			return err
		}
		_, err := u.Outbox.Create(ctx, email)
		return err
	})
	if err != nil {
		internal := entities.InternalServerError(err, nil)
		return CreateOutput{Error: &internal}
	}

	return CreateOutput{Data: entities.InvitationCreated{Invitation: invitation, Token: token}}
}
