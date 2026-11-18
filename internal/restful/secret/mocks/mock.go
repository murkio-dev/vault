package mocks

import (
	"context"
	"database/sql"
	"sync"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	secretcontracts "github.com/murkio-dev/vault/internal/restful/secret/contracts"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// Mocks

// SecretRepo é o fake do repositório de segredos: implementa todos os contratos
// de segredo. Cada método usa a *Func correspondente quando definida; sem ela,
// Create, AddVersion e Rename devolvem o segredo do write, as leituras por id
// devolvem o valor zero, as listas vêm vazias e as contagens, zero. Guarda a
// aplicação e o dono de cada chamada para o teste conferir o isolamento.
// Seguro para uso concorrente.
type SecretRepo struct {
	CreateFunc          func(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
	FindByIDFunc        func(ctx context.Context, application, id uuid.UUID) (entities.Secret, error)
	FindByOwnerFunc     func(ctx context.Context, application, owner, id uuid.UUID) (entities.Secret, error)
	ListFunc            func(ctx context.Context, application, owner uuid.UUID, limit, offset int) ([]entities.Secret, int, error)
	CountFunc           func(ctx context.Context, application, owner uuid.UUID) (int, error)
	CountReferencesFunc func(ctx context.Context, application, secret uuid.UUID) (int, error)
	AddVersionFunc      func(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
	RenameFunc          func(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
	DeleteFunc          func(ctx context.Context, application, id uuid.UUID) error

	mu           sync.Mutex
	Created      []entities.SecretWrite
	Added        []entities.SecretWrite
	Renamed      []entities.SecretWrite
	Deleted      []uuid.UUID
	Found        []uuid.UUID
	Windows      [][2]int
	Applications []uuid.UUID
	Owners       []uuid.UUID
}

func (m *SecretRepo) touch(application uuid.UUID, owner ...uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Applications = append(m.Applications, application)
	m.Owners = append(m.Owners, owner...)
}

func (m *SecretRepo) Create(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
	m.touch(write.ApplicationID)
	secret, err := write.Secret, error(nil)
	if m.CreateFunc != nil {
		secret, err = m.CreateFunc(ctx, write)
	}
	if err == nil {
		m.mu.Lock()
		m.Created = append(m.Created, write)
		m.mu.Unlock()
	}
	return secret, err
}

func (m *SecretRepo) FindById(ctx context.Context, application, id uuid.UUID) (entities.Secret, error) {
	m.touch(application)
	m.mu.Lock()
	m.Found = append(m.Found, id)
	m.mu.Unlock()
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, application, id)
	}
	return entities.Secret{}, nil
}

func (m *SecretRepo) FindByOwner(ctx context.Context, application, owner, id uuid.UUID) (entities.Secret, error) {
	m.touch(application, owner)
	m.mu.Lock()
	m.Found = append(m.Found, id)
	m.mu.Unlock()
	if m.FindByOwnerFunc != nil {
		return m.FindByOwnerFunc(ctx, application, owner, id)
	}
	return entities.Secret{}, nil
}

func (m *SecretRepo) List(ctx context.Context, application, owner uuid.UUID, limit, offset int) ([]entities.Secret, int, error) {
	m.touch(application, owner)
	m.mu.Lock()
	m.Windows = append(m.Windows, [2]int{limit, offset})
	m.mu.Unlock()
	if m.ListFunc != nil {
		return m.ListFunc(ctx, application, owner, limit, offset)
	}
	return []entities.Secret{}, 0, nil
}

func (m *SecretRepo) Count(ctx context.Context, application, owner uuid.UUID) (int, error) {
	m.touch(application, owner)
	if m.CountFunc != nil {
		return m.CountFunc(ctx, application, owner)
	}
	return 0, nil
}

func (m *SecretRepo) CountReferences(ctx context.Context, application, secret uuid.UUID) (int, error) {
	m.touch(application)
	if m.CountReferencesFunc != nil {
		return m.CountReferencesFunc(ctx, application, secret)
	}
	return 0, nil
}

func (m *SecretRepo) AddVersion(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
	m.touch(write.ApplicationID)
	secret, err := write.Secret, error(nil)
	if m.AddVersionFunc != nil {
		secret, err = m.AddVersionFunc(ctx, write)
	}
	if err == nil {
		m.mu.Lock()
		m.Added = append(m.Added, write)
		m.mu.Unlock()
	}
	return secret, err
}

func (m *SecretRepo) Rename(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
	m.touch(write.ApplicationID)
	secret, err := write.Secret, error(nil)
	if m.RenameFunc != nil {
		secret, err = m.RenameFunc(ctx, write)
	}
	if err == nil {
		m.mu.Lock()
		m.Renamed = append(m.Renamed, write)
		m.mu.Unlock()
	}
	return secret, err
}

func (m *SecretRepo) Delete(ctx context.Context, application, id uuid.UUID) error {
	m.touch(application)
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

func (m *SecretRepo) Application() uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Applications) == 0 {
		return uuid.UUID{}
	}
	return m.Applications[len(m.Applications)-1]
}

func (m *SecretRepo) Owner() uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Owners) == 0 {
		return uuid.UUID{}
	}
	return m.Owners[len(m.Owners)-1]
}

