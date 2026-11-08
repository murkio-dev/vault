package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestTransactor(t *testing.T) {
	t.Run("Quando a função dá certo.", func(t *testing.T) { whenFunctionSucceeds(t) })
	t.Run("Quando a função devolve erro.", func(t *testing.T) { whenFunctionFails(t) })
	t.Run("Quando a função dá panic.", func(t *testing.T) { whenFunctionPanics(t) })
	t.Run("Quando há transações aninhadas.", func(t *testing.T) { whenTransactionsAreNested(t) })
	t.Run("Quando a conexão fora de transação é usada.", func(t *testing.T) { whenConnIsUsedOutsideATransaction(t) })
	t.Run("Quando o erro é de unicidade ou de restrição.", func(t *testing.T) { whenErrorIsAConstraintViolation(t) })
}

// Functions

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.New(t.TempDir() + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE items (name TEXT UNIQUE NOT NULL CHECK (length(name) < 10))`)
	require.NoError(t, err)
	return db
}

func count(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n))
	return n
}

func whenFunctionSucceeds(t *testing.T) {
	db := newDB(t)

	err := sqlite.NewTransactor(db).Do(context.Background(), func(ctx context.Context) error {
		_, err := sqlite.Conn(ctx, db).ExecContext(ctx, `INSERT INTO items VALUES ('a')`)
		return err
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, count(t, db))
}

func whenFunctionFails(t *testing.T) {
	db := newDB(t)
	boom := errors.New("boom")

	err := sqlite.NewTransactor(db).Do(context.Background(), func(ctx context.Context) error {
		if _, err := sqlite.Conn(ctx, db).ExecContext(ctx, `INSERT INTO items VALUES ('a')`); err != nil {
			return err
		}
		return boom
	})

	assert.ErrorIs(t, err, boom)
	assert.Zero(t, count(t, db), "tudo é desfeito")
}

func whenFunctionPanics(t *testing.T) {
	db := newDB(t)

	assert.Panics(t, func() {
		_ = sqlite.NewTransactor(db).Do(context.Background(), func(ctx context.Context) error {
			_, _ = sqlite.Conn(ctx, db).ExecContext(ctx, `INSERT INTO items VALUES ('a')`)
			panic("boom")
		})
	})

	assert.Zero(t, count(t, db), "o panic desfaz a transação em vez de a deixar aberta")
}

func whenTransactionsAreNested(t *testing.T) {
	db := newDB(t)
	transactor := sqlite.NewTransactor(db)

	err := transactor.Do(context.Background(), func(ctx context.Context) error {
		_, _ = sqlite.Conn(ctx, db).ExecContext(ctx, `INSERT INTO items VALUES ('a')`)
		return transactor.Do(ctx, func(inner context.Context) error {
			_, err := sqlite.Conn(inner, db).ExecContext(inner, `INSERT INTO items VALUES ('b')`)
			return err
		})
	})
	assert.NoError(t, err, "a de dentro reaproveita a transação (a conexão é única: abrir outra travaria)")
	assert.Equal(t, 2, count(t, db))

	err = transactor.Do(context.Background(), func(ctx context.Context) error {
		_, _ = sqlite.Conn(ctx, db).ExecContext(ctx, `INSERT INTO items VALUES ('c')`)
		_ = transactor.Do(ctx, func(context.Context) error { return errors.New("interna") })
		return errors.New("externa")
	})
	assert.Error(t, err)
	assert.Equal(t, 2, count(t, db), "quem abriu é quem fecha: a falha de fora desfaz o que a de dentro viu")
}

func whenConnIsUsedOutsideATransaction(t *testing.T) {
	db := newDB(t)

	assert.Same(t, db, sqlite.Conn(context.Background(), db).(*sql.DB))
}

func whenErrorIsAConstraintViolation(t *testing.T) {
	db := newDB(t)
	_, err := db.Exec(`INSERT INTO items VALUES ('a')`)
	require.NoError(t, err)

	_, duplicate := db.Exec(`INSERT INTO items VALUES ('a')`)
	_, check := db.Exec(`INSERT INTO items VALUES ('nome-grande-demais')`)

	assert.True(t, sqlite.IsUniqueViolation(duplicate))
	assert.True(t, sqlite.IsConstraintViolation(duplicate))
	assert.False(t, sqlite.IsUniqueViolation(check))
	assert.True(t, sqlite.IsConstraintViolation(check))
	assert.False(t, sqlite.IsUniqueViolation(errors.New("outro")))
	assert.False(t, sqlite.IsConstraintViolation(nil))
}
