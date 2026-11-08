package entities

import (
	"crypto/rand"
	"encoding/hex"
	"time"
	"uuid"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
)

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleDeveloper
}

type MemberStatus string

const (
	MemberActive    MemberStatus = "active"
	MemberSuspended MemberStatus = "suspended"
)

func (s MemberStatus) Valid() bool {
	return s == MemberActive || s == MemberSuspended
}

type Member struct {
	ID            uuid.UUID    `json:"id"`
	ApplicationID uuid.UUID    `json:"application_id"`
	UserID        uuid.UUID    `json:"user_id"`
	UriID         string       `json:"uri_id"`
	Role          Role         `json:"role"`
	Status        MemberStatus `json:"status"`
	InvitedBy     *uuid.UUID   `json:"invited_by,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     *time.Time   `json:"updated_at,omitempty"`
}

func (m Member) Active() bool {
	return m.Status == MemberActive
}

func NewMember(applicationID, userID uuid.UUID, role Role, invitedBy *uuid.UUID) Member {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		panic("entities: no randomness available: " + err.Error())
	}

	return Member{
		ID:            uuid.NewV7(),
		ApplicationID: applicationID,
		UserID:        userID,
		UriID:         hex.EncodeToString(raw),
		Role:          role,
		Status:        MemberActive,
		InvitedBy:     invitedBy,
		CreatedAt:     time.Now().UTC(),
	}
}
