package mocks

import (
	"context"
	"database/sql"
	"sync"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	membercontracts "github.com/murkio-dev/vault/internal/restful/member/contracts"
)

// Mocks

type Guard struct {
	PerformFunc func(ctx context.Context, input entities.AuthorizeInput) entities.AuthorizeOutput
	Member      entities.Member

	Calls  int
	Inputs []entities.AuthorizeInput
}

func (m *Guard) Perform(ctx context.Context, input entities.AuthorizeInput) entities.AuthorizeOutput {
	m.Calls++
	m.Inputs = append(m.Inputs, input)

	if m.PerformFunc != nil {
		return m.PerformFunc(ctx, input)
	}

	member := m.Member
	if member.ID == (uuid.UUID{}) {
		member = NewMember(entities.RoleAdmin)
	}

	owner := input.Owner
	if owner == entities.SelfOwner {
		owner = member.ID
	}

	return entities.AuthorizeOutput{Member: member, Owner: owner}
}

func (m *Guard) Deny(denied entities.Error) *Guard {
	m.PerformFunc = func(context.Context, entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{Error: &denied}
	}
	return m
}

func (m *Guard) Actions() []entities.Action {
	actions := make([]entities.Action, len(m.Inputs))
	for i, input := range m.Inputs {
		actions[i] = input.Action
	}
	return actions
}

var _ contracts.IAccessGuard = (*Guard)(nil)

// MemberRepo é o fake do repositório de membros: implementa todos os contratos
// de membro. Cada método usa a *Func correspondente quando definida; sem ela,
// devolve um valor padrão sem erro (Create devolve o próprio membro, as
// leituras por id devolvem o valor zero, FindByUser devolve sql.ErrNoRows como
// quem não é membro, List devolve uma lista vazia). Guarda os argumentos de cada
// chamada para o teste conferir que o use case pediu a aplicação certa. É
// seguro para uso concorrente.
type MemberRepo struct {
	CreateFunc      func(ctx context.Context, member entities.Member) (entities.Member, error)
	FindByIDFunc    func(ctx context.Context, application, id uuid.UUID) (entities.Member, error)
	FindByUserFunc  func(ctx context.Context, application, user uuid.UUID) (entities.Member, error)
	ListFunc        func(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Member, int, error)
	CountAdminsFunc func(ctx context.Context, application uuid.UUID) (int, error)
	UpgradeFunc     func(ctx context.Context, member entities.Member) (entities.Member, error)
	DeleteFunc      func(ctx context.Context, application, id uuid.UUID) error

	mu           sync.Mutex
	Created      []entities.Member
	Upgraded     []entities.Member
	Deleted      []uuid.UUID
	Found        []uuid.UUID
	Windows      [][2]int
	Applications []uuid.UUID
}

func (m *MemberRepo) Create(ctx context.Context, member entities.Member) (entities.Member, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, member.ApplicationID)
	m.mu.Unlock()
	if m.CreateFunc != nil {
		out, err := m.CreateFunc(ctx, member)
		if err == nil {
			m.mu.Lock()
			m.Created = append(m.Created, member)
			m.mu.Unlock()
		}
		return out, err
	}
	m.mu.Lock()
	m.Created = append(m.Created, member)
	m.mu.Unlock()
	return member, nil
}

func (m *MemberRepo) FindById(ctx context.Context, application, id uuid.UUID) (entities.Member, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.Found = append(m.Found, id)
	m.mu.Unlock()
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, application, id)
	}
	return entities.Member{}, nil
}

func (m *MemberRepo) FindByUser(ctx context.Context, application, user uuid.UUID) (entities.Member, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.mu.Unlock()
	if m.FindByUserFunc != nil {
		return m.FindByUserFunc(ctx, application, user)
	}
	return entities.Member{}, sql.ErrNoRows
}

func (m *MemberRepo) List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Member, int, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.Windows = append(m.Windows, [2]int{limit, offset})
	m.mu.Unlock()
	if m.ListFunc != nil {
		return m.ListFunc(ctx, application, limit, offset)
	}
	return []entities.Member{}, 0, nil
}

func (m *MemberRepo) CountActiveAdmins(ctx context.Context, application uuid.UUID) (int, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.mu.Unlock()
	if m.CountAdminsFunc != nil {
		return m.CountAdminsFunc(ctx, application)
	}
	return 0, nil
}

func (m *MemberRepo) Upgrade(ctx context.Context, member entities.Member) (entities.Member, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, member.ApplicationID)
	m.mu.Unlock()
	if m.UpgradeFunc != nil {
		out, err := m.UpgradeFunc(ctx, member)
		if err == nil {
			m.mu.Lock()
			m.Upgraded = append(m.Upgraded, member)
			m.mu.Unlock()
		}
		return out, err
	}
	m.mu.Lock()
	m.Upgraded = append(m.Upgraded, member)
	m.mu.Unlock()
	return member, nil
}

func (m *MemberRepo) Delete(ctx context.Context, application, id uuid.UUID) error {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.mu.Unlock()
	if m.DeleteFunc != nil {
		if err := m.DeleteFunc(ctx, application, id); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.Deleted = append(m.Deleted, id)
	m.mu.Unlock()
	return nil
}

var _ membercontracts.IMemberRepository = (*MemberRepo)(nil)

// Helpers

func NewMember(role entities.Role) entities.Member {
	return entities.NewMember(uuid.NewV7(), uuid.NewV7(), role, nil)
}

func NewUserRequester() entities.ApiHeader {
	return entities.ApiHeader{
		ClientIP:  "203.0.113.7",
		UserAgent: "go-test",
		RequestID: uuid.New().String(),
		Actor:     entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7()},
	}
}

func NewTokenRequester(member entities.Member) entities.ApiHeader {
	return entities.ApiHeader{
		ClientIP:  "203.0.113.7",
		UserAgent: "go-sdk",
		RequestID: uuid.New().String(),
		Actor: entities.Actor{Kind: entities.ActorToken, UserID: member.UserID, TokenID: uuid.NewV7(),
			MemberID: member.ID, ApplicationID: member.ApplicationID},
	}
}

func (m *MemberRepo) Application() uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Applications) == 0 {
		return uuid.UUID{}
	}
	return m.Applications[len(m.Applications)-1]
}

// NewRepo devolve o fake com o padrão de um membro que existe: FindById devolve
// um developer com o id pedido, na aplicação pedida.
func NewRepo() *MemberRepo {
	return &MemberRepo{FindByIDFunc: func(_ context.Context, application, id uuid.UUID) (entities.Member, error) {
		member := NewMember(entities.RoleDeveloper)
		member.ID, member.ApplicationID = id, application
		return member, nil
	}}
}
