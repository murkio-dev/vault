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

type ApplicationRepository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) ApplicationRepository {
	return ApplicationRepository{
		DB: db,
	}
}

var (
	_ contracts.IApplicationCreator    = ApplicationRepository{}
	_ contracts.IApplicationChecker    = ApplicationRepository{}
	_ contracts.IApplicationFinder     = ApplicationRepository{}
	_ contracts.IApplicationLister     = ApplicationRepository{}
	_ contracts.IApplicationUserLister = ApplicationRepository{}
	_ contracts.IApplicationUpdater    = ApplicationRepository{}
	_ contracts.IApplicationUpgrader   = ApplicationRepository{}
	_ contracts.IApplicationDeleter    = ApplicationRepository{}
)

// Functions

const applicationColumns = "id, name, created_by, updated_at, created_at"

func (r ApplicationRepository) Create(ctx context.Context, input entities.Application) (entities.Application, error) {

	query := "INSERT INTO applications (id, name, created_by, updated_at, created_at) VALUES (?, ?, ?, ?, ?)"
	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, query, input.ID.String(), input.Name, input.CreatedBy.String(), input.UpdatedAt, input.CreatedAt)
	if err != nil {
		return entities.Application{}, err
	}

	return input, nil
}

// Exist diz se já há uma aplicação ativa com o nome. A aplicação apagada
// libera o nome.
func (r ApplicationRepository) Exist(ctx context.Context, input entities.Application) (bool, error) {

	var output bool

	query := "SELECT EXISTS(SELECT 1 FROM applications WHERE name = ? AND deleted_at IS NULL)"
	err := sqlite.Conn(ctx, r.DB).QueryRowContext(ctx, query, input.Name).Scan(&output)
	if err != nil {
		return false, err
	}

	return output, nil
}

// FindById não pede tenant: a aplicação é o próprio tenant, e quem decide se a
// pessoa pode vê-la é o use case Authorize, antes.
func (r ApplicationRepository) FindById(ctx context.Context, input uuid.UUID) (entities.Application, error) {

	query := "SELECT " + applicationColumns + " FROM applications WHERE id = ? AND deleted_at IS NULL"

	return scanApplication(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx, query, input.String()))
}

// List devolve todas as aplicações ativas (só o platform admin chega aqui).
func (r ApplicationRepository) List(ctx context.Context, limit, offset int) ([]entities.Application, int, error) {
	return r.list(ctx, "FROM applications WHERE deleted_at IS NULL", nil, limit, offset)
}

// ListByUser devolve só as aplicações de que a conta é membro ativo.
func (r ApplicationRepository) ListByUser(ctx context.Context, user uuid.UUID, limit, offset int) ([]entities.Application, int, error) {
	if err := entities.Require(user); err != nil {
		return nil, 0, err
	}

	return r.list(ctx, `FROM applications WHERE deleted_at IS NULL AND id IN (SELECT application_id FROM members WHERE user_id = ? AND status = 'active' AND deleted_at IS NULL)`, []any{user.String()}, limit, offset)
}

func (r ApplicationRepository) list(ctx context.Context, from string, args []any, limit, offset int) ([]entities.Application, int, error) {

	conn := sqlite.Conn(ctx, r.DB)

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) "+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.QueryContext(ctx, "SELECT "+applicationColumns+" "+from+" ORDER BY id LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	output := []entities.Application{}
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, 0, err
		}
		output = append(output, app)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return output, total, nil
}

func (r ApplicationRepository) Update(ctx context.Context, input entities.Application) (entities.Application, error) {

	now := time.Now().UTC()
	query := "UPDATE applications SET name = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL"

	if err := r.change(ctx, query, input.Name, now, input.ID.String()); err != nil {
		return entities.Application{}, err
	}

	return r.FindById(ctx, input.ID)
}

// Upgrade é o PATCH: só muda o que veio. Hoje o único campo é o nome; vazio
// mantém o atual.
func (r ApplicationRepository) Upgrade(ctx context.Context, input entities.Application) (entities.Application, error) {

	now := time.Now().UTC()
	query := "UPDATE applications SET name = COALESCE(NULLIF(?, ''), name), updated_at = ? WHERE id = ? AND deleted_at IS NULL"

	if err := r.change(ctx, query, input.Name, now, input.ID.String()); err != nil {
		return entities.Application{}, err
	}

	return r.FindById(ctx, input.ID)
}

// Delete apaga a aplicação sem perder a trilha: marca a aplicação, os membros e
// os segredos como apagados e revoga os tokens. As versões cifradas e a
// auditoria ficam. Chame dentro de uma transação (IAuditedTx) para a cascata
// ser atômica.
func (r ApplicationRepository) Delete(ctx context.Context, input uuid.UUID) error {

	now := time.Now().UTC()
	id := input.String()

	if err := r.change(ctx, "UPDATE applications SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL", now, now, id); err != nil {
		return err
	}

	conn := sqlite.Conn(ctx, r.DB)
	cascade := []struct {
		query string
		args  []any
	}{
		{"UPDATE access_tokens SET revoked_at = ? WHERE application_id = ? AND revoked_at IS NULL", []any{now, id}},
		{"UPDATE secrets SET deleted_at = ? WHERE application_id = ? AND deleted_at IS NULL", []any{now, id}},
		{"UPDATE members SET deleted_at = ? WHERE application_id = ? AND deleted_at IS NULL", []any{now, id}},
		{"UPDATE invitations SET revoked_at = ? WHERE application_id = ? AND revoked_at IS NULL AND accepted_at IS NULL", []any{now, id}},
	}
	for _, step := range cascade {
		if _, err := conn.ExecContext(ctx, step.query, step.args...); err != nil {
			return err
		}
	}

	return nil
}

func (r ApplicationRepository) change(ctx context.Context, query string, args ...any) error {

	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanApplication(row scanner) (entities.Application, error) {
	var app entities.Application

	if err := row.Scan(&app.ID, &app.Name, &app.CreatedBy, &app.UpdatedAt, &app.CreatedAt); err != nil {
		return entities.Application{}, err
	}

	return app, nil
}
