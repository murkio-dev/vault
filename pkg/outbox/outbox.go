package outbox

import (
	"context"
	"database/sql"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// Builder

type Outbox struct {
	DB *sql.DB
}

func New(db *sql.DB) Outbox {
	return Outbox{DB: db}
}

var _ contracts.IOutboxCreator = Outbox{}

// Functions

// Create grava o pedido para outro serviço. Dentro de uma transação
// (IAuditedTx), entra nela: ou a mudança e o pedido são gravados, ou nenhum. A
// leitura e a publicação são do relay externo, não deste serviço.
func (r Outbox) Create(ctx context.Context, event entities.OutboxEvent) (entities.OutboxEvent, error) {
	const query = `
		INSERT INTO outbox_events (id, application_id, type, payload, exchange, envelope, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, query, event.ID.String(), event.ApplicationID.String(),
		event.Type, string(event.Payload), event.Exchange, event.Envelope, event.CreatedAt)
	if err != nil {
		return entities.OutboxEvent{}, err
	}

	return event, nil
}
