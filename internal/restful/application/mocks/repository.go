package mocks

import (
	"context"
	"sync"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

// Mocks

// ApplicationRepo é o fake do repositório de aplicações: implementa todos os
// contratos de aplicação. Cada método usa a *Func correspondente quando
// definida; sem ela, Create, Update e Upgrade devolvem a própria aplicação,
// Exist devolve false, as leituras por id devolvem o valor zero e as listas
// vêm vazias. Guarda os argumentos das chamadas. Seguro para uso concorrente.
type ApplicationRepo struct {
	CreateFunc     func(ctx context.Context, application entities.Application) (entities.Application, error)
	ExistFunc      func(ctx context.Context, application entities.Application) (bool, error)
	FindByIDFunc   func(ctx context.Context, id uuid.UUID) (entities.Application, error)
	ListFunc       func(ctx context.Context, limit, offset int) ([]entities.Application, int, error)
	ListByUserFunc func(ctx context.Context, user uuid.UUID, limit, offset int) ([]entities.Application, int, error)
	UpdateFunc     func(ctx context.Context, application entities.Application) (entities.Application, error)
	UpgradeFunc    func(ctx context.Context, application entities.Application) (entities.Application, error)
	DeleteFunc     func(ctx context.Context, id uuid.UUID) error

	mu       sync.Mutex
	Created  []entities.Application
	Updated  []entities.Application
	Upgraded []entities.Application
	Deleted  []uuid.UUID
	Found    []uuid.UUID
	Windows  [][2]int
	// Users guarda a conta de cada ListByUser, na ordem.
	Users []uuid.UUID
}

func (m *ApplicationRepo) Create(ctx context.Context, application entities.Application) (entities.Application, error) {
	return record(m, &m.Created, m.CreateFunc, ctx, application)
}

func (m *ApplicationRepo) Exist(ctx context.Context, application entities.Application) (bool, error) {
	if m.ExistFunc != nil {
		return m.ExistFunc(ctx, application)
	}
	return false, nil
}

func (m *ApplicationRepo) FindById(ctx context.Context, id uuid.UUID) (entities.Application, error) {
	m.mu.Lock()
	m.Found = append(m.Found, id)
	m.mu.Unlock()
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return entities.Application{}, nil
}

func (m *ApplicationRepo) List(ctx context.Context, limit, offset int) ([]entities.Application, int, error) {
	m.mu.Lock()
	m.Windows = append(m.Windows, [2]int{limit, offset})
	m.mu.Unlock()
	if m.ListFunc != nil {
		return m.ListFunc(ctx, limit, offset)
	}
	return []entities.Application{}, 0, nil
}

func (m *ApplicationRepo) ListByUser(ctx context.Context, user uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
	m.mu.Lock()
	m.Users = append(m.Users, user)
	m.Windows = append(m.Windows, [2]int{limit, offset})
	m.mu.Unlock()
	if m.ListByUserFunc != nil {
		return m.ListByUserFunc(ctx, user, limit, offset)
	}
	return []entities.Application{}, 0, nil
}

func (m *ApplicationRepo) Update(ctx context.Context, application entities.Application) (entities.Application, error) {
	return record(m, &m.Updated, m.UpdateFunc, ctx, application)
}

func (m *ApplicationRepo) Upgrade(ctx context.Context, application entities.Application) (entities.Application, error) {
	return record(m, &m.Upgraded, m.UpgradeFunc, ctx, application)
}

func (m *ApplicationRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		if err := m.DeleteFunc(ctx, id); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.Deleted = append(m.Deleted, id)
	m.mu.Unlock()
	return nil
}

// record chama fn (ou devolve o próprio input) e guarda o input quando deu certo.
func record(m *ApplicationRepo, into *[]entities.Application, fn func(context.Context, entities.Application) (entities.Application, error), ctx context.Context, input entities.Application) (entities.Application, error) {
	out, err := input, error(nil)
	if fn != nil {
		out, err = fn(ctx, input)
	}
	if err == nil {
		m.mu.Lock()
		*into = append(*into, input)
		m.mu.Unlock()
	}
	return out, err
}

var (
	_ contracts.IApplicationCreator    = (*ApplicationRepo)(nil)
	_ contracts.IApplicationChecker    = (*ApplicationRepo)(nil)
	_ contracts.IApplicationFinder     = (*ApplicationRepo)(nil)
	_ contracts.IApplicationLister     = (*ApplicationRepo)(nil)
	_ contracts.IApplicationUserLister = (*ApplicationRepo)(nil)
	_ contracts.IApplicationUpdater    = (*ApplicationRepo)(nil)
	_ contracts.IApplicationUpgrader   = (*ApplicationRepo)(nil)
	_ contracts.IApplicationDeleter    = (*ApplicationRepo)(nil)
)

// Helpers

// NewRepo devolve o fake com o padrão de uma aplicação que existe: FindById
// devolve uma aplicação com o id pedido.
func NewRepo() *ApplicationRepo {
	return &ApplicationRepo{FindByIDFunc: func(_ context.Context, id uuid.UUID) (entities.Application, error) {
		return entities.Application{ID: id}, nil
	}}
}

// NewRequester devolve um Requester de uma pessoa autenticada.
func NewRequester() entities.ApiHeader {
	return entities.ApiHeader{
		ClientIP:  utils.GenRandomIP(utils.GenRandomLetterAtoE()),
		UserAgent: utils.GenRandomUserAgent(),
		Origin:    utils.GenRandomOrigin(),
		RequestID: uuid.New().String(),
		Country:   utils.GenRandomCountry(),
		CdnRayID:  utils.GenRandomCdnRayID(),
		Actor:     entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7()},
	}
}

func NewApp() entities.Application {
	return entities.Application{
		ID:   uuid.NewV7(),
		Name: utils.GenRandomApplicationName(),
	}
}
