package secretvalues_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/repositories"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/migration"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/secretvalues"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestValues(t *testing.T) {
	t.Run("Quando a chave de cifragem é rotacionada.", func(t *testing.T) { whenEncryptionKeyIsRotated(t) })
	t.Run("Quando lê uma versão antiga.", func(t *testing.T) { whenAnOldVersionIsRead(t) })
	t.Run("Quando resolve uma referência.", func(t *testing.T) { whenAReferenceIsResolved(t) })
	t.Run("Quando a referência perdeu o alvo.", func(t *testing.T) { whenTheReferenceTargetIsGone(t) })
	t.Run("Quando o segredo é de outra aplicação.", func(t *testing.T) { whenSecretBelongsToAnotherApplication(t) })
	t.Run("Quando o ciphertext é de outra versão.", func(t *testing.T) { whenCiphertextIsFromAnotherVersion(t) })
}

// Functions

type env struct {
	db       *sql.DB
	secrets  repositories.SecretRepository
	versions repositories.VersionRepository
	app      uuid.UUID
	member   uuid.UUID
	user     uuid.UUID
}

func newKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, secretbox.KeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := sqlite.New(t.TempDir() + "/vault.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, migration.New(db, migrations_embed.FS).Run(context.Background(), "sqlite"))

	e := &env{db: db, secrets: repositories.NewRepository(db), versions: repositories.NewVersionRepository(db), app: uuid.NewV7(), member: uuid.NewV7(), user: uuid.NewV7()}
	e.seedApplication(t, e.app, "minha-app", e.member)
	return e
}

func (e *env) seedApplication(t *testing.T, app uuid.UUID, name string, member uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	_, err := e.db.Exec(`INSERT INTO applications (id, name, created_by, created_at) VALUES (?, ?, ?, ?)`, app.String(), name, e.user.String(), now)
	require.NoError(t, err)
	_, err = e.db.Exec(`INSERT INTO members (id, application_id, user_id, uri_id, role, created_at) VALUES (?, ?, ?, ?, 'admin', ?)`, member.String(), app.String(), uuid.NewV7().String(), member.String(), now)
	require.NoError(t, err)
}

// store cifra e grava um segredo estático na aplicação, com o box dado.
func (e *env) store(t *testing.T, box *secretbox.Box, app, member uuid.UUID, name, value string) entities.Secret {
	t.Helper()
	versionID := uuid.NewV7()
	ciphertext, err := secretvalues.NewValues(e.secrets, e.versions, e.versions, e.versions, box).Seal(versionID, value)
	require.NoError(t, err)
	secret := entities.NewSecret(app, member, name, entities.SecretString, entities.SourceStatic, nil, e.user)
	created, err := e.secrets.Create(context.Background(), entities.SecretWrite{Secret: secret, VersionID: versionID, Ciphertext: ciphertext})
	require.NoError(t, err)
	return created
}

func (e *env) values(t *testing.T, box *secretbox.Box) secretvalues.Values {
	return secretvalues.NewValues(e.secrets, e.versions, e.versions, e.versions, box)
}

func whenEncryptionKeyIsRotated(t *testing.T) {
	e := newEnv(t)
	oldKey, newKeyBytes := newKey(t), newKey(t)
	oldBox, err := secretbox.New(oldKey)
	require.NoError(t, err)
	secret := e.store(t, oldBox, e.app, e.member, "DATABASE_URL", "valor-secreto")
	stored := func() []byte {
		current, err := e.versions.FindCurrent(context.Background(), e.app, secret.ID)
		require.NoError(t, err)
		versions := []entities.SecretVersion{current}
		return versions[0].Ciphertext
	}
	before := stored()

	rotated, err := secretbox.New(newKeyBytes, oldKey)
	require.NoError(t, err)
	value, version, err := e.values(t, rotated).Value(context.Background(), secret, 0)

	require.NoError(t, err)
	assert.Equal(t, "valor-secreto", value)
	assert.Equal(t, 1, version)
	assert.NotEqual(t, before, stored(), "o valor foi recifrado com a chave nova na leitura")

	onlyNew, err := secretbox.New(newKeyBytes)
	require.NoError(t, err)
	value, _, err = e.values(t, onlyNew).Value(context.Background(), secret, 0)
	require.NoError(t, err, "e agora abre sem a chave antiga")
	assert.Equal(t, "valor-secreto", value)

	_, _, err = e.values(t, oldBox).Value(context.Background(), secret, 0)
	assert.Error(t, err, "a chave antiga sozinha já não abre o recifrado")
}

