package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	auditcontracts "github.com/murkio-dev/vault/internal/restful/audit/contracts"
)

// Mocks

// AuditRepo é o fake do repositório da trilha: implementa IAuditLogRepository.
// Sem *Func, Create devolve a própria linha, List devolve uma lista vazia e
// PurgeExpired não apaga nada. Guarda cada filtro e janela pedidos. Seguro para
// uso concorrente.
type AuditRepo struct {
	CreateFunc func(ctx context.Context, log entities.AuditLog) (entities.AuditLog, error)
	ListFunc   func(ctx context.Context, filter entities.AuditFilter, limit int) ([]entities.AuditLog, error)
	PurgeFunc  func(ctx context.Context, now time.Time, limit int) (int64, error)

	mu      sync.Mutex
	Created []entities.AuditLog
	Filters []entities.AuditFilter
	Windows []int
}

func (m *AuditRepo) Create(ctx context.Context, log entities.AuditLog) (entities.AuditLog, error) {
	if m.CreateFunc != nil {
		out, err := m.CreateFunc(ctx, log)
		if err == nil {
			m.mu.Lock()
			m.Created = append(m.Created, log)
			m.mu.Unlock()
		}
		return out, err
	}
	m.mu.Lock()
	m.Created = append(m.Created, log)
	m.mu.Unlock()
	return log, nil
}

func (m *AuditRepo) List(ctx context.Context, filter entities.AuditFilter, limit int) ([]entities.AuditLog, error) {
	m.mu.Lock()
	m.Filters = append(m.Filters, filter)
	m.Windows = append(m.Windows, limit)
	m.mu.Unlock()
	if m.ListFunc != nil {
		return m.ListFunc(ctx, filter, limit)
	}
	return []entities.AuditLog{}, nil
}

func (m *AuditRepo) PurgeExpired(ctx context.Context, now time.Time, limit int) (int64, error) {
	if m.PurgeFunc != nil {
		return m.PurgeFunc(ctx, now, limit)
	}
	return 0, nil
}

var _ auditcontracts.IAuditLogRepository = (*AuditRepo)(nil)

// Tx é um fake de contracts.IAuditedTx: roda fn e guarda cada entrada de
// auditoria. Com AuditErr, simula a falha de gravar a trilha: a mudança roda e
// depois "desfaz" (o erro volta ao use case, como na transação real).
type Tx struct {
	AuditErr error

	Entries []entities.AuditLog
	Runs    int
}

func (m *Tx) Run(ctx context.Context, entry entities.AuditLog, fn func(ctx context.Context) error) error {
	m.Runs++
	if err := fn(ctx); err != nil {
		return err
	}
	if m.AuditErr != nil {
		return m.AuditErr
	}
	m.Entries = append(m.Entries, entry)
	return nil
}

func (m *Tx) Actions() []string {
	actions := make([]string, len(m.Entries))
	for i, entry := range m.Entries {
		actions[i] = entry.Action
	}
	return actions
}

var _ contracts.IAuditedTx = (*Tx)(nil)
