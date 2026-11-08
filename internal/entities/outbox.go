package entities

import (
	"encoding/json"
	"time"
	"uuid"
)

const (
	MessagingExchange   = "messaging.commands"
	EmailSendRoutingKey = "email.send"

	TemplateVaultInvitation = "vault_invitation"
)

type OutboxEvent struct {
	ID            uuid.UUID
	ApplicationID uuid.UUID
	Type          string
	Payload       json.RawMessage
	Exchange      string
	Envelope      bool
	CreatedAt     time.Time
}

type sendEmail struct {
	ID            uuid.UUID      `json:"id"`
	ApplicationID uuid.UUID      `json:"application_id"`
	To            string         `json:"to"`
	Template      string         `json:"template"`
	Locale        string         `json:"locale"`
	Data          map[string]any `json:"data"`
}

func NewInvitationEmail(invitation Invitation, token string) (OutboxEvent, error) {
	id := uuid.NewV7()

	payload, err := json.Marshal(sendEmail{
		ID:            id,
		ApplicationID: invitation.ApplicationID,
		To:            invitation.Email,
		Template:      TemplateVaultInvitation,
		Locale:        "en",
		Data:          map[string]any{"token": token, "role": string(invitation.Role)},
	})
	if err != nil {
		return OutboxEvent{}, err
	}

	return OutboxEvent{
		ID:            id,
		ApplicationID: invitation.ApplicationID,
		Type:          EmailSendRoutingKey,
		Payload:       payload,
		Exchange:      MessagingExchange,
		Envelope:      false,
		CreatedAt:     time.Now().UTC(),
	}, nil
}
