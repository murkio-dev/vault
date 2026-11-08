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

// SecretRepository guarda os metadados dos segredos. O valor só passa por aqui
// já cifrado (SecretWrite.Ciphertext) e nunca volta: quem lê as versões é o
// VersionRepository.
type SecretRepository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) SecretRepository {
	return SecretRepository{DB: db}
}

// VersionRepository lê as versões de um segredo e recifra o valor de uma delas.
// Fica no mesmo arquivo porque as duas tabelas se movem juntas.
type VersionRepository struct {
	DB *sql.DB
}

func NewVersionRepository(db *sql.DB) VersionRepository {
	return VersionRepository{DB: db}
}

var (
	_ contracts.ISecretCreator          = SecretRepository{}
	_ contracts.ISecretFinder           = SecretRepository{}
	_ contracts.ISecretOwnerFinder      = SecretRepository{}
	_ contracts.ISecretLister           = SecretRepository{}
	_ contracts.ISecretCounter          = SecretRepository{}
	_ contracts.ISecretReferenceCounter = SecretRepository{}
	_ contracts.ISecretVersionAdder     = SecretRepository{}
	_ contracts.ISecretRenamer          = SecretRepository{}
	_ contracts.ISecretDeleter          = SecretRepository{}

	_ contracts.IVersionLister        = VersionRepository{}
	_ contracts.IVersionCurrentFinder = VersionRepository{}
	_ contracts.IVersionNumberFinder  = VersionRepository{}
	_ contracts.IVersionResealer      = VersionRepository{}
)

// Functions

const secretColumns = "id, application_id, member_id, name, type, source, reference_secret_id, current_version, created_by, created_at, updated_at"

