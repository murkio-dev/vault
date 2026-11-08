package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/repositories"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestSecretRepository(t *testing.T) {
	t.Run("Quando falta um id de isolamento.", func(t *testing.T) { whenIsolationIDIsMissing(t) })
	t.Run("Quando outro tenant pede os segredos.", func(t *testing.T) { whenAnotherTenantAsks(t) })
	t.Run("Quando cria com e sem valor.", func(t *testing.T) { whenCreatingWithAndWithoutValue(t) })
	t.Run("Quando separa a tabela de cada membro.", func(t *testing.T) { whenSeparatingMemberTables(t) })
	t.Run("Quando conta os segredos da tabela.", func(t *testing.T) { whenCountingTheTable(t) })
	t.Run("Quando conta quem aponta para o segredo.", func(t *testing.T) { whenCountingReferences(t) })
	t.Run("Quando grava uma nova versão.", func(t *testing.T) { whenAddingAVersion(t) })
	t.Run("Quando duas gravações disputam a versão.", func(t *testing.T) { whenTwoWritesRaceForTheVersion(t) })
	t.Run("Quando renomeia e apaga.", func(t *testing.T) { whenRenamingAndDeleting(t) })
	t.Run("Quando lê as versões.", func(t *testing.T) { whenListingVersions(t) })
	t.Run("Quando recifra uma versão.", func(t *testing.T) { whenResealing(t) })
}

// Functions

type env struct {
	db       *sql.DB
	secrets  repositories.SecretRepository
	versions repositories.VersionRepository
}

func newEnv(t *testing.T) *env {
	db := sqlitetest.New(t)
	return &env{db: db, secrets: repositories.NewRepository(db), versions: repositories.NewVersionRepository(db)}
}

func (e *env) add(t *testing.T, tenant sqlitetest.Tenant, name, ciphertext string) entities.Secret {
	t.Helper()
	secret := entities.NewSecret(tenant.Application, tenant.Member, name, entities.SecretString, entities.SourceStatic, nil, tenant.User)
	created, err := e.secrets.Create(context.Background(), entities.SecretWrite{Secret: secret, VersionID: uuid.NewV7(), Ciphertext: []byte(ciphertext)})
	require.NoError(t, err)
	return created
}

func whenIsolationIDIsMissing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	zero := uuid.UUID{}
	write := entities.SecretWrite{Secret: entities.NewSecret(uuid.NewV7(), uuid.NewV7(), "A", entities.SecretString, entities.SourceStatic, nil, uuid.NewV7())}
	orphan := write
	orphan.ApplicationID = zero
	id := write.ID

	_, createErr := e.secrets.Create(ctx, orphan)
	_, findErr := e.secrets.FindById(ctx, zero, id)
	_, ownerErr := e.secrets.FindByOwner(ctx, write.ApplicationID, zero, id)
	_, _, listErr := e.secrets.List(ctx, write.ApplicationID, zero, 10, 0)
	_, countErr := e.secrets.Count(ctx, write.ApplicationID, zero)
	_, refsErr := e.secrets.CountReferences(ctx, zero, id)
	_, addErr := e.secrets.AddVersion(ctx, orphan)
	_, renameErr := e.secrets.Rename(ctx, orphan)
	deleteErr := e.secrets.Delete(ctx, zero, id)
	_, _, versionsErr := e.versions.List(ctx, write.ApplicationID, zero, 10, 0)
	_, currentErr := e.versions.FindCurrent(ctx, zero, id)
	_, numberErr := e.versions.FindByNumber(ctx, write.ApplicationID, id, 0)
	_, resealErr := e.versions.Reseal(ctx, write.ApplicationID, zero, entities.SecretVersion{ID: uuid.NewV7()})

	for name, err := range map[string]error{
		"Create": createErr, "FindById": findErr, "FindByOwner sem dono": ownerErr, "List sem dono": listErr, "Count sem dono": countErr,
		"CountReferences": refsErr, "AddVersion": addErr, "Rename": renameErr, "Delete": deleteErr,
		"Versions.List sem segredo": versionsErr, "Versions.FindCurrent": currentErr, "Versions.FindByNumber sem número": numberErr, "Versions.Reseal sem segredo": resealErr,
	} {
		assert.ErrorIs(t, err, entities.ErrMissingParam, name)
	}
}

