package entities_test

import (
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestNewInvitationEmail(t *testing.T) {
	t.Run("Quando o contrato do e-mail é montado.", func(t *testing.T) { whenInvitationEmailContractIsBuilt(t) })
}

// Functions

func whenInvitationEmailContractIsBuilt(t *testing.T) {
	invitation, token, err := entities.NewInvitation(uuid.NewV7(), "joao@example.com", entities.RoleDeveloper, uuid.NewV7())
	require.NoError(t, err)

	event, err := entities.NewInvitationEmail(invitation, token)

	require.NoError(t, err)
	assert.Equal(t, entities.MessagingExchange, event.Exchange)
	assert.Equal(t, entities.EmailSendRoutingKey, event.Type)
	assert.False(t, event.Envelope)
	assert.JSONEq(t, `{"id":"`+event.ID.String()+`","application_id":"`+invitation.ApplicationID.String()+`","to":"joao@example.com","template":"vault_invitation","locale":"en","data":{"token":"`+token+`","role":"developer"}}`, string(event.Payload))
}
