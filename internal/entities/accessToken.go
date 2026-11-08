package entities

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

const (
	SDKTokenPrefix     = "vt_"
	TokenDisplayLength = len(SDKTokenPrefix) + 6
	MaxTokenNameLength = 100
	MaxTokenLifetime   = 10 * 365 * 24 * time.Hour
)

type AccessToken struct {
	ID            uuid.UUID  `json:"id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	MemberID      uuid.UUID  `json:"member_id"`
	UriID         string     `json:"uri_id"`
	Name          string     `json:"name"`
	Prefix        string     `json:"prefix"`
	ExpiresAt     *time.Time `json:"expires_at"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	CreatedAt     time.Time  `json:"created_at"`
	// TokenHash e UserID não saem em resposta nenhuma (json:"-"): o hash é o
	// que o banco guarda do token, e o UserID é a conta dona do membro, que o
	// Authenticator lê ao resolver o token no ator.
	TokenHash string    `json:"-"`
	UserID    uuid.UUID `json:"-"`
}

type AccessTokenCreated struct {
	AccessToken
	Token string `json:"token"`
	URI   string `json:"uri"`
}

func NewAccessToken(member Member, name string, expiresAt *time.Time) (AccessToken, string, error) {

	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxTokenNameLength || strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return AccessToken{}, "", errors.New("name must have 1 to 100 printable characters")
	}

	now := time.Now().UTC()
	if expiresAt != nil {
		utc := expiresAt.UTC()
		if !utc.After(now) || utc.After(now.Add(MaxTokenLifetime)) {
			return AccessToken{}, "", errors.New("expires_at must be in the future, within 10 years")
		}
		expiresAt = &utc
	}

	token, err := NewToken(SDKTokenPrefix)
	if err != nil {
		return AccessToken{}, "", err
	}

	return AccessToken{
		ID:            uuid.NewV7(),
		ApplicationID: member.ApplicationID,
		MemberID:      member.ID,
		UriID:         member.UriID,
		Name:          name,
		Prefix:        token[:TokenDisplayLength],
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
		TokenHash:     HashToken(token),
		UserID:        member.UserID,
	}, token, nil
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
