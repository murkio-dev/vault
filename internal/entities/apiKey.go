package entities

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

type KeyType string

const (
	KeyPublishable KeyType = "publishable"
	KeySecret      KeyType = "secret"
)

const (
	// SecretKeyPrefix e PublicKeyPrefix são o início de toda chave do Vault:
	// <tipo>_<sigla do serviço>_, como no Identity (sk_id_) e no Messaging
	// (sk_ms_). Só a sk_vt_ autentica; a pk_vt_ só identifica a aplicação e
	// nunca lê segredo.
	SecretKeyPrefix  = "sk_vt_"
	PublicKeyPrefix  = "pk_vt_"
	KeyDisplayLength = len(SecretKeyPrefix) + 6
	MaxKeyNameLength = 100
	MaxKeyLifetime   = 10 * 365 * 24 * time.Hour

	// RotationOverlap é quanto a chave antiga ainda vale depois de rotacionar,
	// para dar tempo de trocar a chave onde ela está configurada.
	RotationOverlap = 72 * time.Hour
)

var (
	ErrInvalidKeyName      = errors.New("name must have 1 to 100 printable characters")
	ErrInvalidKeyType      = errors.New("type must be publishable or secret")
	ErrInvalidKeyExpiresAt = errors.New("expires_at must be in the future, within 10 years")
)

type ApiKey struct {
	ID            uuid.UUID  `json:"id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	MemberID      uuid.UUID  `json:"member_id"`
	UriID         string     `json:"uri_id"`
	Name          string     `json:"name"`
	Type          KeyType    `json:"type"`
	Prefix        string     `json:"prefix"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
	UpdatedAt     *time.Time `json:"updated_at"`
	CreatedAt     time.Time  `json:"created_at"`
	DeletedAt     *time.Time `json:"deleted_at"`
	// Hash e UserID não saem em resposta nenhuma (json:"-"): o hash é o que o
	// banco guarda da chave, e o UserID é a conta dona do membro, que o
	// Authenticator lê ao resolver a chave no ator.
	Hash   string    `json:"-"`
	UserID uuid.UUID `json:"-"`
}

type ApiKeyCreated struct {
	ApiKey
	Key string `json:"key"`
	URI string `json:"uri"`
}

// NewApiKey gera a chave do membro e devolve a entidade e o valor em claro. O
// valor em claro não é guardado: só o hash e um prefixo curto para exibição.
// Exemplo: NewApiKey(member, "ci", KeySecret, nil) devolve um valor "sk_vt_...".
func NewApiKey(member Member, name string, keyType KeyType, expiresAt *time.Time) (ApiKey, string, error) {

	key := ApiKey{ID: uuid.NewV7(), ApplicationID: member.ApplicationID, MemberID: member.ID, UriID: member.UriID, Type: keyType, UserID: member.UserID}
	now := time.Now().UTC()

	var erros []error
	if err := key.Rename(name); err != nil {
		erros = append(erros, err)
	}
	if keyType != KeyPublishable && keyType != KeySecret {
		erros = append(erros, ErrInvalidKeyType)
	}
	if err := key.ExpireAt(expiresAt, now); err != nil {
		erros = append(erros, err)
	}
	if err := errors.Join(erros...); err != nil {
		return ApiKey{}, "", err
	}

	prefix := PublicKeyPrefix
	if keyType == KeySecret {
		prefix = SecretKeyPrefix
	}
	plain, err := NewToken(prefix)
	if err != nil {
		return ApiKey{}, "", err
	}

	key.Prefix = plain[:KeyDisplayLength]
	key.Hash = HashToken(plain)
	key.CreatedAt = now

	return key, plain, nil
}

// Rename troca o nome: 1 a 100 caracteres imprimíveis, sem espaço nas pontas.
func (k *ApiKey) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxKeyNameLength || strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return ErrInvalidKeyName
	}
	k.Name = name
	return nil
}

// ExpireAt define o vencimento: nil é "não vence"; senão, no futuro e em até 10
// anos a partir de now.
func (k *ApiKey) ExpireAt(expiresAt *time.Time, now time.Time) error {
	if expiresAt == nil {
		k.ExpiresAt = nil
		return nil
	}
	utc := expiresAt.UTC()
	if !utc.After(now) || utc.After(now.Add(MaxKeyLifetime)) {
		return ErrInvalidKeyExpiresAt
	}
	k.ExpiresAt = &utc
	return nil
}

// RotationDeadline é até quando a chave antiga vale depois da rotação: now +
// RotationOverlap, sem nunca estender um vencimento que já era mais cedo.
func (k ApiKey) RotationDeadline(now time.Time) time.Time {
	deadline := now.Add(RotationOverlap)
	if k.ExpiresAt != nil && k.ExpiresAt.Before(deadline) {
		return *k.ExpiresAt
	}
	return deadline
}

type AuditFilter struct {
	ApplicationID uuid.UUID
	ActorID       *uuid.UUID
	Action        string
	TargetID      string
	Outcome       AuditOutcome
	From          *time.Time
	To            *time.Time
	Cursor        uuid.UUID
	Limit         int
}
