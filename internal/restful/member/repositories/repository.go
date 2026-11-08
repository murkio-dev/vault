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

type MemberRepository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) MemberRepository {
	return MemberRepository{DB: db}
}

var (
	_ contracts.IMemberCreator      = MemberRepository{}
	_ contracts.IMemberFinder       = MemberRepository{}
	_ contracts.IMemberUserFinder   = MemberRepository{}
	_ contracts.IMemberLister       = MemberRepository{}
	_ contracts.IMemberAdminCounter = MemberRepository{}
	_ contracts.IMemberUpgrader     = MemberRepository{}
	_ contracts.IMemberDeleter      = MemberRepository{}
)

// Functions

const memberColumns = "id, application_id, user_id, uri_id, role, status, invited_by, created_at, updated_at"

// Create grava o membro na aplicação dele (Member.ApplicationID).
func (r MemberRepository) Create(ctx context.Context, member entities.Member) (entities.Member, error) {
	if err := entities.Require(member.ApplicationID); err != nil {
		return entities.Member{}, err
	}

	const query = `
		INSERT INTO members (id, application_id, user_id, uri_id, role, status, invited_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var invitedBy any
	if member.InvitedBy != nil {
		invitedBy = member.InvitedBy.String()
	}

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, query,
		member.ID.String(), member.ApplicationID.String(), member.UserID.String(), member.UriID,
		string(member.Role), string(member.Status), invitedBy, member.CreatedAt, member.UpdatedAt)
	if err != nil {
		return entities.Member{}, err
	}

	return member, nil
}

// FindById acha o membro da aplicação: o id de um membro de outra aplicação não
// acha nada.
func (r MemberRepository) FindById(ctx context.Context, application, id uuid.UUID) (entities.Member, error) {
	if err := entities.Require(application, id); err != nil {
		return entities.Member{}, err
	}

	return scanMember(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT "+memberColumns+" FROM members WHERE application_id = ? AND id = ? AND deleted_at IS NULL",
		application.String(), id.String()))
}

// FindByUser acha o vínculo da conta com a aplicação (é como o Authorize carrega
// o membro de uma pessoa). Sem vínculo: sql.ErrNoRows.
func (r MemberRepository) FindByUser(ctx context.Context, application, user uuid.UUID) (entities.Member, error) {
	if err := entities.Require(application, user); err != nil {
		return entities.Member{}, err
	}

	return scanMember(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT "+memberColumns+" FROM members WHERE application_id = ? AND user_id = ? AND deleted_at IS NULL ORDER BY id LIMIT 1",
		application.String(), user.String()))
}

// List lista os membros da aplicação.
func (r MemberRepository) List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Member, int, error) {
	if err := entities.Require(application); err != nil {
		return nil, 0, err
	}

	const where = "application_id = ? AND deleted_at IS NULL"
	conn := sqlite.Conn(ctx, r.DB)

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM members WHERE "+where, application.String()).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.QueryContext(ctx, "SELECT "+memberColumns+" FROM members WHERE "+where+" ORDER BY id LIMIT ? OFFSET ?", application.String(), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	members := []entities.Member{}
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, 0, err
		}
		members = append(members, member)
	}

	return members, total, rows.Err()
}

// CountActiveAdmins conta os admins ativos da aplicação.
func (r MemberRepository) CountActiveAdmins(ctx context.Context, application uuid.UUID) (int, error) {
	if err := entities.Require(application); err != nil {
		return 0, err
	}

	var total int
	err := sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM members WHERE application_id = ? AND role = ? AND status = ? AND deleted_at IS NULL",
		application.String(), string(entities.RoleAdmin), string(entities.MemberActive)).Scan(&total)
	return total, err
}

// Upgrade muda só o que veio: papel ou status vazios mantêm o atual.
func (r MemberRepository) Upgrade(ctx context.Context, member entities.Member) (entities.Member, error) {
	if err := entities.Require(member.ApplicationID, member.ID); err != nil {
		return entities.Member{}, err
	}

	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE members SET role = COALESCE(NULLIF(?, ''), role), status = COALESCE(NULLIF(?, ''), status), updated_at = ? WHERE application_id = ? AND id = ? AND deleted_at IS NULL",
		string(member.Role), string(member.Status), time.Now().UTC(), member.ApplicationID.String(), member.ID.String())
	if err := mustAffect(result, err); err != nil {
		return entities.Member{}, err
	}

	return r.FindById(ctx, member.ApplicationID, member.ID)
}

// Delete remove o membro e o que é dele numa conta só: os tokens são revogados
// (o SDK dele para de funcionar) e os segredos são apagados. As versões
// cifradas e a auditoria ficam. Chame dentro de uma transação.
func (r MemberRepository) Delete(ctx context.Context, application, id uuid.UUID) error {
	if err := entities.Require(application, id); err != nil {
		return err
	}

	conn := sqlite.Conn(ctx, r.DB)
	now := time.Now().UTC()
	member := id.String()

	result, err := conn.ExecContext(ctx, "UPDATE members SET deleted_at = ?, updated_at = ? WHERE application_id = ? AND id = ? AND deleted_at IS NULL", now, now, application.String(), member)
	if err := mustAffect(result, err); err != nil {
		return err
	}

	if _, err := conn.ExecContext(ctx, "UPDATE access_tokens SET revoked_at = ? WHERE member_id = ? AND revoked_at IS NULL", now, member); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "UPDATE secrets SET deleted_at = ? WHERE member_id = ? AND deleted_at IS NULL", now, member)
	return err
}

func mustAffect(result sql.Result, err error) error {
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

func scanMember(row scanner) (entities.Member, error) {
	var (
		member    entities.Member
		invitedBy sql.NullString
	)

	err := row.Scan(&member.ID, &member.ApplicationID, &member.UserID, &member.UriID, &member.Role, &member.Status,
		&invitedBy, &member.CreatedAt, &member.UpdatedAt)
	if err != nil {
		return entities.Member{}, err
	}

	if invitedBy.Valid {
		if id, err := uuid.Parse(invitedBy.String); err == nil {
			member.InvitedBy = &id
		}
	}

	return member, nil
}
