package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/mocks"
	"github.com/murkio-dev/vault/internal/restful/apikeys/repositories"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestApiKeyRepository(t *testing.T) {
	t.Run("Quando falta um id de isolamento.", func(t *testing.T) { whenIsolationIDIsMissing(t) })
	t.Run("Quando outro tenant pede os tokens.", func(t *testing.T) { whenAnotherTenantAsks(t) })
	t.Run("Quando cria o token.", func(t *testing.T) { whenCreatingAToken(t) })
	t.Run("Quando lista os tokens do membro.", func(t *testing.T) { whenListingAMembersTokens(t) })
	t.Run("Quando autentica pelo hash.", func(t *testing.T) { whenAuthenticatingByHash(t) })
	t.Run("Quando o token deixa de valer.", func(t *testing.T) { whenTheTokenStopsBeingValid(t) })
	t.Run("Quando a chave é publishable.", func(t *testing.T) { whenTheKeyIsPublishable(t) })
	t.Run("Quando revoga.", func(t *testing.T) { whenRevoking(t) })
	t.Run("Quando renomeia.", func(t *testing.T) { whenRenaming(t) })
	t.Run("Quando antecipa o vencimento.", func(t *testing.T) { whenExpiring(t) })
	t.Run("Quando marca o último uso.", func(t *testing.T) { whenMarkingTheLastUse(t) })
	t.Run("Quando o banco falha.", func(t *testing.T) { whenDatabaseFails(t) })
}

// Functions

func member(t *testing.T, tenant sqlitetest.Tenant) entities.Member {
	t.Helper()
	return entities.Member{ID: tenant.Member, ApplicationID: tenant.Application, UserID: tenant.User, UriID: tenant.Member.String(), Role: entities.RoleAdmin, Status: entities.MemberActive}
}

func add(t *testing.T, repo repositories.ApiKeyRepository, tenant sqlitetest.Tenant) (entities.ApiKey, string) {
	t.Helper()
	return addTyped(t, repo, tenant, entities.KeySecret)
}

func addTyped(t *testing.T, repo repositories.ApiKeyRepository, tenant sqlitetest.Tenant, keyType entities.KeyType) (entities.ApiKey, string) {
	t.Helper()
	token, raw, err := entities.NewApiKey(member(t, tenant), "ci", keyType, nil)
	require.NoError(t, err)
	created, err := repo.Create(entities.WithActor(context.Background(), entities.Actor{Kind: entities.ActorUser, UserID: tenant.User}), token)
	require.NoError(t, err)
	return created, raw
}

func whenIsolationIDIsMissing(t *testing.T) {
	repo := repositories.NewRepository(sqlitetest.New(t))
	ctx := context.Background()
	zero := uuid.UUID{}
	token := entities.ApiKey{ID: uuid.NewV7(), ApplicationID: uuid.NewV7(), MemberID: uuid.NewV7()}
	now := time.Now().UTC()
	token.LastUsedAt = &now
	orphan := token
	orphan.ApplicationID = zero

	_, createErr := repo.Create(ctx, orphan)
	_, findErr := repo.FindById(ctx, zero, token.ID)
	_, _, listErr := repo.List(ctx, zero, token.MemberID, 10, 0)
	_, _, noMemberErr := repo.List(ctx, token.ApplicationID, zero, 10, 0)
	_, hashErr := repo.FindActiveByHash(ctx, "")
	_, touchErr := repo.TouchLastUsed(ctx, orphan)
	revokeErr := repo.Delete(ctx, zero, token.ID)
	_, renameErr := repo.Rename(ctx, orphan)
	expireErr := repo.Expire(ctx, zero, token.ID, now)

	for name, err := range map[string]error{"Create": createErr, "FindById": findErr, "List": listErr, "List sem membro": noMemberErr, "FindActiveByHash": hashErr, "TouchLastUsed": touchErr, "Delete": revokeErr, "Rename": renameErr, "Expire": expireErr} {
		assert.ErrorIs(t, err, entities.ErrMissingParam, name)
	}
}

func whenAnotherTenantAsks(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a, b := sqlitetest.Seed(t, db, "app-a"), sqlitetest.Seed(t, db, "app-b")
	token, _ := add(t, repo, a)

	_, findErr := repo.FindById(context.Background(), b.Application, token.ID)
	list, total, listErr := repo.List(context.Background(), b.Application, a.Member, 10, 0)
	deleteErr := repo.Delete(context.Background(), b.Application, token.ID)
	stored, err := repo.FindById(context.Background(), a.Application, token.ID)

	assert.ErrorIs(t, findErr, sql.ErrNoRows, "o id de um token de outra aplicação não acha nada")
	require.NoError(t, listErr)
	assert.Zero(t, total)
	assert.Empty(t, list, "nem listando o membro de outra aplicação")
	assert.NoError(t, deleteErr, "revogar é idempotente, mas não toca no token alheio")
	require.NoError(t, err)
	assert.Nil(t, stored.DeletedAt, "o token de A continua válido")
}