// VersionRepo é o fake do repositório de versões: implementa todos os contratos
// de versão. Sem *Func, as leituras devolvem sql.ErrNoRows e a lista vem vazia.
type VersionRepo struct {
	ListFunc         func(ctx context.Context, application, secret uuid.UUID, limit, offset int) ([]entities.SecretVersion, int, error)
	FindCurrentFunc  func(ctx context.Context, application, secret uuid.UUID) (entities.SecretVersion, error)
	FindByNumberFunc func(ctx context.Context, application, secret uuid.UUID, number int) (entities.SecretVersion, error)
	ResealFunc       func(ctx context.Context, application, secret uuid.UUID, version entities.SecretVersion) (entities.SecretVersion, error)

	mu           sync.Mutex
	Resealed     []entities.SecretVersion
	Applications []uuid.UUID
	Secrets      []uuid.UUID
}

func (m *VersionRepo) touch(application, secret uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Applications = append(m.Applications, application)
	m.Secrets = append(m.Secrets, secret)
}

func (m *VersionRepo) List(ctx context.Context, application, secret uuid.UUID, limit, offset int) ([]entities.SecretVersion, int, error) {
	m.touch(application, secret)
	if m.ListFunc != nil {
		return m.ListFunc(ctx, application, secret, limit, offset)
	}
	return []entities.SecretVersion{}, 0, nil
}

func (m *VersionRepo) FindCurrent(ctx context.Context, application, secret uuid.UUID) (entities.SecretVersion, error) {
	m.touch(application, secret)
	if m.FindCurrentFunc != nil {
		return m.FindCurrentFunc(ctx, application, secret)
	}
	return entities.SecretVersion{}, sql.ErrNoRows
}

func (m *VersionRepo) FindByNumber(ctx context.Context, application, secret uuid.UUID, number int) (entities.SecretVersion, error) {
	m.touch(application, secret)
	if m.FindByNumberFunc != nil {
		return m.FindByNumberFunc(ctx, application, secret, number)
	}
	return entities.SecretVersion{}, sql.ErrNoRows
}

func (m *VersionRepo) Reseal(ctx context.Context, application, secret uuid.UUID, version entities.SecretVersion) (entities.SecretVersion, error) {
	m.touch(application, secret)
	out, err := version, error(nil)
	if m.ResealFunc != nil {
		out, err = m.ResealFunc(ctx, application, secret, version)
	}
	if err == nil {
		m.mu.Lock()
		m.Resealed = append(m.Resealed, version)
		m.mu.Unlock()
	}
	return out, err
}

var (
	_ secretcontracts.ISecretRepository  = (*SecretRepo)(nil)
	_ secretcontracts.IVersionRepository = (*VersionRepo)(nil)
)

type Values struct {
	SealErr     error
	ValueErr    error
	ValueResult string
	Version     int

	Sealed []string
}

func (m *Values) Seal(_ uuid.UUID, value string) ([]byte, error) {
	if m.SealErr != nil {
		return nil, m.SealErr
	}
	m.Sealed = append(m.Sealed, value)
	return []byte("sealed:" + value), nil
}

