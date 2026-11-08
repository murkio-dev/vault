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

// TokenRepo é o fake do repositório de tokens: implementa todos os contratos de
// token. Cada método usa a *Func correspondente quando definida; sem ela, Create
// e TouchLastUsed devolvem o próprio token, FindActiveByHash devolve
// sql.ErrNoRows, as outras leituras devolvem o valor zero e Revoke dá certo.
// Guarda a aplicação, o membro e o hash de cada chamada. Seguro para uso
// concorrente.
type TokenRepo struct {
	CreateFunc           func(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error)
	FindByIDFunc         func(ctx context.Context, application, id uuid.UUID) (entities.AccessToken, error)
	ListFunc             func(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.AccessToken, int, error)
	FindActiveByHashFunc func(ctx context.Context, tokenHash string) (entities.AccessToken, error)
	TouchLastUsedFunc    func(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error)
	RevokeFunc           func(ctx context.Context, application, id uuid.UUID) error

	mu           sync.Mutex
	Created      []entities.AccessToken
	Touched      []entities.AccessToken
	Revoked      []uuid.UUID
	Found        []uuid.UUID
	Windows      [][2]int
	Applications []uuid.UUID
	Members      []uuid.UUID
	Hashes       []string
}

func (m *TokenRepo) Create(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, token.ApplicationID)
	m.mu.Unlock()
	out, err := token, error(nil)
	if m.CreateFunc != nil {
		out, err = m.CreateFunc(ctx, token)
	}
	if err == nil {
		m.mu.Lock()
		m.Created = append(m.Created, token)
		m.mu.Unlock()
	}
	return out, err
}

func (m *TokenRepo) FindById(ctx context.Context, application, id uuid.UUID) (entities.AccessToken, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.Found = append(m.Found, id)
	m.mu.Unlock()
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, application, id)
	}
	return entities.AccessToken{}, nil
}

func (m *TokenRepo) List(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.AccessToken, int, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, application)
	m.Members = append(m.Members, member)
	m.Windows = append(m.Windows, [2]int{limit, offset})
	m.mu.Unlock()
	if m.ListFunc != nil {
		return m.ListFunc(ctx, application, member, limit, offset)
	}
	return []entities.AccessToken{}, 0, nil
}

func (m *TokenRepo) FindActiveByHash(ctx context.Context, tokenHash string) (entities.AccessToken, error) {
	m.mu.Lock()
	m.Hashes = append(m.Hashes, tokenHash)
	m.mu.Unlock()
	if m.FindActiveByHashFunc != nil {
		return m.FindActiveByHashFunc(ctx, tokenHash)
	}
	return entities.AccessToken{}, sql.ErrNoRows
}

func (m *TokenRepo) TouchLastUsed(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error) {
	m.mu.Lock()
	m.Applications = append(m.Applications, token.ApplicationID)
	m.mu.Unlock()
	out, err := token, error(nil)
	if m.TouchLastUsedFunc != nil {
		out, err = m.TouchLastUsedFunc(ctx, token)
	}
	if err == nil {
		m.mu.Lock()
		m.Touched = append(m.Touched, token)
		m.mu.Unlock()
	}
	return out, err
}

func (m *TokenRepo) Revoke(ctx context.Context, application, id uuid.UUID) error {
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
func (m *TokenRepo) Application() uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Applications) == 0 {
		return uuid.UUID{}
	}
	return m.Applications[len(m.Applications)-1]
}

var (
	_ contracts.ITokenCreator      = (*TokenRepo)(nil)
	_ contracts.ITokenFinder       = (*TokenRepo)(nil)
	_ contracts.ITokenLister       = (*TokenRepo)(nil)
	_ contracts.ITokenHashFinder   = (*TokenRepo)(nil)
	_ contracts.ITokenUsageToucher = (*TokenRepo)(nil)
	_ contracts.ITokenRevoker      = (*TokenRepo)(nil)
)

// Helpers

// NewToken devolve um token do SDK ativo e aleatório.
func NewToken() entities.AccessToken {
	member := entities.NewMember(uuid.NewV7(), uuid.NewV7(), entities.RoleDeveloper, nil)
	token, _, _ := entities.NewAccessToken(member, utils.GenRandomTokenName(), nil)
	return token
}
