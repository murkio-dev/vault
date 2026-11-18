package membercontracts

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

type IMemberRepository interface {
	CountActiveAdmins(ctx context.Context, application uuid.UUID) (int, error)
	Create(ctx context.Context, member entities.Member) (entities.Member, error)
	Delete(ctx context.Context, application, id uuid.UUID) error
	FindById(ctx context.Context, application, id uuid.UUID) (entities.Member, error)
	FindByUser(ctx context.Context, application, user uuid.UUID) (entities.Member, error)
	List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Member, int, error)
	Upgrade(ctx context.Context, member entities.Member) (entities.Member, error)
}
