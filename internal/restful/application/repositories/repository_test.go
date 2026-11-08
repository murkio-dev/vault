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
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/murkio-dev/vault/pkg/utils"
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

	application, err := entities.NewApplication(utils.GenRandomApplicationName(), uuid.New())
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

	application, err := entities.NewApplication(utils.GenRandomApplicationName(), uuid.New())
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

	application, err := entities.NewApplication(utils.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	savedApplication, err := repository.Create(ctx, application)

	assert.Error(t, err)
	assert.Zero(t, savedApplication)
}

func whenCreateFailsOnDuplicateName(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)
	ctx := context.Background()

	sameName := utils.GenRandomApplicationName()

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
	application, err := entities.NewApplication(utils.GenRandomApplicationName(), userID)
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

	application, err := entities.NewApplication(utils.GenRandomApplicationName(), uuid.New())
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

	application, err := entities.NewApplication(utils.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	exists, err := repository.Exist(ctx, application)

	assert.NoError(t, err)
	assert.False(t, exists)
}

func whenExistFailsDueToContext(t *testing.T, db *sql.DB) {
	repository := application_repositories.NewRepository(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	application, err := entities.NewApplication(utils.GenRandomApplicationName(), uuid.New())
	assert.NoError(t, err)

	exists, err := repository.Exist(ctx, application)

	assert.Error(t, err)
	assert.False(t, exists)
}
