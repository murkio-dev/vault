package entities

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"uuid"
)

const (
	InvitationTokenPrefix = "inv_"
	InvitationTTL         = 7 * 24 * time.Hour
	MaxEmailBytes         = 254
)

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationRevoked  InvitationStatus = "revoked"
	InvitationExpired  InvitationStatus = "expired"
)

type Invitation struct {
	ID            uuid.UUID  `json:"id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	Email         string     `json:"email"`
	Role          Role       `json:"role"`
	ExpiresAt     time.Time  `json:"expires_at"`
	CreatedBy     uuid.UUID  `json:"created_by"`
	AcceptedBy    *uuid.UUID `json:"accepted_by,omitempty"`
	AcceptedAt    *time.Time `json:"accepted_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	// TokenHash é o que o banco guarda do token. Fica fora de toda resposta
	// (json:"-"): só o repositório o lê.
	TokenHash string `json:"-"`
}

type InvitationCreated struct {
	Invitation
	Token string `json:"token"`
}

// Status deriva o estado do convite: aceito, revogado, vencido ou pendente.
// Não é uma coluna: o relógio decide o vencimento, então guardar o estado
// deixaria um convite vencido ainda "pendente" até alguém o atualizar. A ordem
// importa: aceito vence revogado, que vence vencido. Entrada: o instante em
// que se quer avaliar (normalmente agora). Exemplo: expires_at no passado e
// sem aceite nem revogação devolve InvitationExpired.
func (i Invitation) Status(now time.Time) InvitationStatus {
	switch {
	case i.AcceptedAt != nil:
		return InvitationAccepted
	case i.RevokedAt != nil:
		return InvitationRevoked
	case !now.Before(i.ExpiresAt):
		return InvitationExpired
	}
	return InvitationPending
}

type InvitationView struct {
	Invitation
	Status InvitationStatus `json:"status"`
}

func (i Invitation) View(now time.Time) InvitationView {
	return InvitationView{Invitation: i, Status: i.Status(now)}
}

func NewInvitation(applicationID uuid.UUID, email string, role Role, createdBy uuid.UUID) (Invitation, string, error) {

	email = strings.TrimSpace(email)
	if err := validateEmail(email); err != nil {
		return Invitation{}, "", err
	}
	if !role.Valid() {
		return Invitation{}, "", errors.New("role must be admin or developer")
	}

	token, err := NewToken(InvitationTokenPrefix)
	if err != nil {
		return Invitation{}, "", err
	}

	now := time.Now().UTC()
	return Invitation{
		ID:            uuid.NewV7(),
		ApplicationID: applicationID,
		Email:         strings.ToLower(email),
		Role:          role,
		ExpiresAt:     now.Add(InvitationTTL),
		CreatedBy:     createdBy,
		CreatedAt:     now,
		TokenHash:     HashToken(token),
	}, token, nil
}

func validateEmail(email string) error {
	if email == "" || len(email) > MaxEmailBytes {
		return errors.New("invalid email")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return errors.New("invalid email")
	}
	return nil
}

func NewToken(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashToken é o SHA-256 em hex do token. É o que o banco guarda e o que se
// procura. Um token de 256 bits aleatórios não precisa de hash lento (argon2,
// bcrypt): o espaço de busca já é inviável, e o hash rápido deixa a
// autenticação de cada requisição do SDK barata. A busca é por igualdade do
// hash, sem comparar o token. Exemplo: HashToken("vt_abc") sempre dá o mesmo
// valor de 64 caracteres, e dois tokens diferentes nunca colidem na prática.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
