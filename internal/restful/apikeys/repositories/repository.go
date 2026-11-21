package repositories

import (
	"context"
	"database/sql"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// Builder

type ApiKeyRepository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) ApiKeyRepository {
	return ApiKeyRepository{DB: db}
}

var _ apikeyscontracts.IApiKeyRepository = ApiKeyRepository{}

// Functions

const keyColumns = `k.id, k.application_id, k.member_id, m.uri_id, k.name, k.type, k.prefix, k.last_used_at, k.expires_at, k.updated_at, k.created_at, k.deleted_at, m.user_id`

// Create grava a chave com o hash (ApiKey.Hash); o valor em claro nunca chega
// aqui. Quem criou é o ator do ctx.
func (r ApiKeyRepository) Create(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error) {
	if err := entities.Require(key.ApplicationID, key.MemberID); err != nil {
		return entities.ApiKey{}, err
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, `
		INSERT INTO api_keys (id, application_id, member_id, name, type, prefix, hash, expires_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID.String(), key.ApplicationID.String(), key.MemberID.String(), key.Name, string(key.Type), key.Prefix, key.Hash,
		key.ExpiresAt, entities.ActorFrom(ctx).UserID.String(), key.CreatedAt)
	if err != nil {
		return entities.ApiKey{}, err
	}

	return key, nil
}

// FindById acha a chave da aplicação: o id de uma chave de outra aplicação não
// acha nada. Devolve a chave mesmo apagada.
func (r ApiKeyRepository) FindById(ctx context.Context, application, id uuid.UUID) (entities.ApiKey, error) {
	if err := entities.Require(application, id); err != nil {
		return entities.ApiKey{}, err
	}

	return scanKey(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT "+keyColumns+` FROM api_keys k JOIN members m ON m.id = k.member_id
		WHERE k.application_id = ? AND k.id = ?`, application.String(), id.String()))
}

// List lista as chaves do membro na aplicação, inclusive as apagadas (é o
// histórico).
func (r ApiKeyRepository) List(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.ApiKey, int, error) {
	if err := entities.Require(application, member); err != nil {
		return nil, 0, err
	}

	const from = ` FROM api_keys k JOIN members m ON m.id = k.member_id
			WHERE k.application_id = ? AND k.member_id = ?`
	args := []any{application.String(), member.String()}
	conn := sqlite.Conn(ctx, r.DB)

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.QueryContext(ctx, "SELECT "+keyColumns+from+" ORDER BY k.id DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	keys := []entities.ApiKey{}
	for rows.Next() {
		key, err := scanKey(rows)
		if err != nil {
			return nil, 0, err
		}
		keys = append(keys, key)
	}

	return keys, total, rows.Err()
}

// FindActiveByHash é a autenticação do SDK: devolve a chave daquele hash SÓ se
// ela vale agora (secreta, não apagada, não vencida, de membro ativo e de
// aplicação ativa); qualquer outro caso devolve sql.ErrNoRows. A pk_vt_ nunca
// autentica. Hash vazio é recusado.
func (r ApiKeyRepository) FindActiveByHash(ctx context.Context, hash string) (entities.ApiKey, error) {
	if hash == "" {
		return entities.ApiKey{}, entities.ErrMissingParam
	}

	return scanKey(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx, "SELECT "+keyColumns+` FROM api_keys k
			JOIN members m ON m.id = k.member_id AND m.status = 'active' AND m.deleted_at IS NULL
			JOIN applications a ON a.id = k.application_id AND a.deleted_at IS NULL
			WHERE k.hash = ? AND k.type = 'secret' AND k.deleted_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > ?)`,
		hash, time.Now().UTC()))
}

// Rename grava o nome (ApiKey.Name) de uma chave não apagada; a apagada devolve
// sql.ErrNoRows.
func (r ApiKeyRepository) Rename(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error) {
	if err := entities.Require(key.ApplicationID, key.ID); err != nil {
		return entities.ApiKey{}, err
	}

	now := time.Now().UTC()
	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE api_keys SET name = ?, updated_at = ? WHERE application_id = ? AND id = ? AND deleted_at IS NULL",
		key.Name, now, key.ApplicationID.String(), key.ID.String())
	if err != nil {
		return entities.ApiKey{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected == 0 {
		return entities.ApiKey{}, sql.ErrNoRows
	}

	key.UpdatedAt = &now
	return key, nil
}

// Expire antecipa o vencimento da chave para at. Não mexe em chave apagada.
func (r ApiKeyRepository) Expire(ctx context.Context, application, id uuid.UUID, at time.Time) error {
	if err := entities.Require(application, id); err != nil {
		return err
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE api_keys SET expires_at = ?, updated_at = ? WHERE application_id = ? AND id = ? AND deleted_at IS NULL",
		at.UTC(), time.Now().UTC(), application.String(), id.String())
	return err
}

// TouchLastUsed marca o último uso (ApiKey.LastUsedAt), no máximo uma vez por
// minuto por chave, para a leitura do SDK não virar uma escrita a cada
// requisição. Chamar dentro do minuto é um sucesso que não muda nada.
func (r ApiKeyRepository) TouchLastUsed(ctx context.Context, key entities.ApiKey) (entities.ApiKey, error) {
	if err := entities.Require(key.ApplicationID, key.ID); err != nil {
		return entities.ApiKey{}, err
	}
	if key.LastUsedAt == nil {
		return entities.ApiKey{}, entities.ErrMissingParam
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE api_keys SET last_used_at = ? WHERE application_id = ? AND id = ? AND (last_used_at IS NULL OR last_used_at < ?)",
		*key.LastUsedAt, key.ApplicationID.String(), key.ID.String(), key.LastUsedAt.Add(-time.Minute))
	if err != nil {
		return entities.ApiKey{}, err
	}

	return key, nil
}

// Delete revoga a chave (a linha fica, com deleted_at e quem apagou, o ator do
// ctx): vale na hora, porque a autenticação confere o banco a cada requisição.
// É idempotente: apagar de novo não muda nada.
func (r ApiKeyRepository) Delete(ctx context.Context, application, id uuid.UUID) error {
	if err := entities.Require(application, id); err != nil {
		return err
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE api_keys SET deleted_at = ?, deleted_by = ? WHERE application_id = ? AND id = ? AND deleted_at IS NULL",
		time.Now().UTC(), entities.ActorFrom(ctx).UserID.String(), application.String(), id.String())
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanKey(row scanner) (entities.ApiKey, error) {
	var key entities.ApiKey

	err := row.Scan(&key.ID, &key.ApplicationID, &key.MemberID, &key.UriID, &key.Name, &key.Type, &key.Prefix,
		&key.LastUsedAt, &key.ExpiresAt, &key.UpdatedAt, &key.CreatedAt, &key.DeletedAt, &key.UserID)
	return key, err
}
