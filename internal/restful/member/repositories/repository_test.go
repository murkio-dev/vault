package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/repositories"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestMemberRepository(t *testing.T) {
	t.Run("Quando falta um id de isolamento.", func(t *testing.T) { whenIsolationIDIsMissing(t) })
	t.Run("Quando outro tenant pede os dados.", func(t *testing.T) { whenAnotherTenantAsks(t) })
	t.Run("Quando o membro é de outra aplicação.", func(t *testing.T) { whenMemberBelongsToAnotherApplication(t) })
	t.Run("Quando acha o vínculo de uma conta.", func(t *testing.T) { whenFindingAnAccount(t) })
	t.Run("Quando conta os admins ativos.", func(t *testing.T) { whenCountingActiveAdmins(t) })
	t.Run("Quando troca o papel e o status.", func(t *testing.T) { whenUpgrading(t) })
	t.Run("Quando remove o membro.", func(t *testing.T) { whenDeleteCascades(t) })
}

// Functions

func add(t *testing.T, repo repositories.MemberRepository, app uuid.UUID, role entities.Role) entities.Member {
	t.Helper()
	member := entities.NewMember(app, uuid.NewV7(), role, nil)
	created, err := repo.Create(context.Background(), member)
	require.NoError(t, err)
	return created
}

func whenIsolationIDIsMissing(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	ctx := context.Background()
	zero := uuid.UUID{}
	member := entities.NewMember(uuid.NewV7(), uuid.NewV7(), entities.RoleAdmin, nil)
	orphan := member
	orphan.ApplicationID = zero

	_, createErr := repo.Create(ctx, orphan)
	_, findErr := repo.FindById(ctx, zero, member.ID)
	_, userErr := repo.FindByUser(ctx, zero, member.UserID)
	_, _, listErr := repo.List(ctx, zero, 10, 0)
	_, countErr := repo.CountActiveAdmins(ctx, zero)
	_, upgradeErr := repo.Upgrade(ctx, orphan)
	deleteErr := repo.Delete(ctx, zero, member.ID)

	for name, err := range map[string]error{"Create": createErr, "FindById": findErr, "FindByUser": userErr, "List": listErr, "CountActiveAdmins": countErr, "Upgrade": upgradeErr, "Delete": deleteErr} {
		assert.ErrorIs(t, err, entities.ErrMissingParam, name+": com a aplicação zerada o repositório não toca o banco, e zero não é \"sem filtro\"")
	}
}

func whenAnotherTenantAsks(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a, b := sqlitetest.Seed(t, db, "app-a"), sqlitetest.Seed(t, db, "app-b")
	ctx := context.Background()

	_, findErr := repo.FindById(ctx, b.Application, a.Member)
	list, total, listErr := repo.List(ctx, b.Application, 10, 0)
	deleteErr := repo.Delete(ctx, b.Application, a.Member)

	assert.ErrorIs(t, findErr, sql.ErrNoRows, "o id de um membro de outra aplicação não acha nada")
	require.NoError(t, listErr)
	assert.Equal(t, 1, total, "cada aplicação lista só os seus")
	assert.Equal(t, b.Member, list[0].ID)
	assert.ErrorIs(t, deleteErr, sql.ErrNoRows)
	found, err := repo.FindById(ctx, a.Application, a.Member)
	require.NoError(t, err)
	assert.Nil(t, found.InvitedBy, "e o membro da outra aplicação continua lá")
}

func whenMemberBelongsToAnotherApplication(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a, b := sqlitetest.Seed(t, db, "app-a"), sqlitetest.Seed(t, db, "app-b")

	_, upgradeErr := repo.Upgrade(context.Background(), entities.Member{ID: a.Member, ApplicationID: b.Application, Role: entities.RoleDeveloper, Status: entities.MemberActive})
	_, userErr := repo.FindByUser(context.Background(), b.Application, uuid.NewV7())

	assert.ErrorIs(t, upgradeErr, sql.ErrNoRows, "o membro vale na aplicação dele: com a aplicação de outro, não muda nada")
	assert.ErrorIs(t, userErr, sql.ErrNoRows)
}

