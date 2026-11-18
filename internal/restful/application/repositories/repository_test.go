package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	application_repositories "github.com/murkio-dev/vault/internal/restful/application/repositories"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/migration"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sqlite.New(t.TempDir() + "/vault.db")
	assert.NoError(t, err)

	assert.NoError(t, migration.New(db, migrations_embed.FS).Run(context.Background(), "sqlite"))

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// Main test

func TestSwitch(t *testing.T) {
	db := setupTestDB(t)

	// Small tests
	t.Run("Quando o repositório é inicializado com sucesso.", func(t *testing.T) { whenBuildSuccess(t, db) })
	t.Run("Quando cria uma aplicação com sucesso.", func(t *testing.T) { whenCreateSuccess(t, db) })
	t.Run("Quando falha ao criar por chave primária duplicada.", func(t *testing.T) { whenCreateFailsOnDuplicateID(t, db) })
	t.Run("Quando a criação falha devido ao contexto cancelado.", func(t *testing.T) { whenCreateFailsDueToContext(t, db) })
	t.Run("Quando falha ao criar por nome de aplicação duplicado.", func(t *testing.T) { whenCreateFailsOnDuplicateName(t, db) })
	t.Run("Quando verifica a persistência direta das colunas no banco.", func(t *testing.T) { whenCreatePersistsAllFields(t, db) })
	t.Run("Quando verifica a existência e a aplicação existe no banco.", func(t *testing.T) { whenExistReturnsTrue(t, db) })
	t.Run("Quando verifica a existência e a aplicação não existe no banco.", func(t *testing.T) { whenExistReturnsFalse(t, db) })
	t.Run("Quando a consulta de existência falha devido ao contexto cancelado.", func(t *testing.T) { whenExistFailsDueToContext(t, db) })
	t.Run("Quando acha a aplicação pelo id.", func(t *testing.T) { whenFindByIDSucceeds(t) })
	t.Run("Quando a aplicação não existe.", func(t *testing.T) { whenFindByIDDoesNotExist(t) })
	t.Run("Quando a busca falha devido ao contexto cancelado.", func(t *testing.T) { whenFindByIDFailsDueToContext(t) })
	t.Run("Quando lista com página e total.", func(t *testing.T) { whenListPaginates(t) })
	t.Run("Quando a listagem falha devido ao contexto cancelado.", func(t *testing.T) { whenListFailsDueToContext(t) })
	t.Run("Quando lista só as aplicações da conta.", func(t *testing.T) { whenListByUserReturnsOnlyOwnApplications(t) })
	t.Run("Quando a conta não é informada.", func(t *testing.T) { whenListByUserHasNoUser(t) })
	t.Run("Quando substitui o nome.", func(t *testing.T) { whenUpdateReplacesName(t) })
	t.Run("Quando atualiza uma aplicação que não existe.", func(t *testing.T) { whenUpdateDoesNotFindApplication(t) })
	t.Run("Quando o upgrade vem sem nome.", func(t *testing.T) { whenUpgradeHasNoName(t) })
	t.Run("Quando o upgrade traz um nome novo.", func(t *testing.T) { whenUpgradeChangesName(t) })
	t.Run("Quando o upgrade não acha a aplicação.", func(t *testing.T) { whenUpgradeDoesNotFindApplication(t) })
	t.Run("Quando apaga a aplicação e a cascata.", func(t *testing.T) { whenDeleteCascades(t) })
	t.Run("Quando apaga uma aplicação que não existe ou já foi apagada.", func(t *testing.T) { whenDeleteDoesNotFindApplication(t) })
}

// Functions

func whenBuildSuccess(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)

	assert.NotNil(t, repository)
	assert.NotNil(t, repository.DB)
}

func whenCreateSuccess(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)
	ctx := context.Background()

	application, err := entities.NewApplication(pkgmocks.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	savedApplication, err := repository.Create(ctx, application)

	assert.NoError(t, err)
	assert.NotNil(t, savedApplication)
	assert.Equal(t, application.ID, savedApplication.ID)
	assert.Equal(t, application.Name, savedApplication.Name)
}

