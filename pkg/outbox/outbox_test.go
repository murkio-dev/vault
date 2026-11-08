package outbox_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/migration"
	"github.com/murkio-dev/vault/pkg/outbox"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestOutbox(t *testing.T) {
	t.Run("Quando enfileira o e-mail do convite.", func(t *testing.T) { whenInvitationEmailIsEnqueued(t) })
	t.Run("Quando enfileira dentro de uma transação desfeita.", func(t *testing.T) { whenEnqueueIsRolledBack(t) })
	t.Run("Quando o contrato do e-mail é montado.", func(t *testing.T) { whenInvitationEmailContractIsBuilt(t) })
}

// Functions

func setup(t *testing.T) (outbox.Outbox, sqlite.Transactor, uuid.UUID, func(query string) int) {
	t.Helper()
	db, err := sqlite.New(t.TempDir() + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, migration.New(db, migrations_embed.FS).Run(context.Background(), "sqlite"))

	app := uuid.NewV7()
	_, err = db.Exec(`INSERT INTO applications (id, name, created_by, created_at) VALUES (?, 'minha-app', 'u', CURRENT_TIMESTAMP)`, app.String())
	require.NoError(t, err)

	return outbox.New(db), sqlite.NewTransactor(db), app, func(query string) int {
		var n int
		require.NoError(t, db.QueryRow(query).Scan(&n))
		return n
	}
}

func newEvent(app uuid.UUID) entities.OutboxEvent {
	invitation, token, _ := entities.NewInvitation(app, "joao@example.com", entities.RoleDeveloper, uuid.NewV7())
	event, _ := entities.NewInvitationEmail(invitation, token)
	return event
}

func whenInvitationEmailIsEnqueued(t *testing.T) {
	queue, _, app, count := setup(t)

	created, err := queue.Create(context.Background(), newEvent(app))

	require.NoError(t, err)
	assert.NotEqual(t, uuid.UUID{}, created.ID)
	assert.Equal(t, 1, count(`SELECT COUNT(*) FROM outbox_events WHERE type = 'email.send' AND exchange = 'messaging.commands' AND envelope = 0 AND published_at IS NULL`))

	_, err = queue.Create(context.Background(), newEvent(uuid.NewV7()))
	assert.Error(t, err, "application_id precisa existir: a outbox respeita a chave estrangeira")
}

func whenEnqueueIsRolledBack(t *testing.T) {
	queue, transactor, app, count := setup(t)

	err := transactor.Do(context.Background(), func(ctx context.Context) error {
		_, err := queue.Create(ctx, newEvent(app))
		require.NoError(t, err)
		return assert.AnError
	})

	assert.Error(t, err)
	assert.Zero(t, count(`SELECT COUNT(*) FROM outbox_events`), "a outbox entra e sai com a transação da mudança")
}

func whenInvitationEmailContractIsBuilt(t *testing.T) {
	invitation, token, err := entities.NewInvitation(uuid.NewV7(), "joao@example.com", entities.RoleDeveloper, uuid.NewV7())
	require.NoError(t, err)

	event, err := entities.NewInvitationEmail(invitation, token)

	require.NoError(t, err)
	assert.Equal(t, entities.MessagingExchange, event.Exchange)
	assert.Equal(t, entities.EmailSendRoutingKey, event.Type)
	assert.False(t, event.Envelope)
	assert.JSONEq(t, `{"id":"`+event.ID.String()+`","application_id":"`+invitation.ApplicationID.String()+`","to":"joao@example.com","template":"vault_invitation","locale":"en","data":{"token":"`+token+`","role":"developer"}}`, string(event.Payload))
}
