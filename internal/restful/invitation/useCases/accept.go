package useCases

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type AcceptInvitationUseCase struct {
	TokenFinder   contracts.IInvitationTokenFinder
	Acceptor      contracts.IInvitationAcceptor
	MemberCreator contracts.IMemberCreator
	MemberFinder  contracts.IMemberUserFinder
	Guard         contracts.IAccessGuard
	Tx            contracts.IAuditedTx
	Audit         contracts.IAuditLogCreator
}

func NewAcceptInvitationUseCase(tokenFinder contracts.IInvitationTokenFinder, acceptor contracts.IInvitationAcceptor, memberCreator contracts.IMemberCreator, memberFinder contracts.IMemberUserFinder, guard contracts.IAccessGuard, tx contracts.IAuditedTx, audit contracts.IAuditLogCreator) AcceptInvitationUseCase {
	return AcceptInvitationUseCase{TokenFinder: tokenFinder, Acceptor: acceptor, MemberCreator: memberCreator, MemberFinder: memberFinder, Guard: guard, Tx: tx, Audit: audit}
}

// DTOS

type AcceptInput struct {
	Requester entities.ApiHeader
	Token     string
}

type AcceptOutput struct {
	Data  entities.Member `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform transforma o convite num vínculo da conta autenticada com a
// aplicação. O token decide: inexistente, errado, vencido, usado e revogado
// respondem todos 404 igual, e a tentativa que falha fica na auditoria. O
// aceite e a criação do membro são uma transação só, e o UPDATE condicional do
// convite faz dois aceites simultâneos do mesmo link terminarem com um só
// membro.
func (u AcceptInvitationUseCase) Perform(ctx context.Context, input AcceptInput) AcceptOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Action: entities.ActionInvitationAccept}).Error; denied != nil {
		return AcceptOutput{Error: denied}
	}

	actor := input.Requester.Actor
	now := time.Now().UTC()

	invitation, err := u.find(ctx, input.Token)
	if err != nil || invitation.Status(now) != entities.InvitationPending {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			internal := entities.InternalServerError(err, nil)
			return AcceptOutput{Error: &internal}
		}
		u.deny(ctx, input.Requester, invitation)
		notFound := entities.NotFoundError("Resource")
		return AcceptOutput{Error: &notFound}
	}

	_, err = u.MemberFinder.FindByUser(ctx, invitation.ApplicationID, actor.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		internal := entities.InternalServerError(err, nil)
		return AcceptOutput{Error: &internal}
	}
	if err == nil {
		conflict := entities.ConflictError("Already a member", "This account already belongs to the application.")
		return AcceptOutput{Error: &conflict}
	}

	invitedBy := invitation.CreatedBy
	member := entities.NewMember(invitation.ApplicationID, actor.UserID, invitation.Role, &invitedBy)
	entry := entities.NewAuditLog(input.Requester, invitation.ApplicationID, entities.AuditInvitationAccepted, entities.AuditAllowed,
		entities.AuditTarget{Type: "invitation", ID: invitation.ID.String()}, map[string]any{"member_id": member.ID.String(), "role": string(member.Role)})

	accepted := entities.Invitation{ID: invitation.ID, ApplicationID: invitation.ApplicationID, AcceptedAt: &now, AcceptedBy: &actor.UserID}
	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		if _, err := u.Acceptor.Accept(ctx, accepted); err != nil {
			return err
		}
		_, err := u.MemberCreator.Create(ctx, member)
		return err
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFound := entities.NotFoundError("Resource")
			return AcceptOutput{Error: &notFound}
		}
		internal := entities.InternalServerError(err, nil)
		return AcceptOutput{Error: &internal}
	}

	return AcceptOutput{Data: member}
}

// find procura o convite pelo hash do token: quem aceita ainda não sabe a
// aplicação, o token é a credencial. Token sem o prefixo ou grande demais nem
// chega ao banco.
func (u AcceptInvitationUseCase) find(ctx context.Context, token string) (entities.Invitation, error) {
	if !strings.HasPrefix(token, entities.InvitationTokenPrefix) || len(token) > 128 {
		return entities.Invitation{}, sql.ErrNoRows
	}

	return u.TokenFinder.FindByTokenHash(ctx, entities.HashToken(token))
}

// deny registra a tentativa que falhou. O alvo é o convite, se o token chegou a
// achar um; a aplicação fica de fora da resposta, mas entra na trilha.
func (u AcceptInvitationUseCase) deny(ctx context.Context, requester entities.ApiHeader, invitation entities.Invitation) {
	target := entities.AuditTarget{}
	if invitation.ID != (uuid.UUID{}) {
		target = entities.AuditTarget{Type: "invitation", ID: invitation.ID.String()}
	}
	_, _ = u.Audit.Create(ctx, entities.NewAuditLog(requester, invitation.ApplicationID, entities.AuditAccessDenied, entities.AuditDenied,
		target, map[string]any{"action": string(entities.ActionInvitationAccept)}))
}