func whenCreateFailsOnDuplicateID(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)
	ctx := context.Background()

	application, err := entities.NewApplication(pkgmocks.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	_, err = repository.Create(ctx, application)
	assert.NoError(t, err)

	savedApplication, err := repository.Create(ctx, application)

	assert.Error(t, err)
	assert.Zero(t, savedApplication)
}

func whenCreateFailsDueToContext(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	application, err := entities.NewApplication(pkgmocks.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	savedApplication, err := repository.Create(ctx, application)

	assert.Error(t, err)
	assert.Zero(t, savedApplication)
}

func whenCreateFailsOnDuplicateName(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)
	ctx := context.Background()

	sameName := pkgmocks.GenRandomApplicationName()

	application1, err := entities.NewApplication(sameName, uuid.New())
	assert.NoError(t, err)
	_, err = repository.Create(ctx, application1)
	assert.NoError(t, err)

	application2, err := entities.NewApplication(sameName, uuid.New())
	assert.NoError(t, err)

	savedApplication2, err := repository.Create(ctx, application2)

	assert.Error(t, err)
	assert.Zero(t, savedApplication2)
}

func whenCreatePersistsAllFields(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	application, err := entities.NewApplication(pkgmocks.GenRandomApplicationName(), userID)
	assert.NoError(t, err)

	_, err = repository.Create(ctx, application)
	assert.NoError(t, err)

	var id, name, createdBy string
	var createdAt time.Time
	err = db.QueryRow("SELECT id, name, created_by, created_at FROM applications WHERE id = ?", application.ID.String()).Scan(&id, &name, &createdBy, &createdAt)

	assert.NoError(t, err)
	assert.Equal(t, application.ID.String(), id)
	assert.Equal(t, application.Name, name)
	assert.Equal(t, userID.String(), createdBy)
	assert.WithinDuration(t, application.CreatedAt, createdAt, time.Second)
}

func whenExistReturnsTrue(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)
	ctx := context.Background()

	application, err := entities.NewApplication(pkgmocks.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	_, err = repository.Create(ctx, application)
	assert.NoError(t, err)

	exists, err := repository.Exist(ctx, application)

	assert.NoError(t, err)
	assert.True(t, exists)
}

func whenExistReturnsFalse(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)
	ctx := context.Background()

	application, err := entities.NewApplication(pkgmocks.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	exists, err := repository.Exist(ctx, application)

	assert.NoError(t, err)
	assert.False(t, exists)
}

func whenExistFailsDueToContext(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	application, err := entities.NewApplication(pkgmocks.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	exists, err := repository.Exist(ctx, application)

	assert.Error(t, err)
	assert.False(t, exists)
}

func whenFindByIDSucceeds(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	name := pkgmocks.GenRandomApplicationName()
	tenant := sqlitetest.Seed(t, db, name)

	found, err := repository.FindById(context.Background(), tenant.Application)

	assert.NoError(t, err)
	assert.Equal(t, tenant.Application, found.ID)
	assert.Equal(t, name, found.Name)
	assert.Equal(t, tenant.User, found.CreatedBy)
}

func whenFindByIDDoesNotExist(t *testing.T) {
	repository := application_repositories.NewRepository(sqlitetest.New(t))

	found, err := repository.FindById(context.Background(), uuid.NewV7())

	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Zero(t, found)
}

func whenFindByIDFailsDueToContext(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	tenant := sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	found, err := repository.FindById(ctx, tenant.Application)

	assert.Error(t, err)
	assert.Zero(t, found)
}

func whenListPaginates(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	for range 3 {
		sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())
	}

	first, total, err := repository.List(context.Background(), 2, 0)
	assert.NoError(t, err)
	assert.Len(t, first, 2)
	assert.Equal(t, 3, total)

	last, total, err := repository.List(context.Background(), 2, 2)
	assert.NoError(t, err)
	assert.Len(t, last, 1)
	assert.Equal(t, 3, total)
	assert.NotEqual(t, first[0].ID, last[0].ID)

	beyond, total, err := repository.List(context.Background(), 2, 10)
	assert.NoError(t, err)
	assert.NotNil(t, beyond)
	assert.Empty(t, beyond)
	assert.Equal(t, 3, total)
}