func whenCreatingAToken(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	b := sqlitetest.Seed(t, db, "app-b")

	token, raw := add(t, repo, a)
	foreign, _, _ := entities.NewApiKey(member(t, b), "x", entities.KeySecret, nil)
	foreign.ApplicationID = uuid.UUID{}
	_, missing := repo.Create(context.Background(), foreign)
	var hash, createdBy, keyType string
	require.NoError(t, db.QueryRow(`SELECT hash, created_by, type FROM api_keys WHERE id = ?`, token.ID.String()).Scan(&hash, &createdBy, &keyType))

	assert.Equal(t, entities.HashToken(raw), hash, "o banco guarda o hash, nunca o token")
	assert.NotContains(t, hash, raw)
	assert.Equal(t, a.User.String(), createdBy, "quem criou é o ator do ctx")
	assert.Equal(t, "secret", keyType)
	assert.ErrorIs(t, missing, entities.ErrMissingParam)
}

func whenListingAMembersTokens(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	first, _ := add(t, repo, a)
	add(t, repo, a)
	require.NoError(t, repo.Delete(context.Background(), a.Application, first.ID))

	list, total, err := repo.List(context.Background(), a.Application, a.Member, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, 2, total, "o histórico mostra o revogado")
	assert.Len(t, list, 2)
	assert.Equal(t, a.Member.String(), list[0].UriID, "traz o uri_id do membro")
	assert.Empty(t, list[0].Hash, "o hash não volta do banco")
}

func whenAuthenticatingByHash(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	token, raw := add(t, repo, a)

	found, err := repo.FindActiveByHash(context.Background(), entities.HashToken(raw))
	_, noneErr := repo.FindActiveByHash(context.Background(), entities.HashToken("sk_vt_inventado"))

	require.NoError(t, err)
	assert.Equal(t, token.ID, found.ID)
	assert.Equal(t, a.User, found.UserID, "traz a conta dona, para o ator")
	assert.Equal(t, a.Application, found.ApplicationID)
	assert.ErrorIs(t, noneErr, sql.ErrNoRows)
}

func whenTheTokenStopsBeingValid(t *testing.T) {
	cases := map[string]func(*testing.T, *sqlDB, sqlitetest.Tenant, entities.ApiKey){
		"revogado": func(t *testing.T, db *sqlDB, _ sqlitetest.Tenant, token entities.ApiKey) {
			db.exec(t, `UPDATE api_keys SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, token.ID.String())
		},
		"vencido": func(t *testing.T, db *sqlDB, _ sqlitetest.Tenant, token entities.ApiKey) {
			db.exec(t, `UPDATE api_keys SET expires_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Second), token.ID.String())
		},
		"membro suspenso": func(t *testing.T, db *sqlDB, tenant sqlitetest.Tenant, _ entities.ApiKey) {
			db.exec(t, `UPDATE members SET status = 'suspended' WHERE id = ?`, tenant.Member.String())
		},
		"membro removido": func(t *testing.T, db *sqlDB, tenant sqlitetest.Tenant, _ entities.ApiKey) {
			db.exec(t, `UPDATE members SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, tenant.Member.String())
		},
		"aplicação apagada": func(t *testing.T, db *sqlDB, tenant sqlitetest.Tenant, _ entities.ApiKey) {
			db.exec(t, `UPDATE applications SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, tenant.Application.String())
		},
	}
	for name, invalidate := range cases {
		db := sqlitetest.New(t)
		repo := repositories.NewRepository(db)
		a := sqlitetest.Seed(t, db, "app-a")
		token, raw := add(t, repo, a)
		invalidate(t, &sqlDB{db}, a, token)

		_, err := repo.FindActiveByHash(context.Background(), entities.HashToken(raw))

		assert.ErrorIs(t, err, sql.ErrNoRows, name+": vale na hora")
	}
}

func whenRevoking(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	token, raw := add(t, repo, a)
	admin := uuid.NewV7()

	first := repo.Delete(entities.WithActor(context.Background(), entities.Actor{Kind: entities.ActorUser, UserID: admin}), a.Application, token.ID)
	second := repo.Delete(entities.WithActor(context.Background(), entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7()}), a.Application, token.ID)
	var revokedBy string
	require.NoError(t, db.QueryRow(`SELECT deleted_by FROM api_keys WHERE id = ?`, token.ID.String()).Scan(&revokedBy))
	_, hashErr := repo.FindActiveByHash(context.Background(), entities.HashToken(raw))

	assert.NoError(t, first)
	assert.NoError(t, second, "revogar de novo é idempotente")
	assert.Equal(t, admin.String(), revokedBy, "quem revogou primeiro fica registrado, a segunda chamada não sobrescreve")
	assert.ErrorIs(t, hashErr, sql.ErrNoRows)
}

