package invitationcontracts

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

type IInvitationRepository interface {
	Accept(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error)
	Create(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error)
	// FindByTokenHash acha o convite pelo hash do token (a credencial de
	// quem aceita ainda não tem aplicação).
	FindByTokenHash(ctx context.Context, tokenHash string) (entities.Invitation, error)
	List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Invitation, int, error)
	Revoke(ctx context.Context, application, id uuid.UUID) error
}

type IOutboxRepository interface {
	Create(ctx context.Context, event entities.OutboxEvent) (entities.OutboxEvent, error)
}