func (m *Values) Value(_ context.Context, secret entities.Secret, version int) (string, int, error) {
	if m.ValueErr != nil {
		return "", 0, m.ValueErr
	}
	number := m.Version
	if number == 0 {
		number = secret.Version
	}
	if version > 0 {
		number = version
	}
	return m.ValueResult, number, nil
}

var _ contracts.ISecretValues = (*Values)(nil)

// Rig reúne os fakes que todo use case de segredo recebe, para cada teste não
// montar os cinco à mão. Existe porque os use cases de segredo compartilham as
// mesmas dependências (repositório, versões, cifra, guard e transação auditada).
// Os campos são os próprios fakes: o teste troca uma *Func ou um erro neles e
// depois confere o que foi gravado. Owner é o membro para o qual o guard
// resolve o dono (o "me" do pedido).
// Exemplo: r := NewRig(); r.Guard.Deny(entities.ForbiddenError()) faz todo use
// case montado com r.Guard responder 403 sem tocar r.Secrets.
// O Rig é passado por valor: a cópia aponta para os mesmos fakes, então cada
// cenário cria o seu com NewRig e nunca divide um Rig entre t.Run.
type Rig struct {
	Secrets  *SecretRepo
	Versions *VersionRepo
	Values   *Values
	Guard    *member_mocks.Guard
	Tx       *audit_mocks.Tx
	Owner    uuid.UUID
}

// Helpers

func NewSecret() entities.Secret {
	secret := entities.NewSecret(uuid.NewV7(), uuid.NewV7(), pkgmocks.GenRandomSecretName(), entities.SecretString, entities.SourceStatic, nil, uuid.NewV7())
	secret.Version = 1
	return secret
}

func NewDraft() entities.SecretDraft {
	return entities.SecretDraft{Name: pkgmocks.GenRandomSecretName(), Type: entities.SecretString, Source: entities.SourceStatic, Value: pkgmocks.GenRandomSecretValue()}
}

func NewUniqueViolation() error {
	db, err := sqlite.New(":memory:")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE unique_test (name TEXT UNIQUE)`); err != nil {
		panic(err)
	}
	if _, err := db.Exec(`INSERT INTO unique_test VALUES ('a')`); err != nil {
		panic(err)
	}
	_, err = db.Exec(`INSERT INTO unique_test VALUES ('a')`)
	return err
}

func NewWrite() entities.SecretWrite {
	return entities.SecretWrite{Secret: NewSecret(), VersionID: uuid.NewV7(), Ciphertext: []byte("sealed:v")}
}

// NewRepo devolve o fake com o padrão de um repositório que funciona:
// FindById e FindByOwner devolvem um segredo com o id pedido, na aplicação (e no
// dono) pedidos; Create devolve o segredo gravado (versão 1 se houver valor);
// AddVersion sobe a versão; Rename devolve o segredo renomeado.
func NewRepo() *SecretRepo {
	return &SecretRepo{
		FindByIDFunc: func(_ context.Context, application, id uuid.UUID) (entities.Secret, error) {
			secret := NewSecret()
			secret.ID, secret.ApplicationID = id, application
			return secret, nil
		},
		FindByOwnerFunc: func(_ context.Context, application, owner, id uuid.UUID) (entities.Secret, error) {
			secret := NewSecret()
			secret.ID, secret.ApplicationID, secret.MemberID = id, application, owner
			return secret, nil
		},
		CreateFunc: func(_ context.Context, write entities.SecretWrite) (entities.Secret, error) {
			secret := write.Secret
			if write.Ciphertext != nil {
				secret.Version = 1
			}
			return secret, nil
		},
		AddVersionFunc: func(_ context.Context, write entities.SecretWrite) (entities.Secret, error) {
			secret := write.Secret
			secret.Version++
			return secret, nil
		},
		RenameFunc: func(_ context.Context, write entities.SecretWrite) (entities.Secret, error) {
			return write.Secret, nil
		},
	}
}

func NewRig() Rig {
	owner := uuid.NewV7()
	guard := &member_mocks.Guard{}
	guard.PerformFunc = func(_ context.Context, _ entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{Owner: owner}
	}

	return Rig{
		Secrets:  NewRepo(),
		Versions: &VersionRepo{},
		Values:   &Values{ValueResult: "valor-revelado"},
		Guard:    guard,
		Tx:       &audit_mocks.Tx{},
		Owner:    owner,
	}
}