func whenMarkingTheLastUse(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	token, _ := add(t, repo, a)
	first := time.Now().UTC().Truncate(time.Second)
	soon := first.Add(10 * time.Second)
	later := first.Add(2 * time.Minute)

	_, err := repo.TouchLastUsed(context.Background(), entities.ApiKey{ID: token.ID, ApplicationID: a.Application, LastUsedAt: &first})
	require.NoError(t, err)
	_, err = repo.TouchLastUsed(context.Background(), entities.ApiKey{ID: token.ID, ApplicationID: a.Application, LastUsedAt: &soon})
	require.NoError(t, err)
	after1, _ := repo.FindById(context.Background(), a.Application, token.ID)
	_, err = repo.TouchLastUsed(context.Background(), entities.ApiKey{ID: token.ID, ApplicationID: a.Application, LastUsedAt: &later})
	require.NoError(t, err)
	after2, _ := repo.FindById(context.Background(), a.Application, token.ID)
	_, incomplete := repo.TouchLastUsed(context.Background(), entities.ApiKey{ID: token.ID, ApplicationID: a.Application})

	require.NotNil(t, after1.LastUsedAt)
	assert.True(t, after1.LastUsedAt.Equal(first), "dentro de um minuto, não vira outra escrita")
	require.NotNil(t, after2.LastUsedAt)
	assert.True(t, after2.LastUsedAt.Equal(later), "depois de um minuto, atualiza")
	assert.ErrorIs(t, incomplete, entities.ErrMissingParam, "sem a hora do uso não há o que marcar")
}

func whenTheKeyIsPublishable(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	key, raw := addTyped(t, repo, a, entities.KeyPublishable)

	_, err := repo.FindActiveByHash(context.Background(), entities.HashToken(raw))
	stored, findErr := repo.FindById(context.Background(), a.Application, key.ID)

	assert.ErrorIs(t, err, sql.ErrNoRows, "a pk_vt_ nunca autentica")
	require.NoError(t, findErr)
	assert.Equal(t, entities.KeyPublishable, stored.Type)
	assert.Equal(t, entities.PublicKeyPrefix, stored.Prefix[:len(entities.PublicKeyPrefix)])
}

func whenRenaming(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	key, _ := add(t, repo, a)
	key.Name = "deploy"

	renamed, err := repo.Rename(context.Background(), key)
	stored, _ := repo.FindById(context.Background(), a.Application, key.ID)
	require.NoError(t, repo.Delete(context.Background(), a.Application, key.ID))
	_, deletedErr := repo.Rename(context.Background(), key)

	require.NoError(t, err)
	assert.NotNil(t, renamed.UpdatedAt)
	assert.Equal(t, "deploy", stored.Name)
	assert.NotNil(t, stored.UpdatedAt)
	assert.ErrorIs(t, deletedErr, sql.ErrNoRows, "chave apagada não muda")
}

func whenExpiring(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	a := sqlitetest.Seed(t, db, "app-a")
	key, raw := add(t, repo, a)

	require.NoError(t, repo.Expire(context.Background(), a.Application, key.ID, time.Now().Add(time.Hour)))
	_, stillValid := repo.FindActiveByHash(context.Background(), entities.HashToken(raw))
	require.NoError(t, repo.Expire(context.Background(), a.Application, key.ID, time.Now().Add(-time.Second)))
	_, expired := repo.FindActiveByHash(context.Background(), entities.HashToken(raw))

	assert.NoError(t, stillValid, "vale até o novo vencimento")
	assert.ErrorIs(t, expired, sql.ErrNoRows, "depois dele, não vale mais")
}

// Helpers

type sqlDB struct{ *sql.DB }

func (d *sqlDB) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := d.Exec(query, args...)
	require.NoError(t, err)
}

func whenDatabaseFails(t *testing.T) {
	db := sqlitetest.New(t)
	repo := repositories.NewRepository(db)
	app, id := uuid.NewV7(), uuid.NewV7()
	token := mocks.NewApiKey()
	require.NoError(t, db.Close())
	ctx := context.Background()

	_, create := repo.Create(ctx, token)
	_, find := repo.FindById(ctx, app, id)
	_, _, list := repo.List(ctx, app, id, 10, 0)
	_, byHash := repo.FindActiveByHash(ctx, "hash")
	_, touch := repo.TouchLastUsed(ctx, token)
	revoke := repo.Delete(ctx, app, id)
	_, rename := repo.Rename(ctx, token)
	expire := repo.Expire(ctx, app, id, time.Now())

	for name, err := range map[string]error{"Create": create, "FindById": find, "List": list, "FindActiveByHash": byHash, "TouchLastUsed": touch, "Delete": revoke, "Rename": rename, "Expire": expire} {
		assert.Error(t, err, name)
	}
}
