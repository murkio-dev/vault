package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/invitation/repositories"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestInvitationRepository(t *testing.T) {
	t.Run("Quando falta um id de isolamento.", func(t *testing.T) { whenIsolationIDIsMissing(t) })
	t.Run("Quando outro tenant pede os convites.", func(t *testing.T) { whenAnotherTenantAsks(t) })
	t.Run("Quando procura pelo hash do token.", func(t *testing.T) { whenLookingUpByTokenHash(t) })
	t.Run("Quando o convite é aceito.", func(t *testing.T) { whenInvitationIsAccepted(t) })
	t.Run("Quando o convite não está mais pendente.", func(t *testing.T) { whenInvitationIsNotPending(t) })
	t.Run("Quando o convite é revogado.", func(t *testing.T) { whenInvitationIsRevoked(t) })
}

// Functions

func add(t *testing.T, repo repositories.InvitationRepository, app uuid.UUID) (entities.Invitation, string) {
	t.Helper()
	invitation, token, err := entities.NewInvitation(app, "joao@example.com", entities.RoleDeveloper, uuid.NewV7())
	require.NoError(t, err)
	created, err := repo.Create(context.Background(), invitation)
	require.NoError(t, err)
	return created, token
}

// stored lê as colunas de estado do convite direto do banco.
func stored(t *testing.T, db *sql.DB, id uuid.UUID) (acceptedBy sql.NullString, revokedAt sql.NullTime) {
	t.Helper()
	require.NoError(t, db.QueryRow(`SELECT accepted_by, revoked_at FROM invitations WHERE id = ?`, id.String()).Scan(&acceptedBy, &revokedAt))
	return acceptedBy, revokedAt
}

func whenIsolationIDIsMissing(t *testing.T) {
	repo := repositories.NewRepository(sqlitetest.New(t))
	ctx := context.Background()
	zero := uuid.UUID{}
	invitation, _, _ := entities.NewInvitation(uuid.NewV7(), "joao@example.com", entities.RoleDeveloper, uuid.NewV7())
	now := time.Now().UTC()
	by := uuid.NewV7()
	invitation.AcceptedAt, invitation.AcceptedBy = &now, &by
	orphan := invitation
	orphan.ApplicationID = zero

	_, createErr := repo.Create(ctx, orphan)
	_, hashErr := repo.FindByTokenHash(ctx, "")
	_, _, listErr := repo.List(ctx, zero, 10, 0)
	_, acceptErr := repo.Accept(ctx, orphan)
	revokeErr := repo.Revoke(ctx, zero, invitation.ID)

	for name, err := range map[string]error{"Create": createErr, "FindByTokenHash": hashErr, "List": listErr, "Accept": acceptErr, "Revoke": revokeErr} {
		assert.ErrorIs(t, err, entities.ErrMissingParam, name)
	}
}

func whenAnotherTenantAsks(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a, b := sqlitetest.Seed(t, db, "app-a"), sqlitetest.Seed(t, db, "app-b")
	invitation, _ := add(t, repo, a.Application)

	list, total, listErr := repo.List(context.Background(), b.Application, 10, 0)
	revokeErr := repo.Revoke(context.Background(), b.Application, invitation.ID)
	now, by := time.Now().UTC(), uuid.NewV7()
	_, acceptErr := repo.Accept(context.Background(), entities.Invitation{ID: invitation.ID, ApplicationID: b.Application, AcceptedAt: &now, AcceptedBy: &by})

	require.NoError(t, listErr)
	assert.Zero(t, total)
	assert.Empty(t, list, "a outra aplicação não vê o convite")
	assert.ErrorIs(t, revokeErr, sql.ErrNoRows, "nem o revoga")
	assert.ErrorIs(t, acceptErr, sql.ErrNoRows, "nem o aceita por ela")
	acceptedBy, revokedAt := stored(t, db, invitation.ID)
	assert.False(t, acceptedBy.Valid)
	assert.False(t, revokedAt.Valid, "e o convite segue pendente")
}

func whenLookingUpByTokenHash(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	invitation, token := add(t, repo, a.Application)

	found, err := repo.FindByTokenHash(context.Background(), entities.HashToken(token))
	_, noneErr := repo.FindByTokenHash(context.Background(), entities.HashToken("inv_outro"))

	require.NoError(t, err)
	assert.Equal(t, invitation.ID, found.ID, "o token é a credencial: acha sem saber a aplicação")
	assert.Empty(t, found.TokenHash, "o hash não volta do banco")
	assert.ErrorIs(t, noneErr, sql.ErrNoRows)
}

func whenInvitationIsAccepted(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	invitation, _ := add(t, repo, a.Application)
	now, by := time.Now().UTC(), uuid.NewV7()

	accepted, err := repo.Accept(context.Background(), entities.Invitation{ID: invitation.ID, ApplicationID: a.Application, AcceptedAt: &now, AcceptedBy: &by})
	require.NoError(t, err)
	_, second := repo.Accept(context.Background(), entities.Invitation{ID: invitation.ID, ApplicationID: a.Application, AcceptedAt: &now, AcceptedBy: &by})

	assert.NotNil(t, accepted.AcceptedAt)
	assert.ErrorIs(t, second, sql.ErrNoRows, "o segundo aceite do mesmo link perde")
	acceptedBy, _ := stored(t, db, invitation.ID)
	if assert.True(t, acceptedBy.Valid) {
		assert.Equal(t, by.String(), acceptedBy.String)
	}
}

func whenInvitationIsNotPending(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	expired, _ := add(t, repo, a.Application)
	revoked, _ := add(t, repo, a.Application)
	_, err := db.Exec(`UPDATE invitations SET expires_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Minute), expired.ID.String())
	require.NoError(t, err)
	require.NoError(t, repo.Revoke(context.Background(), a.Application, revoked.ID))
	now, by := time.Now().UTC(), uuid.NewV7()

	_, expiredErr := repo.Accept(context.Background(), entities.Invitation{ID: expired.ID, ApplicationID: a.Application, AcceptedAt: &now, AcceptedBy: &by})
	_, revokedErr := repo.Accept(context.Background(), entities.Invitation{ID: revoked.ID, ApplicationID: a.Application, AcceptedAt: &now, AcceptedBy: &by})
	_, incomplete := repo.Accept(context.Background(), entities.Invitation{ID: expired.ID, ApplicationID: a.Application})

	assert.ErrorIs(t, expiredErr, sql.ErrNoRows, "vencido não se aceita")
	assert.ErrorIs(t, revokedErr, sql.ErrNoRows, "revogado também não")
	assert.ErrorIs(t, incomplete, entities.ErrMissingParam, "o aceite precisa de quem aceitou e quando")
}

func whenInvitationIsRevoked(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	invitation, _ := add(t, repo, a.Application)

	first := repo.Revoke(context.Background(), a.Application, invitation.ID)
	second := repo.Revoke(context.Background(), a.Application, invitation.ID)

	assert.NoError(t, first)
	assert.ErrorIs(t, second, sql.ErrNoRows, "revogar um convite já encerrado não existe")
	_, revokedAt := stored(t, db, invitation.ID)
	assert.True(t, revokedAt.Valid, "a linha fica, com revoked_at")
}
