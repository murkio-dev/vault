package contracts

import (
	"context"

	"github.com/murkio-dev/vault/internal/entities"
)

// IAuditedTx roda fn numa transação e grava entry na trilha na MESMA
// transação: se fn falhar, a auditoria não é gravada; se a auditoria falhar, a
// mudança é desfeita. É a forma de todo use case alterar algo, e o que torna a
// auditoria fail-closed.
type IAuditedTx interface {
	Run(ctx context.Context, entry entities.AuditLog, fn func(ctx context.Context) error) error
}
