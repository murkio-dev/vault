package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/invitation/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestListInvitations(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenListBuildSucceeds(t) })
	t.Run("Quando lista com o status derivado.", func(t *testing.T) { whenListShowsDerivedStatus(t) })
	t.Run("Quando não há convites.", func(t *testing.T) { whenListIsEmpty(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenListIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenListFailsInDb(t) })
}

// Functions

func whenListBuildSucceeds(t *testing.T) {
	repo, guard := &mocks.InvitationRepo{}, &member_mocks.Guard{}

	useCase := useCases.NewListInvitationsUseCase(repo, guard)

	assert.Equal(t, repo, useCase.Lister)
	assert.Equal(t, guard, useCase.Guard)
}

func whenListShowsDerivedStatus(t *testing.T) {
	now := time.Now().UTC()
	pending, accepted, revoked, expired := mocks.NewInvitation(), mocks.NewInvitation(), mocks.NewInvitation(), mocks.NewInvitation()
	accepted.AcceptedAt = &now
	revoked.RevokedAt = &now
	expired.ExpiresAt = now.Add(-time.Minute)
	repo := &mocks.InvitationRepo{ListFunc: func(context.Context, uuid.UUID, int, int) ([]entities.Invitation, int, error) {
		return []entities.Invitation{pending, accepted, revoked, expired}, 4, nil
	}}

	output := useCases.NewListInvitationsUseCase(repo, &member_mocks.Guard{}).
		Perform(context.Background(), useCases.ListInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7()})

	assert.Nil(t, output.Error)
	statuses := make([]entities.InvitationStatus, len(output.Data))
	for i, view := range output.Data {
		statuses[i] = view.Status
	}
	assert.Equal(t, []entities.InvitationStatus{entities.InvitationPending, entities.InvitationAccepted, entities.InvitationRevoked, entities.InvitationExpired}, statuses)
}

func whenListIsEmpty(t *testing.T) {
	output := useCases.NewListInvitationsUseCase(&mocks.InvitationRepo{}, &member_mocks.Guard{}).
		Perform(context.Background(), useCases.ListInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7()})

	assert.Nil(t, output.Error)
	assert.NotNil(t, output.Data, "lista vazia é um array, não nulo")
	assert.Empty(t, output.Data)
}

func whenListIsDenied(t *testing.T) {
	repo := &mocks.InvitationRepo{ListFunc: func(context.Context, uuid.UUID, int, int) ([]entities.Invitation, int, error) {
		t.Error("negado, o repositório não é chamado")
		return nil, 0, nil
	}}
	guard := (&member_mocks.Guard{}).Deny(entities.ForbiddenError())

	output := useCases.NewListInvitationsUseCase(repo, guard).
		Perform(context.Background(), useCases.ListInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
}

func whenListFailsInDb(t *testing.T) {
	repo := &mocks.InvitationRepo{ListFunc: func(context.Context, uuid.UUID, int, int) ([]entities.Invitation, int, error) {
		return nil, 0, errors.New("segredo-interno")
	}}

	output := useCases.NewListInvitationsUseCase(repo, &member_mocks.Guard{}).
		Perform(context.Background(), useCases.ListInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}
