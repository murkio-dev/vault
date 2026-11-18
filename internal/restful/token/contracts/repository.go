package tokencontracts

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

type ITokenRepository interface {
	Create(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error)
	// FindActiveByHash é a autenticação do SDK: acha o token válido agora (não
	// revogado, não vencido, de membro ativo) pelo hash.
	FindActiveByHash(ctx context.Context, tokenHash string) (entities.AccessToken, error)
	FindById(ctx context.Context, application, id uuid.UUID) (entities.AccessToken, error)
	List(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.AccessToken, int, error)
	// Revoke revoga o token; é idempotente.
	Revoke(ctx context.Context, application, id uuid.UUID) error
	TouchLastUsed(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error)
}
