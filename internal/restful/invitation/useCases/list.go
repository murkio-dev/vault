package useCases

import (
	"context"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// MaxListed é quantos convites a listagem devolve (os mais novos).
const MaxListed = 200

// Builder

type ListInvitationsUseCase struct {
	Lister contracts.IInvitationLister
	Guard  contracts.IAccessGuard
}

func NewListInvitationsUseCase(lister contracts.IInvitationLister, guard contracts.IAccessGuard) ListInvitationsUseCase {
	return ListInvitationsUseCase{Lister: lister, Guard: guard}
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

	invitations, _, err := u.Lister.List(ctx, input.Application, MaxListed, 0)
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
