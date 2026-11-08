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

type InvitationRepository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) InvitationRepository {
	return InvitationRepository{DB: db}
}

var (
	_ contracts.IInvitationCreator     = InvitationRepository{}
	_ contracts.IInvitationLister      = InvitationRepository{}
	_ contracts.IInvitationTokenFinder = InvitationRepository{}
	_ contracts.IInvitationAcceptor    = InvitationRepository{}
	_ contracts.IInvitationRevoker     = InvitationRepository{}
)

// Functions

const invitationColumns = "id, application_id, email, role, expires_at, created_by, accepted_by, accepted_at, revoked_at, created_at"

// Create grava o convite com o hash do token (Invitation.TokenHash), na
// aplicação dele (Invitation.ApplicationID).
func (r InvitationRepository) Create(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error) {
	if err := entities.Require(invitation.ApplicationID); err != nil {
		return entities.Invitation{}, err
	}

	const query = `
		INSERT INTO invitations (id, application_id, email, role, token_hash, expires_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, query, invitation.ID.String(), invitation.ApplicationID.String(),
		invitation.Email, string(invitation.Role), invitation.TokenHash, invitation.ExpiresAt, invitation.CreatedBy.String(), invitation.CreatedAt)
	if err != nil {
		return entities.Invitation{}, err
	}

	return invitation, nil
}

// FindByTokenHash devolve o convite daquele token, em qualquer estado (quem
// aceita ainda não sabe a aplicação: o token é a credencial). Hash vazio é
// recusado.
func (r InvitationRepository) FindByTokenHash(ctx context.Context, tokenHash string) (entities.Invitation, error) {
	if tokenHash == "" {
		return entities.Invitation{}, entities.ErrMissingParam
	}

	return scanInvitation(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT "+invitationColumns+" FROM invitations WHERE token_hash = ?", tokenHash))
}

// List lista os convites da aplicação, do mais novo para o mais antigo.
func (r InvitationRepository) List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Invitation, int, error) {
	if err := entities.Require(application); err != nil {
		return nil, 0, err
	}

	const where = "application_id = ?"
	conn := sqlite.Conn(ctx, r.DB)

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM invitations WHERE "+where, application.String()).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.QueryContext(ctx, "SELECT "+invitationColumns+" FROM invitations WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", application.String(), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	invitations := []entities.Invitation{}
	for rows.Next() {
		invitation, err := scanInvitation(rows)
		if err != nil {
			return nil, 0, err
		}
		invitations = append(invitations, invitation)
	}

	return invitations, total, rows.Err()
}

// Accept marca o convite como aceito por Invitation.AcceptedBy, na hora
// Invitation.AcceptedAt, mas SÓ se ele ainda estiver pendente (não aceito, não
// revogado, não vencido). O UPDATE condicional é o que faz dois aceites
// simultâneos do mesmo link terminarem com um só vencedor; o outro recebe
// sql.ErrNoRows.
func (r InvitationRepository) Accept(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error) {
	if err := entities.Require(invitation.ApplicationID, invitation.ID); err != nil {
		return entities.Invitation{}, err
	}
	if invitation.AcceptedAt == nil || invitation.AcceptedBy == nil {
		return entities.Invitation{}, entities.ErrMissingParam
	}

	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE invitations SET accepted_at = ?, accepted_by = ? WHERE application_id = ? AND id = ? AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > ?",
		*invitation.AcceptedAt, invitation.AcceptedBy.String(), invitation.ApplicationID.String(), invitation.ID.String(), *invitation.AcceptedAt)
	if err := mustAffect(result, err); err != nil {
		return entities.Invitation{}, err
	}

	return invitation, nil
}

// Revoke revoga um convite pendente (a linha fica, com revoked_at). Convite que
// não existe ou que já se encerrou devolve sql.ErrNoRows.
func (r InvitationRepository) Revoke(ctx context.Context, application, id uuid.UUID) error {
	if err := entities.Require(application, id); err != nil {
		return err
	}

	return mustAffect(sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE invitations SET revoked_at = ? WHERE application_id = ? AND id = ? AND accepted_at IS NULL AND revoked_at IS NULL",
		time.Now().UTC(), application.String(), id.String()))
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

func scanInvitation(row scanner) (entities.Invitation, error) {
	var (
		invitation entities.Invitation
		acceptedBy sql.NullString
	)

	err := row.Scan(&invitation.ID, &invitation.ApplicationID, &invitation.Email, &invitation.Role, &invitation.ExpiresAt,
		&invitation.CreatedBy, &acceptedBy, &invitation.AcceptedAt, &invitation.RevokedAt, &invitation.CreatedAt)
	if err != nil {
		return entities.Invitation{}, err
	}

	if acceptedBy.Valid {
		if id, err := uuid.Parse(acceptedBy.String); err == nil {
			invitation.AcceptedBy = &id
		}
	}

	return invitation, nil
}
