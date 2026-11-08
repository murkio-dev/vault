package entities

import (
	"encoding/json"
	"time"
	"uuid"
)

type AuditOutcome string

const (
	AuditAllowed AuditOutcome = "allowed"
	AuditDenied  AuditOutcome = "denied"
)

const (
	AuditApplicationCreated = "application.created"
	AuditApplicationUpdated = "application.updated"
	AuditApplicationDeleted = "application.deleted"
	AuditMemberAdded        = "member.added"
	AuditMemberUpdated      = "member.updated"
	AuditMemberRemoved      = "member.removed"
	AuditInvitationCreated  = "invitation.created"
	AuditInvitationAccepted = "invitation.accepted"
	AuditInvitationRevoked  = "invitation.revoked"
	AuditSecretCreated      = "secret.created"
	AuditSecretUpdated      = "secret.updated"
	AuditSecretRenamed      = "secret.renamed"
	AuditSecretDeleted      = "secret.deleted"
	AuditSecretRevealed     = "secret.revealed"
	AuditTokenCreated       = "token.created"
	AuditTokenRevoked       = "token.revoked"
	AuditEnvRead            = "env.read"
	AuditAccessDenied       = "access.denied"
)

const MaxAuditText = 256

type AuditLog struct {
	ID            uuid.UUID       `json:"id"`
	ApplicationID *uuid.UUID      `json:"application_id,omitempty"`
	ActorType     string          `json:"actor_type"`
	ActorID       *uuid.UUID      `json:"actor_id,omitempty"`
	TokenID       *uuid.UUID      `json:"token_id,omitempty"`
	Action        string          `json:"action"`
	Outcome       AuditOutcome    `json:"outcome"`
	TargetType    string          `json:"target_type,omitempty"`
	TargetID      string          `json:"target_id,omitempty"`
	IP            string          `json:"ip,omitempty"`
	UserAgent     string          `json:"user_agent,omitempty"`
	RequestID     string          `json:"request_id,omitempty"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
}

type AuditTarget struct {
	Type string
	ID   string
}

func NewAuditLog(requester ApiHeader, applicationID uuid.UUID, action string, outcome AuditOutcome, target AuditTarget, metadata map[string]any) AuditLog {

	log := AuditLog{
		ID:         uuid.NewV7(),
		ActorType:  requester.Actor.AuditType(),
		Action:     action,
		Outcome:    outcome,
		TargetType: target.Type,
		TargetID:   target.ID,
		IP:         truncate(requester.ClientIP),
		UserAgent:  truncate(requester.UserAgent),
		RequestID:  truncate(requester.RequestID),
		CreatedAt:  time.Now().UTC(),
	}

	if applicationID != (uuid.UUID{}) {
		log.ApplicationID = &applicationID
	}
	if requester.Actor.UserID != (uuid.UUID{}) {
		id := requester.Actor.UserID
		log.ActorID = &id
	}
	if requester.Actor.TokenID != (uuid.UUID{}) {
		id := requester.Actor.TokenID
		log.TokenID = &id
	}

	log.Metadata = json.RawMessage("{}")
	if len(metadata) > 0 {
		if raw, err := json.Marshal(metadata); err == nil {
			log.Metadata = raw
		}
	}

	return log
}

func truncate(value string) string {
	if len(value) > MaxAuditText {
		return value[:MaxAuditText]
	}
	return value
}
