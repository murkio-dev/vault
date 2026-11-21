package apikeyscontracts

import (
	"context"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

type IApiKeyRepository interface {
	Create(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error)
	// FindActiveByHash é a autenticação do SDK: acha a chave secreta válida
	// agora (não apagada, não vencida, de membro ativo) pelo hash. Uma chave
	// publishable nunca é achada aqui.
	FindActiveByHash(ctx context.Context, hash string) (entities.ApiKey, error)
	FindById(ctx context.Context, application, id uuid.UUID) (entities.ApiKey, error)
	List(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.ApiKey, int, error)
	// Rename grava o nome novo da chave.
	Rename(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error)
	// Expire antecipa o vencimento da chave (é o fim da chave antiga numa rotação).
	Expire(ctx context.Context, application, id uuid.UUID, at time.Time) error
	// Delete revoga a chave; é idempotente.
	Delete(ctx context.Context, application, id uuid.UUID) error
	TouchLastUsed(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error)
}
