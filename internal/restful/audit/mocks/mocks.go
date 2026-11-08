package mocks

import (
	"context"
	"sync"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Mocks

// AuditRepo é o fake do repositório da trilha: implementa IAuditLogCreator e
// IAuditLogLister. Sem *Func, Create devolve a própria linha e List devolve uma
// lista vazia. Guarda cada filtro e janela pedidos. Seguro para uso concorrente.
type AuditRepo struct {
	CreateFunc func(ctx context.Context, log entities.AuditLog) (entities.AuditLog, error)
	ListFunc   func(ctx context.Context, filter entities.AuditFilter, limit int) ([]entities.AuditLog, error)

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

var (
	_ contracts.IAuditLogCreator = (*AuditRepo)(nil)
	_ contracts.IAuditLogLister  = (*AuditRepo)(nil)
)

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

// Actions devolve as ações auditadas, na ordem.
func (m *Tx) Actions() []string {
	actions := make([]string, len(m.Entries))
	for i, entry := range m.Entries {
		actions[i] = entry.Action
	}
	return actions
}

var _ contracts.IAuditedTx = (*Tx)(nil)
