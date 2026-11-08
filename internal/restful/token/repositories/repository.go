package repositories

import (
	"context"
	"database/sql"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// Builder

type TokenRepository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) TokenRepository {
	return TokenRepository{DB: db}
}

var (
	_ contracts.ITokenCreator      = TokenRepository{}
	_ contracts.ITokenFinder       = TokenRepository{}
	_ contracts.ITokenLister       = TokenRepository{}
	_ contracts.ITokenHashFinder   = TokenRepository{}
	_ contracts.ITokenUsageToucher = TokenRepository{}
	_ contracts.ITokenRevoker      = TokenRepository{}
)

// Functions

const tokenColumns = `t.id, t.application_id, t.member_id, m.uri_id, t.name, t.prefix, t.expires_at, t.last_used_at, t.revoked_at, t.created_at, m.user_id`

// Create grava o token com o hash (AccessToken.TokenHash); o token em claro
// nunca chega aqui. Quem criou é o ator do ctx.
func (r TokenRepository) Create(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error) {
	if err := entities.Require(token.ApplicationID, token.MemberID); err != nil {
		return entities.AccessToken{}, err
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, `
		INSERT INTO access_tokens (id, application_id, member_id, name, prefix, token_hash, expires_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		token.ID.String(), token.ApplicationID.String(), token.MemberID.String(), token.Name, token.Prefix, token.TokenHash,
		token.ExpiresAt, entities.ActorFrom(ctx).UserID.String(), token.CreatedAt)
	if err != nil {
		return entities.AccessToken{}, err
	}

	return token, nil
}

// FindById acha o token da aplicação: o id de um token de outra aplicação não
// acha nada. Devolve o token mesmo revogado.
func (r TokenRepository) FindById(ctx context.Context, application, id uuid.UUID) (entities.AccessToken, error) {
	if err := entities.Require(application, id); err != nil {
		return entities.AccessToken{}, err
	}

	return scanToken(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT "+tokenColumns+` FROM access_tokens t JOIN members m ON m.id = t.member_id
		WHERE t.application_id = ? AND t.id = ?`, application.String(), id.String()))
}

// List lista os tokens do membro na aplicação, inclusive os revogados (é o
// histórico).
func (r TokenRepository) List(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.AccessToken, int, error) {
	if err := entities.Require(application, member); err != nil {
		return nil, 0, err
	}

	const from = ` FROM access_tokens t JOIN members m ON m.id = t.member_id
			WHERE t.application_id = ? AND t.member_id = ?`
	args := []any{application.String(), member.String()}
	conn := sqlite.Conn(ctx, r.DB)

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.QueryContext(ctx, "SELECT "+tokenColumns+from+" ORDER BY t.id DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tokens := []entities.AccessToken{}
	for rows.Next() {
		token, err := scanToken(rows)
		if err != nil {
			return nil, 0, err
		}
		tokens = append(tokens, token)
	}

	return tokens, total, rows.Err()
}

// FindActiveByHash é a autenticação do SDK: devolve o token daquele hash SÓ se
// ele vale agora (não revogado, não vencido, de membro ativo e de aplicação
// ativa); qualquer outro caso devolve sql.ErrNoRows. Hash vazio é recusado.
func (r TokenRepository) FindActiveByHash(ctx context.Context, tokenHash string) (entities.AccessToken, error) {
	if tokenHash == "" {
		return entities.AccessToken{}, entities.ErrMissingParam
	}

	return scanToken(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx, "SELECT "+tokenColumns+` FROM access_tokens t
			JOIN members m ON m.id = t.member_id AND m.status = 'active' AND m.deleted_at IS NULL
			JOIN applications a ON a.id = t.application_id AND a.deleted_at IS NULL
			WHERE t.token_hash = ? AND t.revoked_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > ?)`,
		tokenHash, time.Now().UTC()))
}

// TouchLastUsed marca o último uso (AccessToken.LastUsedAt), no máximo uma vez
// por minuto por token, para a leitura do SDK não virar uma escrita a cada
// requisição. Chamar dentro do minuto é um sucesso que não muda nada.
func (r TokenRepository) TouchLastUsed(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error) {
	if err := entities.Require(token.ApplicationID, token.ID); err != nil {
		return entities.AccessToken{}, err
	}
	if token.LastUsedAt == nil {
		return entities.AccessToken{}, entities.ErrMissingParam
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE access_tokens SET last_used_at = ? WHERE application_id = ? AND id = ? AND (last_used_at IS NULL OR last_used_at < ?)",
		*token.LastUsedAt, token.ApplicationID.String(), token.ID.String(), token.LastUsedAt.Add(-time.Minute))
	if err != nil {
		return entities.AccessToken{}, err
	}

	return token, nil
}

// Revoke revoga o token (a linha fica, com revoked_at e quem revogou, o ator do
// ctx): vale na hora, porque a autenticação confere o banco a cada requisição.
// É idempotente: revogar de novo não muda nada.
func (r TokenRepository) Revoke(ctx context.Context, application, id uuid.UUID) error {
	if err := entities.Require(application, id); err != nil {
		return err
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE access_tokens SET revoked_at = ?, revoked_by = ? WHERE application_id = ? AND id = ? AND revoked_at IS NULL",
		time.Now().UTC(), entities.ActorFrom(ctx).UserID.String(), application.String(), id.String())
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanToken(row scanner) (entities.AccessToken, error) {
	var token entities.AccessToken

	err := row.Scan(&token.ID, &token.ApplicationID, &token.MemberID, &token.UriID, &token.Name, &token.Prefix,
		&token.ExpiresAt, &token.LastUsedAt, &token.RevokedAt, &token.CreatedAt, &token.UserID)
	return token, err
}