func whenListFailsDueToContext(t *testing.T) {
	repository := application_repositories.NewRepository(sqlitetest.New(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	list, total, err := repository.List(ctx, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, list)
	assert.Zero(t, total)
}

func whenListByUserReturnsOnlyOwnApplications(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	mine := sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())
	sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())

	list, total, err := repository.ListByUser(context.Background(), mine.User, 10, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	if assert.Len(t, list, 1) {
		assert.Equal(t, mine.Application, list[0].ID)
	}

	stranger, total, err := repository.ListByUser(context.Background(), uuid.NewV7(), 10, 0)
	assert.NoError(t, err)
	assert.NotNil(t, stranger)
	assert.Empty(t, stranger)
	assert.Zero(t, total)
}

func whenListByUserHasNoUser(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())

	list, total, err := repository.ListByUser(context.Background(), uuid.UUID{}, 10, 0)

	assert.ErrorIs(t, err, entities.ErrMissingParam, "zero não é \"sem filtro\": não devolve as aplicações de todo mundo")
	assert.Nil(t, list)
	assert.Zero(t, total)
}

func whenUpdateReplacesName(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	tenant := sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())
	newName := pkgmocks.GenRandomApplicationName()

	updated, err := repository.Update(context.Background(), entities.Application{ID: tenant.Application, Name: newName})

	assert.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, tenant.Application, updated.ID)
}

func whenUpdateDoesNotFindApplication(t *testing.T) {
	repository := application_repositories.NewRepository(sqlitetest.New(t))

	updated, err := repository.Update(context.Background(), entities.Application{ID: uuid.NewV7(), Name: pkgmocks.GenRandomApplicationName()})

	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Zero(t, updated)
}

func whenUpgradeHasNoName(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	name := pkgmocks.GenRandomApplicationName()
	tenant := sqlitetest.Seed(t, db, name)

	upgraded, err := repository.Upgrade(context.Background(), entities.Application{ID: tenant.Application})

	assert.NoError(t, err)
	assert.Equal(t, name, upgraded.Name, "nome vazio mantém o atual")
}

func whenUpgradeChangesName(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	tenant := sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())
	newName := pkgmocks.GenRandomApplicationName()

	upgraded, err := repository.Upgrade(context.Background(), entities.Application{ID: tenant.Application, Name: newName})

	assert.NoError(t, err)
	assert.Equal(t, newName, upgraded.Name)
}

func whenUpgradeDoesNotFindApplication(t *testing.T) {
	repository := application_repositories.NewRepository(sqlitetest.New(t))

	upgraded, err := repository.Upgrade(context.Background(), entities.Application{ID: uuid.NewV7(), Name: pkgmocks.GenRandomApplicationName()})

	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Zero(t, upgraded)
}

func whenDeleteCascades(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	deleted := sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())
	kept := sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())

	assert.NoError(t, repository.Delete(context.Background(), deleted.Application))

	_, err := repository.FindById(context.Background(), deleted.Application)
	assert.ErrorIs(t, err, sql.ErrNoRows)

	var activeMembers int
	assert.NoError(t, db.QueryRow("SELECT COUNT(*) FROM members WHERE application_id = ? AND deleted_at IS NULL", deleted.Application.String()).Scan(&activeMembers))
	assert.Zero(t, activeMembers, "os membros da aplicação apagada saem junto")

	_, err = repository.FindById(context.Background(), kept.Application)
	assert.NoError(t, err, "a outra aplicação não é tocada")
	assert.NoError(t, db.QueryRow("SELECT COUNT(*) FROM members WHERE application_id = ? AND deleted_at IS NULL", kept.Application.String()).Scan(&activeMembers))
	assert.Equal(t, 1, activeMembers)
}

func whenDeleteDoesNotFindApplication(t *testing.T) {
	db := sqlitetest.New(t)
	repository := application_repositories.NewRepository(db)
	tenant := sqlitetest.Seed(t, db, pkgmocks.GenRandomApplicationName())

	assert.ErrorIs(t, repository.Delete(context.Background(), uuid.NewV7()), sql.ErrNoRows)

	assert.NoError(t, repository.Delete(context.Background(), tenant.Application))
	assert.ErrorIs(t, repository.Delete(context.Background(), tenant.Application), sql.ErrNoRows, "apagar de novo não é sucesso")
}
