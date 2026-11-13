package repositories_test

import (
	"context"
	"database/sql"
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
	t.Run("Quando expurga o que passou da retenção.", func(t *testing.T) { whenPurgingExpired(t) })
	t.Run("Quando o banco guarda a retenção.", func(t *testing.T) { whenDatabaseGuardsRetention(t) })
	t.Run("Quando o lote do expurgo é inválido.", func(t *testing.T) { whenPurgeLimitIsInvalid(t) })
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

func entryAged(t *testing.T, repo repositories.AuditRepository, app, actor uuid.UUID, age time.Duration) entities.AuditLog {
	t.Helper()
	log := entry(app, actor, entities.AuditSecretRevealed, entities.AuditAllowed)
	log.CreatedAt = time.Now().UTC().Add(-age)
	saved, err := repo.Create(context.Background(), log)
	require.NoError(t, err)
	return saved
}

func countAudit(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&n))
	return n
}

func whenPurgingExpired(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	day := 24 * time.Hour
	for range 3 {
		entryAged(t, repo, a.Application, a.User, 400*day)
	}
	entryAged(t, repo, a.Application, a.User, 10*day)
	entryAged(t, repo, a.Application, a.User, 364*day)
	inMargin := entryAged(t, repo, a.Application, a.User, 365*day+30*time.Minute)

	first, err := repo.PurgeExpired(context.Background(), time.Now(), 2)
	second, secondErr := repo.PurgeExpired(context.Background(), time.Now(), 2)
	third, thirdErr := repo.PurgeExpired(context.Background(), time.Now(), 2)

	require.NoError(t, err)
	require.NoError(t, secondErr)
	require.NoError(t, thirdErr)
	assert.EqualValues(t, 2, first, "respeita o tamanho do lote")
	assert.EqualValues(t, 1, second, "lote incompleto diz que o expurgo terminou")
	assert.EqualValues(t, 0, third)
	assert.Equal(t, 3, countAudit(t, db), "ficam a de 10 dias, a de 364 e a que está na folga de 1 h")
	var kept int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE id = ?`, inMargin.ID.String()).Scan(&kept))
	assert.Equal(t, 1, kept, "o corte fica 1 h além do limite do trigger")
}

func whenDatabaseGuardsRetention(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	recent := entryAged(t, repo, a.Application, a.User, 10*24*time.Hour)
	old := entryAged(t, repo, a.Application, a.User, 400*24*time.Hour)

	_, recentErr := db.Exec(`DELETE FROM audit_logs WHERE id = ?`, recent.ID.String())
	_, updateOldErr := db.Exec(`UPDATE audit_logs SET outcome = 'denied' WHERE id = ?`, old.ID.String())
	_, oldErr := db.Exec(`DELETE FROM audit_logs WHERE id = ?`, old.ID.String())

	assert.True(t, sqlite.IsConstraintViolation(recentErr), "dentro de 1 ano, nem o DELETE direto passa")
	assert.True(t, sqlite.IsConstraintViolation(updateOldErr), "reescrever é recusado mesmo no que já venceu")
	assert.NoError(t, oldErr, "o que passou de 1 ano pode ser apagado")
	assert.Equal(t, 1, countAudit(t, db))
}

func whenPurgeLimitIsInvalid(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)

	for _, limit := range []int{0, -1, repositories.MaxPurgeBatch + 1} {
		_, err := repo.PurgeExpired(context.Background(), time.Now(), limit)

		assert.Error(t, err, "limit %d", limit)
	}
}