func whenFindingAnAccount(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a, b := sqlitetest.Seed(t, db, "app-a"), sqlitetest.Seed(t, db, "app-b")
	dev := add(t, repo, a.Application, entities.RoleDeveloper)

	member, err := repo.FindByUser(context.Background(), a.Application, dev.UserID)
	_, otherApp := repo.FindByUser(context.Background(), b.Application, dev.UserID)
	_, absent := repo.FindByUser(context.Background(), a.Application, uuid.NewV7())

	require.NoError(t, err)
	assert.Equal(t, dev.ID, member.ID)
	assert.ErrorIs(t, otherApp, sql.ErrNoRows, "o vínculo é por aplicação")
	assert.ErrorIs(t, absent, sql.ErrNoRows)
}

func whenCountingActiveAdmins(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	second := add(t, repo, a.Application, entities.RoleAdmin)
	add(t, repo, a.Application, entities.RoleDeveloper)
	_, err := repo.Upgrade(context.Background(), entities.Member{ID: second.ID, ApplicationID: a.Application, Status: entities.MemberSuspended})
	require.NoError(t, err)

	admins, err := repo.CountActiveAdmins(context.Background(), a.Application)

	require.NoError(t, err)
	assert.Equal(t, 1, admins, "o admin suspenso e o developer não contam")
}

func whenUpgrading(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	dev := add(t, repo, a.Application, entities.RoleDeveloper)

	partial, err := repo.Upgrade(context.Background(), entities.Member{ID: dev.ID, ApplicationID: a.Application, Role: entities.RoleAdmin})
	require.NoError(t, err)
	status, err := repo.Upgrade(context.Background(), entities.Member{ID: dev.ID, ApplicationID: a.Application, Status: entities.MemberSuspended})
	require.NoError(t, err)
	_, missing := repo.Upgrade(context.Background(), entities.Member{ID: uuid.NewV7(), ApplicationID: a.Application, Role: entities.RoleAdmin})

	assert.Equal(t, entities.RoleAdmin, partial.Role)
	assert.Equal(t, entities.MemberActive, partial.Status, "o PATCH não toca no que não veio")
	assert.Equal(t, entities.RoleAdmin, status.Role, "e o status sozinho não troca o papel")
	assert.Equal(t, entities.MemberSuspended, status.Status)
	assert.NotNil(t, status.UpdatedAt)
	assert.ErrorIs(t, missing, sql.ErrNoRows)
}

func whenDeleteCascades(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	dev := add(t, repo, a.Application, entities.RoleDeveloper)
	now := time.Now().UTC()
	_, err := db.Exec(`INSERT INTO access_tokens (id, application_id, member_id, name, prefix, token_hash, created_by, created_at) VALUES ('t', ?, ?, 'n', 'vt_x', 'h', 'u', ?)`, a.Application.String(), dev.ID.String(), now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO secrets (id, application_id, member_id, name, type, source, created_by, created_at) VALUES ('s', ?, ?, 'A', 'string', 'static', 'u', ?)`, a.Application.String(), dev.ID.String(), now)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(context.Background(), a.Application, dev.ID))

	var revoked, deleted int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM access_tokens WHERE id = 't' AND revoked_at IS NOT NULL`).Scan(&revoked))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM secrets WHERE id = 's' AND deleted_at IS NOT NULL`).Scan(&deleted))
	assert.Equal(t, 1, revoked, "os tokens dele são revogados")
	assert.Equal(t, 1, deleted, "e os segredos, apagados")
	_, findErr := repo.FindById(context.Background(), a.Application, dev.ID)
	assert.ErrorIs(t, findErr, sql.ErrNoRows)
	assert.ErrorIs(t, repo.Delete(context.Background(), a.Application, dev.ID), sql.ErrNoRows, "apagar de novo não existe")
}