func whenAnotherTenantAsks(t *testing.T) {
	e := newEnv(t)
	a, b := sqlitetest.Seed(t, e.db, "app-a"), sqlitetest.Seed(t, e.db, "app-b")
	secret := e.add(t, a, "DATABASE_URL", "cifrado-de-a")
	ctx := context.Background()

	_, findErr := e.secrets.FindById(ctx, b.Application, secret.ID)
	_, ownerErr := e.secrets.FindByOwner(ctx, b.Application, a.Member, secret.ID)
	list, total, listErr := e.secrets.List(ctx, b.Application, a.Member, 10, 0)
	deleteErr := e.secrets.Delete(ctx, b.Application, secret.ID)
	_, renameErr := e.secrets.Rename(ctx, entities.SecretWrite{Secret: entities.Secret{ID: secret.ID, ApplicationID: b.Application, Name: "ROUBADO"}})
	versions, _, versionsErr := e.versions.List(ctx, b.Application, secret.ID, 10, 0)
	_, currentErr := e.versions.FindCurrent(ctx, b.Application, secret.ID)
	_, resealErr := e.versions.Reseal(ctx, b.Application, secret.ID, entities.SecretVersion{ID: uuid.NewV7(), Ciphertext: []byte("x")})

	assert.ErrorIs(t, findErr, sql.ErrNoRows, "o id de um segredo de outra aplicação não acha nada")
	assert.ErrorIs(t, ownerErr, sql.ErrNoRows)
	require.NoError(t, listErr)
	assert.Zero(t, total)
	assert.Empty(t, list)
	assert.ErrorIs(t, deleteErr, sql.ErrNoRows)
	assert.ErrorIs(t, renameErr, sql.ErrNoRows)
	require.NoError(t, versionsErr)
	assert.Empty(t, versions, "nem as versões (o ciphertext) por outra aplicação")
	assert.ErrorIs(t, currentErr, sql.ErrNoRows)
	assert.ErrorIs(t, resealErr, sql.ErrNoRows)
	still, err := e.secrets.FindById(ctx, a.Application, secret.ID)
	require.NoError(t, err)
	assert.Equal(t, "DATABASE_URL", still.Name, "e o segredo de A segue intacto")
}

