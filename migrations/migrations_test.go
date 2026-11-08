package migrations_embed_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/migration"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func migrated(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.New(t.TempDir() + "/vault.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, migration.New(db, migrations_embed.FS).Run(context.Background(), "sqlite"))
	return db
}

func TestSchema(t *testing.T) {
	t.Run("Quando as migrations rodam.", func(t *testing.T) { whenMigrationsRun(t) })
	t.Run("Quando as migrations rodam de novo.", func(t *testing.T) { whenMigrationsRunTwice(t) })
	t.Run("Quando alguém altera a auditoria.", func(t *testing.T) { whenAuditIsTampered(t) })
	t.Run("Quando alguém altera uma versão de segredo.", func(t *testing.T) { whenSecretVersionIsTampered(t) })
	t.Run("Quando o nome de uma aplicação apagada é reusado.", func(t *testing.T) { whenApplicationNameIsReused(t) })
}

func whenMigrationsRun(t *testing.T) {
	db := migrated(t)

	for _, table := range []string{"applications", "members", "invitations", "secrets", "secret_versions", "access_tokens", "audit_logs", "outbox_events"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		assert.NoError(t, err, table)
	}
}

func whenMigrationsRunTwice(t *testing.T) {
	db := migrated(t)

	require.NoError(t, migration.New(db, migrations_embed.FS).Run(context.Background(), "sqlite"))

	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var version string
		require.NoError(t, rows.Scan(&version))
		assert.True(t, strings.HasSuffix(version, ".up.sql"), "uma .down.sql nunca roda no boot: %s", version)
		count++
	}
	assert.Equal(t, 8, count)
}

func seedApplication(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO applications (id, name, created_by, created_at) VALUES (?, ?, 'u', ?)`, id, name, time.Now().UTC())
	require.NoError(t, err)
}

func whenAuditIsTampered(t *testing.T) {
	db := migrated(t)
	_, err := db.Exec(`INSERT INTO audit_logs (id, actor_type, action, outcome, created_at) VALUES ('1', 'user', 'secret.revealed', 'allowed', ?)`, time.Now().UTC())
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE audit_logs SET outcome = 'denied' WHERE id = '1'`)
	assert.Error(t, err, "a trilha não pode ser reescrita")
	_, err = db.Exec(`DELETE FROM audit_logs WHERE id = '1'`)
	assert.Error(t, err, "a trilha não pode ser apagada")
	assert.True(t, sqlite.IsConstraintViolation(err))

	_, err = db.Exec(`INSERT INTO audit_logs (id, actor_type, action, outcome, metadata, created_at) VALUES ('2', 'user', 'x', 'allowed', 'não é json', ?)`, time.Now().UTC())
	assert.Error(t, err, "metadata precisa ser JSON")
}

func whenSecretVersionIsTampered(t *testing.T) {
	db := migrated(t)
	now := time.Now().UTC()
	seedApplication(t, db, "app", "minha-app")
	_, err := db.Exec(`INSERT INTO members (id, application_id, user_id, uri_id, role, created_at) VALUES ('m', 'app', 'u', 'uri', 'admin', ?)`, now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO secrets (id, application_id, member_id, name, type, source, created_by, created_at) VALUES ('s', 'app', 'm', 'A', 'string', 'static', 'u', ?)`, now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO secret_versions (id, secret_id, version, ciphertext, created_by, created_at) VALUES ('v', 's', 1, x'01', 'u', ?)`, now)
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE secret_versions SET ciphertext = x'02' WHERE id = 'v'`)
	assert.NoError(t, err, "recifrar com a chave nova é permitido")
	_, err = db.Exec(`UPDATE secret_versions SET version = 2 WHERE id = 'v'`)
	assert.Error(t, err, "o resto da versão não muda")
	_, err = db.Exec(`DELETE FROM secret_versions WHERE id = 'v'`)
	assert.Error(t, err, "versão não se apaga")
}

func whenApplicationNameIsReused(t *testing.T) {
	db := migrated(t)
	seedApplication(t, db, "a", "minha-app")

	_, err := db.Exec(`INSERT INTO applications (id, name, created_by, created_at) VALUES ('b', 'minha-app', 'u', ?)`, time.Now().UTC())
	assert.True(t, sqlite.IsUniqueViolation(err), "duas ativas com o mesmo nome")

	_, err = db.Exec(`UPDATE applications SET deleted_at = ? WHERE id = 'a'`, time.Now().UTC())
	require.NoError(t, err)
	seedApplication(t, db, "c", "minha-app")
}
