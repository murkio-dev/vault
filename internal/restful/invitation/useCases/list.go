package useCases

import (
	"context"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	invitationcontracts "github.com/murkio-dev/vault/internal/restful/invitation/contracts"
)

const MaxListed = 200

// Builder

type ListInvitationsUseCase struct {
	Repository invitationcontracts.IInvitationRepository
	Guard      contracts.IAccessGuard
}

func NewListInvitationsUseCase(repository invitationcontracts.IInvitationRepository, guard contracts.IAccessGuard) ListInvitationsUseCase {
	return ListInvitationsUseCase{Repository: repository, Guard: guard}
}

// DTOS

type ListInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
}

type ListOutput struct {
	Data  []entities.InvitationView `json:"data"`
	Error *entities.Error           `json:"error,omitempty"`
}

// UseCase

func (u ListInvitationsUseCase) Perform(ctx context.Context, input ListInput) ListOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionInvitationManage}).Error; denied != nil {
		return ListOutput{Error: denied}
	}

	invitations, _, err := u.Repository.List(ctx, input.Application, MaxListed, 0)
	if err != nil {
		internal := entities.InternalServerError(err, nil)
		return ListOutput{Error: &internal}
	}

	now := time.Now().UTC()
	views := make([]entities.InvitationView, len(invitations))
	for i, invitation := range invitations {
		views[i] = invitation.View(now)
	}

	return ListOutput{Data: views}
}