func whenCreatingWithAndWithoutValue(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	withValue := e.add(t, a, "DATABASE_URL", "cifrado")
	reference := entities.NewSecret(a.Application, a.Member, "ALIAS", entities.SecretString, entities.SourceReference, &withValue.ID, a.User)

	alias, err := e.secrets.Create(context.Background(), entities.SecretWrite{Secret: reference})
	_, duplicate := e.secrets.Create(context.Background(), entities.SecretWrite{Secret: entities.NewSecret(a.Application, a.Member, "DATABASE_URL", entities.SecretString, entities.SourceStatic, nil, a.User), VersionID: uuid.NewV7(), Ciphertext: []byte("x")})

	require.NoError(t, err)
	assert.Equal(t, 1, withValue.Version, "com valor nasce na versão 1")
	assert.Zero(t, alias.Version, "uma referência não tem versão")
	if assert.NotNil(t, alias.ReferenceSecretID) {
		assert.Equal(t, withValue.ID, *alias.ReferenceSecretID)
	}
	var stored int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM secret_versions`).Scan(&stored))
	assert.Equal(t, 1, stored, "só o segredo com valor ganhou versão")
	assert.Error(t, duplicate, "nome repetido na tabela do membro")
}

func whenSeparatingMemberTables(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	e.add(t, a, "MINE", "c")
	other := entities.NewMember(a.Application, uuid.NewV7(), entities.RoleDeveloper, nil)
	_, err := e.db.Exec(`INSERT INTO members (id, application_id, user_id, uri_id, role, created_at) VALUES (?, ?, ?, 'u2', 'developer', CURRENT_TIMESTAMP)`, other.ID.String(), a.Application.String(), other.UserID.String())
	require.NoError(t, err)
	theirs := entities.NewSecret(a.Application, other.ID, "THEIRS", entities.SecretString, entities.SourceStatic, nil, a.User)
	created, err := e.secrets.Create(context.Background(), entities.SecretWrite{Secret: theirs, VersionID: uuid.NewV7(), Ciphertext: []byte("c")})
	require.NoError(t, err)
	ctx := context.Background()

	mine, mineTotal, _ := e.secrets.List(ctx, a.Application, a.Member, 10, 0)
	_, wrongTable := e.secrets.FindByOwner(ctx, a.Application, a.Member, created.ID)
	_, rightTable := e.secrets.FindByOwner(ctx, a.Application, other.ID, created.ID)
	_, anyTable := e.secrets.FindById(ctx, a.Application, created.ID)

	assert.Equal(t, 1, mineTotal)
	assert.Equal(t, "MINE", mine[0].Name, "a listagem é só da tabela do dono")
	assert.ErrorIs(t, wrongTable, sql.ErrNoRows, "com o dono, não acha segredo de outra tabela")
	assert.NoError(t, rightTable)
	assert.NoError(t, anyTable, "FindById, sem dono, acha (é como a referência encontra o alvo)")
}

func whenCountingTheTable(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	e.add(t, a, "ONE", "c")
	two := e.add(t, a, "TWO", "c")
	require.NoError(t, e.secrets.Delete(context.Background(), a.Application, two.ID))

	count, err := e.secrets.Count(context.Background(), a.Application, a.Member)
	none, noneErr := e.secrets.Count(context.Background(), a.Application, uuid.NewV7())

	require.NoError(t, err)
	assert.Equal(t, 1, count, "o apagado não conta")
	require.NoError(t, noneErr)
	assert.Zero(t, none)
}

func whenCountingReferences(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	target := e.add(t, a, "TARGET", "c")
	for _, name := range []string{"ALIAS_A", "ALIAS_B"} {
		_, err := e.secrets.Create(context.Background(), entities.SecretWrite{Secret: entities.NewSecret(a.Application, a.Member, name, entities.SecretString, entities.SourceReference, &target.ID, a.User)})
		require.NoError(t, err)
	}

	referenced, err := e.secrets.CountReferences(context.Background(), a.Application, target.ID)
	none, noneErr := e.secrets.CountReferences(context.Background(), a.Application, uuid.NewV7())

	require.NoError(t, err)
	assert.Equal(t, 2, referenced)
	require.NoError(t, noneErr)
	assert.Zero(t, none)
}

func whenAddingAVersion(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	secret := e.add(t, a, "TOKEN", "v1")
	author := uuid.NewV7()

	updated, err := e.secrets.AddVersion(context.Background(), entities.SecretWrite{Secret: secret, VersionID: uuid.NewV7(), Ciphertext: []byte("v2"), Author: author})

	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	assert.NotNil(t, updated.UpdatedAt)
	var by string
	require.NoError(t, e.db.QueryRow(`SELECT created_by FROM secret_versions WHERE secret_id = ? AND version = 2`, secret.ID.String()).Scan(&by))
	assert.Equal(t, author.String(), by, "a versão tem o autor de quem gravou")
}

func whenTwoWritesRaceForTheVersion(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	stale := e.add(t, a, "TOKEN", "v1")
	_, err := e.secrets.AddVersion(context.Background(), entities.SecretWrite{Secret: stale, VersionID: uuid.NewV7(), Ciphertext: []byte("v2"), Author: a.User})
	require.NoError(t, err)

	_, lost := e.secrets.AddVersion(context.Background(), entities.SecretWrite{Secret: stale, VersionID: uuid.NewV7(), Ciphertext: []byte("v2-outro"), Author: a.User})

	assert.ErrorIs(t, lost, sql.ErrNoRows, "quem leu a versão 1 e chegou depois não grava outra versão 2")
	var versions int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM secret_versions WHERE secret_id = ?`, stale.ID.String()).Scan(&versions))
	assert.Equal(t, 2, versions)
}

