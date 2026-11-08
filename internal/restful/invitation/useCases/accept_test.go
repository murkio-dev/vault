package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestAcceptInvitation(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenAcceptBuildSucceeds(t) })
	t.Run("Quando aceita o convite.", func(t *testing.T) { whenAcceptSucceeds(t) })
	t.Run("Quando o token é inválido.", func(t *testing.T) { whenAcceptTokenIsInvalid(t) })
	t.Run("Quando o convite não está pendente.", func(t *testing.T) { whenAcceptInvitationIsNotPending(t) })
	t.Run("Quando a conta já é membro.", func(t *testing.T) { whenAcceptAccountIsAlreadyMember(t) })
	t.Run("Quando outro aceita o mesmo link antes.", func(t *testing.T) { whenAcceptLosesTheRace(t) })
	t.Run("Quando não há credencial.", func(t *testing.T) { whenAcceptHasNoCredential(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenAcceptFailsInDb(t) })
}

// Functions

type acceptRig struct {
	invitations *mocks.InvitationRepo
	members     *member_mocks.MemberRepo
	guard       *member_mocks.Guard
	tx          *audit_mocks.Tx
	audit       *audit_mocks.AuditRepo
	useCase     useCases.AcceptInvitationUseCase
}

func newAcceptRig(invitation entities.Invitation) *acceptRig {
	rig := &acceptRig{
		invitations: &mocks.InvitationRepo{FindByTokenHashFunc: func(context.Context, string) (entities.Invitation, error) {
			return invitation, nil
		}},
		members: &member_mocks.MemberRepo{},
		guard:   &member_mocks.Guard{},
		tx:      &audit_mocks.Tx{},
		audit:   &audit_mocks.AuditRepo{},
	}
	rig.useCase = useCases.NewAcceptInvitationUseCase(rig.invitations, rig.invitations, rig.members, rig.members, rig.guard, rig.tx, rig.audit)
	return rig
}

func validToken() string { return entities.InvitationTokenPrefix + strings.Repeat("A", 43) }

func whenAcceptBuildSucceeds(t *testing.T) {
	rig := newAcceptRig(mocks.NewInvitation())

	assert.Equal(t, rig.invitations, rig.useCase.TokenFinder)
	assert.Equal(t, rig.invitations, rig.useCase.Acceptor)
	assert.Equal(t, rig.members, rig.useCase.MemberCreator)
	assert.Equal(t, rig.members, rig.useCase.MemberFinder)
	assert.Equal(t, rig.guard, rig.useCase.Guard)
	assert.Equal(t, rig.tx, rig.useCase.Tx)
	assert.Equal(t, rig.audit, rig.useCase.Audit)
}

func whenAcceptSucceeds(t *testing.T) {
	invitation := mocks.NewInvitation()
	rig := newAcceptRig(invitation)
	requester := member_mocks.NewUserRequester()

	output := rig.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: requester, Token: validToken()})

	assert.Nil(t, output.Error)
	assert.Equal(t, requester.Actor.UserID, output.Data.UserID, "o membro é a conta que aceitou")
	assert.Equal(t, invitation.ApplicationID, output.Data.ApplicationID)
	assert.Equal(t, invitation.Role, output.Data.Role)
	assert.Len(t, rig.invitations.Accepted, 1)
	assert.Len(t, rig.members.Created, 1)
	assert.Equal(t, []string{entities.AuditInvitationAccepted}, rig.tx.Actions())
	assert.Equal(t, []string{entities.HashToken(validToken())}, rig.invitations.Hashes, "a busca é pelo hash")
	assert.Equal(t, invitation.ApplicationID, rig.invitations.Application(), "o aceite vai na aplicação do convite")
	assert.Equal(t, invitation.ApplicationID, rig.members.Application(), "e o membro também")
	if assert.NotNil(t, rig.invitations.Accepted[0].AcceptedBy) {
		assert.Equal(t, requester.Actor.UserID, *rig.invitations.Accepted[0].AcceptedBy)
	}
}