func whenAnOldVersionIsRead(t *testing.T) {
	e := newEnv(t)
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	values := e.values(t, box)
	secret := e.store(t, box, e.app, e.member, "TOKEN", "v1")
	versionID := uuid.NewV7()
	ciphertext, err := values.Seal(versionID, "v2")
	require.NoError(t, err)
	secret, err = e.secrets.AddVersion(context.Background(), entities.SecretWrite{Secret: secret, VersionID: versionID, Ciphertext: ciphertext, Author: e.user})
	require.NoError(t, err)

	current, currentVersion, _ := values.Value(context.Background(), secret, 0)
	old, oldVersion, _ := values.Value(context.Background(), secret, 1)
	_, _, missing := values.Value(context.Background(), secret, 9)

	assert.Equal(t, [2]any{"v2", 2}, [2]any{current, currentVersion})
	assert.Equal(t, [2]any{"v1", 1}, [2]any{old, oldVersion})
	assert.ErrorIs(t, missing, sql.ErrNoRows)
}

func whenAReferenceIsResolved(t *testing.T) {
	e := newEnv(t)
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	target := e.store(t, box, e.app, e.member, "SHARED", "valor-do-alvo")
	alias := entities.NewSecret(e.app, e.member, "ALIAS", entities.SecretString, entities.SourceReference, &target.ID, e.user)
	alias, err = e.secrets.Create(context.Background(), entities.SecretWrite{Secret: alias})
	require.NoError(t, err)

	value, version, err := e.values(t, box).Value(context.Background(), alias, 0)

	require.NoError(t, err)
	assert.Equal(t, "valor-do-alvo", value)
	assert.Equal(t, 1, version, "a versão é a do alvo")
}

func whenTheReferenceTargetIsGone(t *testing.T) {
	e := newEnv(t)
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	target := e.store(t, box, e.app, e.member, "SHARED", "v")
	alias := entities.NewSecret(e.app, e.member, "ALIAS", entities.SecretString, entities.SourceReference, &target.ID, e.user)
	alias, err = e.secrets.Create(context.Background(), entities.SecretWrite{Secret: alias})
	require.NoError(t, err)
	require.NoError(t, e.secrets.Delete(context.Background(), e.app, target.ID))

	_, _, err = e.values(t, box).Value(context.Background(), alias, 0)

	assert.ErrorIs(t, err, sql.ErrNoRows)

	orphan := alias
	orphan.ReferenceSecretID = nil
	_, _, err = e.values(t, box).Value(context.Background(), orphan, 0)
	assert.ErrorIs(t, err, sql.ErrNoRows, "referência sem alvo")
}

func whenSecretBelongsToAnotherApplication(t *testing.T) {
	e := newEnv(t)
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	otherApp, otherMember := uuid.NewV7(), uuid.NewV7()
	e.seedApplication(t, otherApp, "outra-app", otherMember)
	foreign := e.store(t, box, otherApp, otherMember, "SECRET", "valor-alheio")

	mismatched := foreign
	mismatched.ApplicationID = e.app
	_, _, err = e.values(t, box).Value(context.Background(), mismatched, 0)

	assert.ErrorIs(t, err, sql.ErrNoRows, "o id de um segredo não abre as versões por outra aplicação")
}

func whenCiphertextIsFromAnotherVersion(t *testing.T) {
	e := newEnv(t)
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	a := e.store(t, box, e.app, e.member, "AAA", "valor-de-a")
	b := e.store(t, box, e.app, e.member, "BBB", "valor-de-b")
	_, err = e.db.Exec(`UPDATE secret_versions SET ciphertext = (SELECT ciphertext FROM secret_versions WHERE secret_id = ?) WHERE secret_id = ?`, a.ID.String(), b.ID.String())
	require.NoError(t, err)

	_, _, err = e.values(t, box).Value(context.Background(), b, 0)

	assert.Error(t, err, "o ciphertext de uma linha não abre em outra (o AAD é a versão)")
}
