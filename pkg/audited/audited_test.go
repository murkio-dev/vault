package audited_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	"github.com/murkio-dev/vault/pkg/audited"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestAudited(t *testing.T) {
	t.Run("Quando a mudança e a auditoria dão certo.", func(t *testing.T) { whenBothSucceed(t) })
	t.Run("Quando a mudança falha.", func(t *testing.T) { whenChangeFails(t) })
	t.Run("Quando a auditoria falha.", func(t *testing.T) { whenAuditFails(t) })
}

// Mocks

// recorder devolve um fake do repositório da trilha que grava na tabela trail
// do banco de teste, dentro da transação do ctx, ou falha com err.
func recorder(db *sql.DB, err error) *audit_mocks.AuditRepo {
	return &audit_mocks.AuditRepo{CreateFunc: func(ctx context.Context, log entities.AuditLog) (entities.AuditLog, error) {
		if err != nil {
			return entities.AuditLog{}, err
		}
		_, execErr := sqlite.Conn(ctx, db).ExecContext(ctx, `INSERT INTO trail VALUES (?)`, log.Action)
		return log, execErr
	}}
}

// Functions

func setup(t *testing.T, recorderErr error) (audited.Audited, *sql.DB) {
	t.Helper()
	db, err := sqlite.New(t.TempDir() + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE changes (name TEXT); CREATE TABLE trail (action TEXT)`)
	require.NoError(t, err)
	return audited.New(sqlite.NewTransactor(db), recorder(db, recorderErr)), db
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&n))
	return n
}

func entry() entities.AuditLog {
	return entities.NewAuditLog(entities.ApiHeader{}, uuid.NewV7(), entities.AuditSecretCreated, entities.AuditAllowed, entities.AuditTarget{}, nil)
}

func change(db *sql.DB, err error) func(context.Context) error {
	return func(ctx context.Context) error {
		if _, execErr := sqlite.Conn(ctx, db).ExecContext(ctx, `INSERT INTO changes VALUES ('x')`); execErr != nil {
			return execErr
		}
		return err
	}
}

func whenBothSucceed(t *testing.T) {
	tx, db := setup(t, nil)

	err := tx.Run(context.Background(), entry(), change(db, nil))

	assert.NoError(t, err)
	assert.Equal(t, 1, count(t, db, "changes"))
	assert.Equal(t, 1, count(t, db, "trail"))
}

func whenChangeFails(t *testing.T) {
	tx, db := setup(t, nil)
	boom := errors.New("boom")

	err := tx.Run(context.Background(), entry(), change(db, boom))

	assert.ErrorIs(t, err, boom)
	assert.Zero(t, count(t, db, "changes"))
	assert.Zero(t, count(t, db, "trail"), "mudança que falhou não deixa linha na trilha")
}

func whenAuditFails(t *testing.T) {
	tx, db := setup(t, errors.New("disk full"))

	err := tx.Run(context.Background(), entry(), change(db, nil))

	assert.Error(t, err)
	assert.Zero(t, count(t, db, "changes"), "fail-closed: sem trilha, a mudança é desfeita")
}
