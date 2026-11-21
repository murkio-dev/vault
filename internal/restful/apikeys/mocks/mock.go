package mocks

import (
	"context"
	"database/sql"
	"sync"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
)

// Mocks

// ApiKeyRepo é o fake do repositório de chaves: implementa todos os contratos
// de chave. Cada método usa a *Func correspondente quando definida; sem ela,
// Create, Rename e TouchLastUsed devolvem a própria chave, FindActiveByHash
// devolve sql.ErrNoRows, as outras leituras devolvem o valor zero e Expire e
// Delete dão certo. Guarda a aplicação, o membro e o hash de cada chamada.
// Seguro para uso concorrente.
type ApiKeyRepo struct {
	CreateFunc           func(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error)
	FindByIDFunc         func(ctx context.Context, application, id uuid.UUID) (entities.ApiKey, error)
	ListFunc             func(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.ApiKey, int, error)
	FindActiveByHashFunc func(ctx context.Context, hash string) (entities.ApiKey, error)
	RenameFunc           func(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error)
	ExpireFunc           func(ctx context.Context, application, id uuid.UUID, at time.Time) error
	TouchLastUsedFunc    func(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error)
	DeleteFunc           func(ctx context.Context, application, id uuid.UUID) error

	mu           sync.Mutex
	Created      []entities.ApiKey
	Renamed      []entities.ApiKey
	Expired      map[uuid.UUID]time.Time
	Touched      []entities.ApiKey
	Deleted      []uuid.UUID
	Found        []uuid.UUID
	Windows      [][2]int
	Applications []uuid.UUID
	Members      []uuid.UUID
	Hashes       []string
}

func (m *ApiKeyRepo) Create(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, key.ApplicationID)
	m.mu.Unlock()
	out, err := key, error(nil)
	if m.CreateFunc != nil {
		out, err = m.CreateFunc(ctx, key)
	}
	if err == nil {
		m.mu.Lock()
		m.Created = append(m.Created, key)
		m.mu.Unlock()
	}
	return out, err
}

func (m *ApiKeyRepo) FindById(ctx context.Context, application, id uuid.UUID) (entities.ApiKey, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.Found = append(m.Found, id)
	m.mu.Unlock()
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, application, id)
	}
	return entities.ApiKey{}, nil
}

func (m *ApiKeyRepo) List(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.ApiKey, int, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.Members = append(m.Members, member)
	m.Windows = append(m.Windows, [2]int{limit, offset})
	m.mu.Unlock()
	if m.ListFunc != nil {
		return m.ListFunc(ctx, application, member, limit, offset)
	}
	return []entities.ApiKey{}, 0, nil
}

func (m *ApiKeyRepo) FindActiveByHash(ctx context.Context, hash string) (entities.ApiKey, error) {
	m.mu.Lock()
	m.Hashes = append(m.Hashes, hash)
	m.mu.Unlock()
	if m.FindActiveByHashFunc != nil {
		return m.FindActiveByHashFunc(ctx, hash)
	}
	return entities.ApiKey{}, sql.ErrNoRows
}

func (m *ApiKeyRepo) Rename(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, key.ApplicationID)
	m.mu.Unlock()
	out, err := key, error(nil)
	if m.RenameFunc != nil {
		out, err = m.RenameFunc(ctx, key)
	}
	if err == nil {
		m.mu.Lock()
		m.Renamed = append(m.Renamed, key)
		m.mu.Unlock()
	}
	return out, err
}

func (m *ApiKeyRepo) Expire(ctx context.Context, application, id uuid.UUID, at time.Time) error {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.mu.Unlock()
	if m.ExpireFunc != nil {
		if err := m.ExpireFunc(ctx, application, id, at); err != nil {
			return err
		}
	}
	m.mu.Lock()
	if m.Expired == nil {
		m.Expired = map[uuid.UUID]time.Time{}
	}
	m.Expired[id] = at
	m.mu.Unlock()
	return nil
}

func (m *ApiKeyRepo) TouchLastUsed(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, key.ApplicationID)
	m.mu.Unlock()
	out, err := key, error(nil)
	if m.TouchLastUsedFunc != nil {
		out, err = m.TouchLastUsedFunc(ctx, key)
	}
	if err == nil {
		m.mu.Lock()
		m.Touched = append(m.Touched, key)
		m.mu.Unlock()
	}
	return out, err
}

func (m *ApiKeyRepo) Delete(ctx context.Context, application, id uuid.UUID) error {
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

func (m *ApiKeyRepo) Application() uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Applications) == 0 {
		return uuid.UUID{}
	}
	return m.Applications[len(m.Applications)-1]
}

var _ apikeyscontracts.IApiKeyRepository = (*ApiKeyRepo)(nil)

// Helpers

func NewApiKey() entities.ApiKey {
	member := entities.NewMember(uuid.NewV7(), uuid.NewV7(), entities.RoleDeveloper, nil)
	key, _, _ := entities.NewApiKey(member, pkgmocks.GenRandomTokenName(), entities.KeySecret, nil)
	return key
}
