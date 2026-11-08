package repositories_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/repositories"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestAuditRepository(t *testing.T) {
	t.Run("Quando grava e lê uma linha.", func(t *testing.T) { whenWritingAndReading(t) })
	t.Run("Quando falta a aplicação.", func(t *testing.T) { whenApplicationIsMissing(t) })
	t.Run("Quando outro tenant lê a trilha.", func(t *testing.T) { whenAnotherTenantReads(t) })
	t.Run("Quando filtra a trilha.", func(t *testing.T) { whenFiltering(t) })
	t.Run("Quando pagina por cursor.", func(t *testing.T) { whenPagingByCursor(t) })
	t.Run("Quando alguém tenta alterar a trilha.", func(t *testing.T) { whenSomeoneTriesToTamper(t) })
}

// Functions

func entry(app uuid.UUID, actor uuid.UUID, action string, outcome entities.AuditOutcome) entities.AuditLog {
	requester := entities.ApiHeader{ClientIP: "203.0.113.7", UserAgent: "go-test", RequestID: "req-1", Actor: entities.Actor{Kind: entities.ActorUser, UserID: actor}}
	return entities.NewAuditLog(requester, app, action, outcome, entities.AuditTarget{Type: "secret", ID: "abc"}, map[string]any{"name": "DATABASE_URL"})
}

func whenWritingAndReading(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	log := entry(a.Application, a.User, entities.AuditSecretRevealed, entities.AuditAllowed)

	created, err := repo.Create(context.Background(), log)
	read, readErr := repo.List(context.Background(), entities.AuditFilter{ApplicationID: a.Application}, 10)

	require.NoError(t, err)
	assert.Equal(t, log.ID, created.ID)
	require.NoError(t, readErr)
	require.Len(t, read, 1)
	assert.Equal(t, entities.AuditSecretRevealed, read[0].Action)
	assert.Equal(t, "abc", read[0].TargetID)
	assert.JSONEq(t, `{"name":"DATABASE_URL"}`, string(read[0].Metadata))
	if assert.NotNil(t, read[0].ActorID) {
		assert.Equal(t, a.User, *read[0].ActorID)
	}
}

func whenApplicationIsMissing(t *testing.T) {
	repo := repositories.NewRepository(sqlitetest.New(t))

	_, err := repo.List(context.Background(), entities.AuditFilter{}, 10)

	assert.ErrorIs(t, err, entities.ErrMissingParam, "sem aplicação, a trilha de ninguém é lida")
}

func whenAnotherTenantReads(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a, b := sqlitetest.Seed(t, db, "app-a"), sqlitetest.Seed(t, db, "app-b")
	_, err := repo.Create(context.Background(), entry(a.Application, a.User, entities.AuditSecretRevealed, entities.AuditAllowed))
	require.NoError(t, err)

	read, err := repo.List(context.Background(), entities.AuditFilter{ApplicationID: b.Application}, 10)

	require.NoError(t, err)
	assert.Empty(t, read, "a trilha de A não aparece na leitura de B")
}

func whenFiltering(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	other := uuid.NewV7()
	for _, log := range []entities.AuditLog{
		entry(a.Application, a.User, entities.AuditSecretRevealed, entities.AuditAllowed),
		entry(a.Application, other, entities.AuditSecretRevealed, entities.AuditAllowed),
		entry(a.Application, other, entities.AuditAccessDenied, entities.AuditDenied),
	} {
		_, err := repo.Create(context.Background(), log)
		require.NoError(t, err)
	}
	read := func(filter entities.AuditFilter) []entities.AuditLog {
		filter.ApplicationID = a.Application
		logs, err := repo.List(context.Background(), filter, 10)
		require.NoError(t, err)
		return logs
	}
	now := time.Now().UTC()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)

	assert.Len(t, read(entities.AuditFilter{}), 3)
	assert.Len(t, read(entities.AuditFilter{ActorID: &other}), 2)
	assert.Len(t, read(entities.AuditFilter{Action: entities.AuditSecretRevealed}), 2)
	assert.Len(t, read(entities.AuditFilter{Outcome: entities.AuditDenied}), 1)
	assert.Len(t, read(entities.AuditFilter{TargetID: "abc"}), 3)
	assert.Empty(t, read(entities.AuditFilter{TargetID: "outro"}))
	assert.Len(t, read(entities.AuditFilter{From: &past, To: &future}), 3)
	assert.Empty(t, read(entities.AuditFilter{From: &future}))
	assert.Empty(t, read(entities.AuditFilter{To: &past}))
}

func whenPagingByCursor(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	for i := 0; i < 7; i++ {
		_, err := repo.Create(context.Background(), entry(a.Application, a.User, entities.AuditSecretRevealed, entities.AuditAllowed))
		require.NoError(t, err)
	}

	seen := map[uuid.UUID]bool{}
	var cursor uuid.UUID
	pages := 0
	for {
		logs, err := repo.List(context.Background(), entities.AuditFilter{ApplicationID: a.Application, Cursor: cursor}, 3)
		require.NoError(t, err)
		if len(logs) == 0 {
			break
		}
		for _, log := range logs {
			assert.False(t, seen[log.ID], "uma linha nunca aparece em duas páginas")
			seen[log.ID] = true
		}
		cursor = logs[len(logs)-1].ID
		pages++
		require.Less(t, pages, 10)
	}

	assert.Len(t, seen, 7, "o cursor percorre a trilha inteira")
	assert.Equal(t, 3, pages)
}

func whenSomeoneTriesToTamper(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	log, err := repo.Create(context.Background(), entry(a.Application, a.User, entities.AuditSecretRevealed, entities.AuditAllowed))
	require.NoError(t, err)

	_, updateErr := db.Exec(`UPDATE audit_logs SET outcome = 'denied' WHERE id = ?`, log.ID.String())
	_, deleteErr := db.Exec(`DELETE FROM audit_logs WHERE id = ?`, log.ID.String())
	_, duplicateErr := repo.Create(context.Background(), log)

	assert.True(t, sqlite.IsConstraintViolation(updateErr), "o banco recusa reescrever")
	assert.True(t, sqlite.IsConstraintViolation(deleteErr), "e apagar")
	assert.Error(t, duplicateErr, "o mesmo id não entra duas vezes")
}
