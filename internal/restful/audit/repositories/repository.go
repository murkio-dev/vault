package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// Builder

type AuditRepository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) AuditRepository {
	return AuditRepository{DB: db}
}

var (
	_ contracts.IAuditLogCreator = AuditRepository{}
	_ contracts.IAuditLogLister  = AuditRepository{}
	_ contracts.IAuditLogPurger  = AuditRepository{}
)

// Functions

// Create insere uma linha na trilha. Dentro de uma transação (IAuditedTx), a
// linha entra nela. A tabela só aceita inserção: o banco recusa UPDATE e
// DELETE, e este repositório não tem outra escrita. Não exige aplicação, porque
// também grava os acessos negados antes de saber a aplicação.
func (r AuditRepository) Create(ctx context.Context, log entities.AuditLog) (entities.AuditLog, error) {

	const query = `
		INSERT INTO audit_logs (id, application_id, actor_type, actor_id, token_id, action, outcome,
			target_type, target_id, ip, user_agent, request_id, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, query,
		log.ID.String(), nullableUUID(log.ApplicationID), log.ActorType, nullableUUID(log.ActorID), nullableUUID(log.TokenID),
		log.Action, string(log.Outcome), nullableText(log.TargetType), nullableText(log.TargetID),
		nullableText(log.IP), nullableText(log.UserAgent), nullableText(log.RequestID),
		string(log.Metadata), log.CreatedAt)
	if err != nil {
		return entities.AuditLog{}, err
	}

	return log, nil
}

// List lê a trilha do mais novo para o mais antigo, com cursor: o UUIDv7 é
// ordenado no tempo, então "id menor que o cursor" é a página seguinte, sem
// OFFSET (que ficaria mais lento e instável quanto mais a trilha crescesse). O
// pagina pelo filter.Cursor, sem offset. Exige filter.ApplicationID; não conta a
// trilha inteira a cada página, porque custaria cada vez mais.
func (r AuditRepository) List(ctx context.Context, filter entities.AuditFilter, limit int) ([]entities.AuditLog, error) {
	if err := entities.Require(filter.ApplicationID); err != nil {
		return nil, err
	}
	application := filter.ApplicationID

	query := `SELECT id, application_id, actor_type, actor_id, token_id, action, outcome, target_type, target_id,
		ip, user_agent, request_id, metadata, created_at FROM audit_logs WHERE application_id = ?`
	args := []any{application.String()}

	add := func(clause string, value any) {
		query += " AND " + clause
		args = append(args, value)
	}
	if filter.ActorID != nil {
		add("actor_id = ?", filter.ActorID.String())
	}
	if filter.Action != "" {
		add("action = ?", filter.Action)
	}
	if filter.TargetID != "" {
		add("target_id = ?", filter.TargetID)
	}
	if filter.Outcome != "" {
		add("outcome = ?", string(filter.Outcome))
	}
	if filter.From != nil {
		add("created_at >= ?", filter.From.UTC())
	}
	if filter.To != nil {
		add("created_at <= ?", filter.To.UTC())
	}
	if filter.Cursor != (uuid.UUID{}) {
		add("id < ?", filter.Cursor.String())
	}

	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := sqlite.Conn(ctx, r.DB).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []entities.AuditLog{}
	for rows.Next() {
		log, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

func scanLog(row interface{ Scan(dest ...any) error }) (entities.AuditLog, error) {
	var (
		log                                            entities.AuditLog
		application, actor, token                      sql.NullString
		targetType, targetID, ip, userAgent, requestID sql.NullString
		metadata                                       string
	)

	err := row.Scan(&log.ID, &application, &log.ActorType, &actor, &token, &log.Action, &log.Outcome, &targetType, &targetID,
		&ip, &userAgent, &requestID, &metadata, &log.CreatedAt)
	if err != nil {
		return entities.AuditLog{}, err
	}

	log.ApplicationID = parseUUID(application)
	log.ActorID = parseUUID(actor)
	log.TokenID = parseUUID(token)
	log.TargetType, log.TargetID = targetType.String, targetID.String
	log.IP, log.UserAgent, log.RequestID = ip.String, userAgent.String, requestID.String
	log.Metadata = json.RawMessage(metadata)

	return log, nil
}

func parseUUID(value sql.NullString) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id, err := uuid.Parse(value.String)
	if err != nil {
		return nil
	}
	return &id
}

// MaxPurgeBatch é o teto de linhas por chamada de PurgeExpired: lotes pequenos
// seguram o lock de escrita do SQLite por pouco tempo, e as gravações da API
// seguem entre um lote e outro.
const MaxPurgeBatch = 5000

// PurgeExpired apaga até limit linhas da trilha anteriores a now menos 1 ano
// menos 1 hora, das mais antigas para as mais novas. A hora de folga existe
// porque o trigger do banco compara por segundo: o corte nunca cai no limite
// dele. Dentro de uma retenção, o trigger recusa o DELETE de qualquer jeito, e
// o erro volta sem apagar nada. limit fora de 1..MaxPurgeBatch devolve erro, e
// um lote que volta com menos linhas que limit diz que o expurgo terminou.
// Exemplo: com 3 linhas de 400 dias e 2 de 10 dias, PurgeExpired(now, 2)
// apaga 2, a chamada seguinte apaga 1 e as de 10 dias ficam.
func (r AuditRepository) PurgeExpired(ctx context.Context, now time.Time, limit int) (int64, error) {

	if limit < 1 || limit > MaxPurgeBatch {
		return 0, fmt.Errorf("audit: purge limit must be between 1 and %d", MaxPurgeBatch)
	}

	cutoff := now.UTC().AddDate(-entities.AuditRetentionYears, 0, 0).Add(-entities.AuditRetentionMargin)

	const query = `
		DELETE FROM audit_logs WHERE id IN (
			SELECT id FROM audit_logs WHERE created_at < ? ORDER BY created_at LIMIT ?
		)`

	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, query, cutoff, limit)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