func whenAcceptTokenIsInvalid(t *testing.T) {
	rig := newAcceptRig(mocks.NewInvitation())
	rig.invitations.FindByTokenHashFunc = func(context.Context, string) (entities.Invitation, error) {
		return entities.Invitation{}, sql.ErrNoRows
	}

	for name, token := range map[string]string{"vazio": "", "sem prefixo": "qualquer-coisa", "gigante": entities.InvitationTokenPrefix + strings.Repeat("A", 200), "inexistente": validToken()} {
		output := rig.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: member_mocks.NewUserRequester(), Token: token})

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusNotFound, output.Error.Status, name)
		}
	}
	assert.Empty(t, rig.invitations.Accepted)
	assert.Empty(t, rig.members.Created)
	assert.Len(t, rig.audit.Created, 4, "cada tentativa que falha fica na trilha")
}

func whenAcceptInvitationIsNotPending(t *testing.T) {
	now := time.Now().UTC()
	accepted, revoked, expired := mocks.NewInvitation(), mocks.NewInvitation(), mocks.NewInvitation()
	accepted.AcceptedAt = &now
	revoked.RevokedAt = &now
	expired.ExpiresAt = now.Add(-time.Minute)

	for name, invitation := range map[string]entities.Invitation{"aceito": accepted, "revogado": revoked, "vencido": expired} {
		rig := newAcceptRig(invitation)

		output := rig.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: member_mocks.NewUserRequester(), Token: validToken()})

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusNotFound, output.Error.Status, name+": igual ao inexistente")
		}
		assert.Empty(t, rig.invitations.Accepted, name)
		assert.Len(t, rig.audit.Created, 1, name)
	}
}

func whenAcceptAccountIsAlreadyMember(t *testing.T) {
	rig := newAcceptRig(mocks.NewInvitation())
	rig.members.FindByUserFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		return member_mocks.NewMember(entities.RoleDeveloper), nil
	}

	output := rig.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: member_mocks.NewUserRequester(), Token: validToken()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusConflict, output.Error.Status)
	}
	assert.Empty(t, rig.invitations.Accepted, "o convite não é consumido à toa")
}

func whenAcceptLosesTheRace(t *testing.T) {
	rig := newAcceptRig(mocks.NewInvitation())
	rig.invitations.AcceptFunc = func(context.Context, entities.Invitation) (entities.Invitation, error) {
		return entities.Invitation{}, sql.ErrNoRows
	}

	output := rig.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: member_mocks.NewUserRequester(), Token: validToken()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Empty(t, rig.members.Created, "quem perdeu a corrida não vira membro")
}

func whenAcceptHasNoCredential(t *testing.T) {
	rig := newAcceptRig(mocks.NewInvitation())
	rig.guard.Deny(entities.UnauthorizedError())

	output := rig.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: entities.ApiHeader{}, Token: validToken()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusUnauthorized, output.Error.Status)
	}
	assert.Empty(t, rig.invitations.Hashes, "sem credencial, o token nem é procurado")
}

func whenAcceptFailsInDb(t *testing.T) {
	rig := newAcceptRig(mocks.NewInvitation())
	rig.members.CreateFunc = func(context.Context, entities.Member) (entities.Member, error) {
		return entities.Member{}, errors.New("segredo-interno")
	}

	output := rig.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: member_mocks.NewUserRequester(), Token: validToken()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}

	lookup := newAcceptRig(mocks.NewInvitation())
	lookup.invitations.FindByTokenHashFunc = func(context.Context, string) (entities.Invitation, error) {
		return entities.Invitation{}, errors.New("segredo-interno")
	}
	failed := lookup.useCase.Perform(context.Background(), useCases.AcceptInput{Requester: member_mocks.NewUserRequester(), Token: validToken()})
	if assert.NotNil(t, failed.Error) {
		assert.Equal(t, entities.ErrTypeInternal, failed.Error.Type, "erro de banco na busca não vira 404")
	}
}
