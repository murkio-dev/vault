package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	membercontracts "github.com/murkio-dev/vault/internal/restful/member/contracts"
	"github.com/murkio-dev/vault/pkg/httpx"
)

// Builder

type ListMembersUseCase struct {
	Repository membercontracts.IMemberRepository
	Guard      contracts.IAccessGuard
}

func NewListMembersUseCase(repository membercontracts.IMemberRepository, guard contracts.IAccessGuard) ListMembersUseCase {
	return ListMembersUseCase{Repository: repository, Guard: guard}
}

// DTOS

type ListInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Page        int
	Limit       int
}

type ListOutput struct {
	Data  []entities.Member   `json:"data"`
	Error *entities.Error     `json:"error,omitempty"`
	Page  *entities.Paginator `json:"page,omitempty"`
}

// UseCase

func (u ListMembersUseCase) Perform(ctx context.Context, input ListInput) ListOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionMemberList}).Error; denied != nil {
		return ListOutput{Error: denied}
	}

	page, limit, offset := httpx.PageWindow(input.Page, input.Limit)

	members, total, err := u.Repository.List(ctx, input.Application, limit, offset)
	if err != nil {
		internal := entities.InternalServerError(err, nil)
		return ListOutput{Error: &internal}
	}

	paginator := entities.NewPaginator(page, limit, total)
	return ListOutput{Data: members, Page: &paginator}
}