func whenRenamingAndDeleting(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	ctx := context.Background()
	first, second := e.add(t, a, "FIRST", "c"), e.add(t, a, "SECOND", "c")

	renamed, err := e.secrets.Rename(ctx, entities.SecretWrite{Secret: entities.Secret{ID: first.ID, ApplicationID: a.Application, Name: "THIRD"}})
	_, clash := e.secrets.Rename(ctx, entities.SecretWrite{Secret: entities.Secret{ID: first.ID, ApplicationID: a.Application, Name: "SECOND"}})
	require.NoError(t, e.secrets.Delete(ctx, a.Application, second.ID))
	_, deleted := e.secrets.FindById(ctx, a.Application, second.ID)
	again := e.secrets.Delete(ctx, a.Application, second.ID)
	var kept int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM secret_versions WHERE secret_id = ?`, second.ID.String()).Scan(&kept))

	require.NoError(t, err)
	assert.Equal(t, "THIRD", renamed.Name)
	assert.Error(t, clash, "nome repetido")
	assert.ErrorIs(t, deleted, sql.ErrNoRows)
	assert.ErrorIs(t, again, sql.ErrNoRows)
	assert.Equal(t, 1, kept, "as versões cifradas ficam para a auditoria")
}

func whenListingVersions(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	ctx := context.Background()
	secret := e.add(t, a, "TOKEN", "v1")
	secret, err := e.secrets.AddVersion(ctx, entities.SecretWrite{Secret: secret, VersionID: uuid.NewV7(), Ciphertext: []byte("v2"), Author: a.User})
	require.NoError(t, err)

	all, total, _ := e.versions.List(ctx, a.Application, secret.ID, 10, 0)
	current, currentErr := e.versions.FindCurrent(ctx, a.Application, secret.ID)
	old, oldErr := e.versions.FindByNumber(ctx, a.Application, secret.ID, 1)
	_, missing := e.versions.FindByNumber(ctx, a.Application, secret.ID, 9)

	assert.Equal(t, 2, total)
	assert.Equal(t, []int{2, 1}, []int{all[0].Version, all[1].Version}, "da mais nova para a mais antiga")
	require.NoError(t, currentErr)
	assert.Equal(t, 2, current.Version)
	assert.Equal(t, []byte("v2"), current.Ciphertext)
	require.NoError(t, oldErr)
	assert.Equal(t, []byte("v1"), old.Ciphertext)
	assert.NotEqual(t, uuid.UUID{}, old.ID, "o id da versão entra no AAD da cifra")
	assert.ErrorIs(t, missing, sql.ErrNoRows)
}

func whenResealing(t *testing.T) {
	e := newEnv(t)
	a := sqlitetest.Seed(t, e.db, "app-a")
	secret := e.add(t, a, "TOKEN", "antigo")
	ctx := context.Background()
	current, err := e.versions.FindCurrent(ctx, a.Application, secret.ID)
	require.NoError(t, err)

	_, resealErr := e.versions.Reseal(ctx, a.Application, secret.ID, entities.SecretVersion{ID: current.ID, Ciphertext: []byte("novo")})
	_, missing := e.versions.Reseal(ctx, a.Application, secret.ID, entities.SecretVersion{ID: uuid.NewV7(), Ciphertext: []byte("novo")})
	_, immutable := e.db.Exec(`UPDATE secret_versions SET version = 9 WHERE id = ?`, current.ID.String())

	require.NoError(t, resealErr)
	after, _ := e.versions.FindCurrent(ctx, a.Application, secret.ID)
	assert.Equal(t, []byte("novo"), after.Ciphertext)
	assert.ErrorIs(t, missing, sql.ErrNoRows)
	assert.Error(t, immutable, "o banco só aceita trocar o ciphertext de uma versão")
}