// Create grava o segredo e, se vier ciphertext, a versão 1. Uma referência vem
// sem ciphertext e fica na versão 0.
func (r SecretRepository) Create(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
	if err := entities.Require(write.ApplicationID); err != nil {
		return entities.Secret{}, err
	}

	conn := sqlite.Conn(ctx, r.DB)
	secret := write.Secret

	secret.Version = 0
	if write.Ciphertext != nil {
		secret.Version = 1
	}

	var reference any
	if secret.ReferenceSecretID != nil {
		reference = secret.ReferenceSecretID.String()
	}

	_, err := conn.ExecContext(ctx, `
		INSERT INTO secrets (id, application_id, member_id, name, type, source, reference_secret_id, current_version, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		secret.ID.String(), secret.ApplicationID.String(), secret.MemberID.String(), secret.Name, string(secret.Type),
		string(secret.Source), reference, secret.Version, secret.CreatedBy.String(), secret.CreatedAt)
	if err != nil {
		return entities.Secret{}, err
	}

	if write.Ciphertext != nil {
		author := write.Author
		if author == (uuid.UUID{}) {
			author = secret.CreatedBy
		}
		if err := insertVersion(ctx, conn, secret.ID, write.VersionID, 1, write.Ciphertext, author, secret.CreatedAt); err != nil {
			return entities.Secret{}, err
		}
	}

	return secret, nil
}

func insertVersion(ctx context.Context, conn sqlite.Executor, secretID, versionID uuid.UUID, version int, ciphertext []byte, createdBy uuid.UUID, at time.Time) error {
	_, err := conn.ExecContext(ctx,
		"INSERT INTO secret_versions (id, secret_id, version, ciphertext, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		versionID.String(), secretID.String(), version, ciphertext, createdBy.String(), at)
	return err
}

// FindById acha o segredo na aplicação, de qualquer dono (é como a referência
// encontra o alvo). Quem lê em nome de um membro usa FindByOwner.
func (r SecretRepository) FindById(ctx context.Context, application, id uuid.UUID) (entities.Secret, error) {
	if err := entities.Require(application, id); err != nil {
		return entities.Secret{}, err
	}

	return scanSecret(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT "+secretColumns+" FROM secrets WHERE application_id = ? AND id = ? AND deleted_at IS NULL", application.String(), id.String()))
}

// FindByOwner só acha o segredo da tabela do dono.
func (r SecretRepository) FindByOwner(ctx context.Context, application, owner, id uuid.UUID) (entities.Secret, error) {
	if err := entities.Require(application, owner, id); err != nil {
		return entities.Secret{}, err
	}

	return scanSecret(sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT "+secretColumns+" FROM secrets WHERE application_id = ? AND member_id = ? AND id = ? AND deleted_at IS NULL",
		application.String(), owner.String(), id.String()))
}

// List lista a tabela de um membro.
func (r SecretRepository) List(ctx context.Context, application, owner uuid.UUID, limit, offset int) ([]entities.Secret, int, error) {
	if err := entities.Require(application, owner); err != nil {
		return nil, 0, err
	}

	const where = "application_id = ? AND member_id = ? AND deleted_at IS NULL"
	args := []any{application.String(), owner.String()}
	conn := sqlite.Conn(ctx, r.DB)

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM secrets WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.QueryContext(ctx, "SELECT "+secretColumns+" FROM secrets WHERE "+where+" ORDER BY name LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	secrets := []entities.Secret{}
	for rows.Next() {
		secret, err := scanSecret(rows)
		if err != nil {
			return nil, 0, err
		}
		secrets = append(secrets, secret)
	}

	return secrets, total, rows.Err()
}

// Count conta os segredos da tabela de um membro.
func (r SecretRepository) Count(ctx context.Context, application, owner uuid.UUID) (int, error) {
	if err := entities.Require(application, owner); err != nil {
		return 0, err
	}

	var total int
	err := sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM secrets WHERE application_id = ? AND member_id = ? AND deleted_at IS NULL",
		application.String(), owner.String()).Scan(&total)
	return total, err
}

// CountReferences conta os segredos da aplicação que apontam para o segredo.
func (r SecretRepository) CountReferences(ctx context.Context, application, secret uuid.UUID) (int, error) {
	if err := entities.Require(application, secret); err != nil {
		return 0, err
	}

	var total int
	err := sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM secrets WHERE application_id = ? AND reference_secret_id = ? AND deleted_at IS NULL",
		application.String(), secret.String()).Scan(&total)
	return total, err
}

// AddVersion grava uma NOVA versão do valor: a versão seguinte à Secret.Version
// que o chamador leu, com o SecretWrite.Ciphertext. O UPDATE condicional em
// current_version faz duas gravações simultâneas não gerarem duas versões com o
// mesmo número: a segunda recebe sql.ErrNoRows.
func (r SecretRepository) AddVersion(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
	if err := entities.Require(write.ApplicationID, write.ID); err != nil {
		return entities.Secret{}, err
	}

	conn := sqlite.Conn(ctx, r.DB)
	now := time.Now().UTC()
	next := write.Version + 1

	result, err := conn.ExecContext(ctx,
		"UPDATE secrets SET current_version = ?, updated_at = ? WHERE application_id = ? AND id = ? AND current_version = ? AND deleted_at IS NULL",
		next, now, write.ApplicationID.String(), write.ID.String(), write.Version)
	if err := mustAffect(result, err); err != nil {
		return entities.Secret{}, err
	}

	if err := insertVersion(ctx, conn, write.ID, write.VersionID, next, write.Ciphertext, write.Author, now); err != nil {
		return entities.Secret{}, err
	}

	return r.FindById(ctx, write.ApplicationID, write.ID)
}

// Rename renomeia o segredo para SecretWrite.Name. O valor e as versões não
// mudam.
func (r SecretRepository) Rename(ctx context.Context, write entities.SecretWrite) (entities.Secret, error) {
	if err := entities.Require(write.ApplicationID, write.ID); err != nil {
		return entities.Secret{}, err
	}

	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE secrets SET name = ?, updated_at = ? WHERE application_id = ? AND id = ? AND deleted_at IS NULL",
		write.Name, time.Now().UTC(), write.ApplicationID.String(), write.ID.String())
	if err := mustAffect(result, err); err != nil {
		return entities.Secret{}, err
	}

	return r.FindById(ctx, write.ApplicationID, write.ID)
}

// Delete apaga o segredo (soft delete: as versões cifradas ficam para a
// auditoria).
func (r SecretRepository) Delete(ctx context.Context, application, id uuid.UUID) error {
	if err := entities.Require(application, id); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx,
		"UPDATE secrets SET deleted_at = ?, updated_at = ? WHERE application_id = ? AND id = ? AND deleted_at IS NULL", now, now, application.String(), id.String())
	return mustAffect(result, err)
}

const versionFrom = ` FROM secret_versions v JOIN secrets s ON s.id = v.secret_id
		WHERE s.application_id = ? AND s.deleted_at IS NULL AND v.secret_id = ?`

// List lista as versões de um segredo ativo da aplicação, da mais nova para a
// mais antiga. Devolve também o ciphertext, que nunca sai em resposta
// (json:"-").
func (r VersionRepository) List(ctx context.Context, application, secret uuid.UUID, limit, offset int) ([]entities.SecretVersion, int, error) {
	if err := entities.Require(application, secret); err != nil {
		return nil, 0, err
	}

	args := []any{application.String(), secret.String()}
	conn := sqlite.Conn(ctx, r.DB)

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*)"+versionFrom, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.QueryContext(ctx, "SELECT v.id, v.version, v.created_by, v.created_at, v.ciphertext"+versionFrom+" ORDER BY v.version DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	versions := []entities.SecretVersion{}
	for rows.Next() {
		var version entities.SecretVersion
		if err := rows.Scan(&version.ID, &version.Version, &version.CreatedBy, &version.CreatedAt, &version.Ciphertext); err != nil {
			return nil, 0, err
		}
		versions = append(versions, version)
	}

	return versions, total, rows.Err()
}

// FindCurrent acha a versão atual do segredo.
func (r VersionRepository) FindCurrent(ctx context.Context, application, secret uuid.UUID) (entities.SecretVersion, error) {
	if err := entities.Require(application, secret); err != nil {
		return entities.SecretVersion{}, err
	}

	return r.find(ctx, versionFrom+" AND v.version = s.current_version", application, secret)
}

// FindByNumber acha uma versão pelo número (a partir de 1).
func (r VersionRepository) FindByNumber(ctx context.Context, application, secret uuid.UUID, number int) (entities.SecretVersion, error) {
	if err := entities.Require(application, secret); err != nil {
		return entities.SecretVersion{}, err
	}
	if number < 1 {
		return entities.SecretVersion{}, entities.ErrMissingParam
	}

	return r.find(ctx, versionFrom+" AND v.version = ?", application, secret, number)
}

func (r VersionRepository) find(ctx context.Context, from string, application, secret uuid.UUID, extra ...any) (entities.SecretVersion, error) {
	args := append([]any{application.String(), secret.String()}, extra...)

	var version entities.SecretVersion
	err := sqlite.Conn(ctx, r.DB).QueryRowContext(ctx,
		"SELECT v.id, v.version, v.created_by, v.created_at, v.ciphertext"+from, args...).
		Scan(&version.ID, &version.Version, &version.CreatedBy, &version.CreatedAt, &version.Ciphertext)
	return version, err
}

// Reseal troca o ciphertext de uma versão: é a recifragem com a chave nova
// quando a ENCRYPTION_KEY gira. É a única alteração que o banco aceita numa
// versão (o trigger recusa o resto). Exige a aplicação e o segredo, para
// recifrar só versão de um segredo da aplicação.
func (r VersionRepository) Reseal(ctx context.Context, application, secret uuid.UUID, version entities.SecretVersion) (entities.SecretVersion, error) {
	if err := entities.Require(application, secret, version.ID); err != nil {
		return entities.SecretVersion{}, err
	}

	result, err := sqlite.Conn(ctx, r.DB).ExecContext(ctx, `
		UPDATE secret_versions SET ciphertext = ? WHERE id = ? AND secret_id = ?
		AND secret_id IN (SELECT id FROM secrets WHERE application_id = ?)`,
		version.Ciphertext, version.ID.String(), secret.String(), application.String())
	if err := mustAffect(result, err); err != nil {
		return entities.SecretVersion{}, err
	}

	return version, nil
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

func scanSecret(row scanner) (entities.Secret, error) {
	var (
		secret    entities.Secret
		reference sql.NullString
	)

	err := row.Scan(&secret.ID, &secret.ApplicationID, &secret.MemberID, &secret.Name, &secret.Type, &secret.Source,
		&reference, &secret.Version, &secret.CreatedBy, &secret.CreatedAt, &secret.UpdatedAt)
	if err != nil {
		return entities.Secret{}, err
	}

	if reference.Valid {
		if id, err := uuid.Parse(reference.String); err == nil {
			secret.ReferenceSecretID = &id
		}
	}

	return secret, nil
}
