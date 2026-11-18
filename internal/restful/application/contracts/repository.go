package applicationcontracts

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

type IApplicationRepository interface {
	Create(ctx context.Context, application entities.Application) (entities.Application, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Exist(ctx context.Context, application entities.Application) (bool, error)
	FindById(ctx context.Context, id uuid.UUID) (entities.Application, error)
	List(ctx context.Context, limit, offset int) ([]entities.Application, int, error)
	ListByUser(ctx context.Context, user uuid.UUID, limit, offset int) ([]entities.Application, int, error)
	Update(ctx context.Context, application entities.Application) (entities.Application, error)
	Upgrade(ctx context.Context, application entities.Application) (entities.Application, error)
}
