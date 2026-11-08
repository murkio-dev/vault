// Package sqlitetest dá aos testes de repositório um banco SQLite real, em
// arquivo temporário, com todas as migrations aplicadas. Fica fora do pacote
// sqlite para o binário não carregar o pacote testing.
package sqlitetest

import (
	"context"
	"database/sql"
	"testing"
	"time"
	"uuid"

	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/migration"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// New abre o banco migrado e o fecha no fim do teste.
func New(t testing.TB) *sql.DB {
	t.Helper()

	db, err := sqlite.New(t.TempDir() + "/vault.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := migration.New(db, migrations_embed.FS).Run(context.Background(), "sqlite"); err != nil {
		t.Fatal(err)
	}

	return db
}

// Tenant é uma aplicação semeada com um membro admin.
type Tenant struct {
	Application uuid.UUID
	Member      uuid.UUID
	User        uuid.UUID
}

// Seed cria uma aplicação com um membro admin e devolve os ids. É o que os
// testes de isolamento usam para ter dois tenants lado a lado.
func Seed(t testing.TB, db *sql.DB, name string) Tenant {
	t.Helper()

	tenant := Tenant{Application: uuid.NewV7(), Member: uuid.NewV7(), User: uuid.NewV7()}
	now := time.Now().UTC()

	if _, err := db.Exec(`INSERT INTO applications (id, name, created_by, created_at) VALUES (?, ?, ?, ?)`,
		tenant.Application.String(), name, tenant.User.String(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO members (id, application_id, user_id, uri_id, role, created_at) VALUES (?, ?, ?, ?, 'admin', ?)`,
		tenant.Member.String(), tenant.Application.String(), tenant.User.String(), tenant.Member.String(), now); err != nil {
		t.Fatal(err)
	}

	return tenant
}
