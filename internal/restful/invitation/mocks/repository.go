package mocks

import (
	"context"
	"database/sql"
	"sync"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

// Mocks

// InvitationRepo é o fake do repositório de convites: implementa todos os
// contratos de convite. Sem *Func, Create e Accept devolvem o próprio convite,
// FindByTokenHash devolve sql.ErrNoRows, List vem vazia e Revoke dá certo.
// Guarda os argumentos das chamadas. Seguro para uso concorrente.
type InvitationRepo struct {
	CreateFunc          func(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error)
	ListFunc            func(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Invitation, int, error)
	FindByTokenHashFunc func(ctx context.Context, tokenHash string) (entities.Invitation, error)
	AcceptFunc          func(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error)
	RevokeFunc          func(ctx context.Context, application, id uuid.UUID) error

	mu           sync.Mutex
	Created      []entities.Invitation
	Accepted     []entities.Invitation
	Revoked      []uuid.UUID
	Hashes       []string
	Windows      [][2]int
	Applications []uuid.UUID
}

func (m *InvitationRepo) Create(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, invitation.ApplicationID)
	m.mu.Unlock()
	out, err := invitation, error(nil)
	if m.CreateFunc != nil {
		out, err = m.CreateFunc(ctx, invitation)
	}
	if err == nil {
		m.mu.Lock()
		m.Created = append(m.Created, invitation)
		m.mu.Unlock()
	}
	return out, err
}

func (m *InvitationRepo) List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Invitation, int, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.Windows = append(m.Windows, [2]int{limit, offset})
	m.mu.Unlock()
	if m.ListFunc != nil {
		return m.ListFunc(ctx, application, limit, offset)
	}
	return []entities.Invitation{}, 0, nil
}

func (m *InvitationRepo) FindByTokenHash(ctx context.Context, tokenHash string) (entities.Invitation, error) {
	m.mu.Lock()
	m.Hashes = append(m.Hashes, tokenHash)
	m.mu.Unlock()
	if m.FindByTokenHashFunc != nil {
		return m.FindByTokenHashFunc(ctx, tokenHash)
	}
	return entities.Invitation{}, sql.ErrNoRows
}

func (m *InvitationRepo) Accept(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, invitation.ApplicationID)
	m.mu.Unlock()
	out, err := invitation, error(nil)
	if m.AcceptFunc != nil {
		out, err = m.AcceptFunc(ctx, invitation)
	}
	if err == nil {
		m.mu.Lock()
		m.Accepted = append(m.Accepted, invitation)
		m.mu.Unlock()
	}
	return out, err
}

func (m *InvitationRepo) Revoke(ctx context.Context, application, id uuid.UUID) error {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.mu.Unlock()
	if m.RevokeFunc != nil {
		if err := m.RevokeFunc(ctx, application, id); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.Revoked = append(m.Revoked, id)
	m.mu.Unlock()
	return nil
}

// Application devolve a aplicação da última chamada (o zero se não houve nenhuma).
func (m *InvitationRepo) Application() uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Applications) == 0 {
		return uuid.UUID{}
	}
	return m.Applications[len(m.Applications)-1]
}

// OutboxRepo é o fake da outbox: só o Create é usado.
type OutboxRepo struct {
	CreateFunc func(ctx context.Context, event entities.OutboxEvent) (entities.OutboxEvent, error)

	mu      sync.Mutex
	Created []entities.OutboxEvent
}

func (m *OutboxRepo) Create(ctx context.Context, event entities.OutboxEvent) (entities.OutboxEvent, error) {
	out, err := event, error(nil)
	if m.CreateFunc != nil {
		out, err = m.CreateFunc(ctx, event)
	}
	if err == nil {
		m.mu.Lock()
		m.Created = append(m.Created, event)
		m.mu.Unlock()
	}
	return out, err
}

var (
	_ contracts.IInvitationCreator     = (*InvitationRepo)(nil)
	_ contracts.IInvitationLister      = (*InvitationRepo)(nil)
	_ contracts.IInvitationTokenFinder = (*InvitationRepo)(nil)
	_ contracts.IInvitationAcceptor    = (*InvitationRepo)(nil)
	_ contracts.IInvitationRevoker     = (*InvitationRepo)(nil)
	_ contracts.IOutboxCreator         = (*OutboxRepo)(nil)
)

// Helpers

// NewInvitation devolve um convite pendente válido e aleatório.
func NewInvitation() entities.Invitation {
	invitation, _, _ := entities.NewInvitation(uuid.NewV7(), utils.GenRandomEmail(), entities.RoleDeveloper, uuid.NewV7())
	return invitation
}
