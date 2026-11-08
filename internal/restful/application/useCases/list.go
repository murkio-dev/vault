package useCases

import (
	"context"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type ListApplicationsUseCase struct {
	Lister     contracts.IApplicationLister
	UserLister contracts.IApplicationUserLister
}

func NewListApplicationsUseCase(lister contracts.IApplicationLister, userLister contracts.IApplicationUserLister) ListApplicationsUseCase {
	return ListApplicationsUseCase{
		Lister:     lister,
		UserLister: userLister,
	}
}

// DTOS

type ListInput struct {
	Requester entities.ApiHeader
	Page      int // starts at 1
	Limit     int // items per page, capped at entities.MaxPageSize
}

type ListOutput struct {
	Data  []entities.Application `json:"data"`
	Error *entities.Error        `json:"error,omitempty"`
	Page  *entities.Paginator    `json:"page,omitempty"`
}

// UseCase

func (u ListApplicationsUseCase) Perform(ctx context.Context, input ListInput) ListOutput {

	actor := input.Requester.Actor
	if !actor.Authenticated() || actor.Kind != entities.ActorUser {
		unauthorized := entities.UnauthorizedError()
		return ListOutput{Error: &unauthorized}
	}

	page := input.Page
	if page < 1 {
		page = 1
	}

	limit := input.Limit
	if limit < 1 || limit > entities.MaxPageSize {
		limit = entities.MaxPageSize
	}
	offset := (page - 1) * limit

	list := func() ([]entities.Application, int, error) {
		return u.UserLister.ListByUser(ctx, actor.UserID, limit, offset)
	}
	if actor.PlatformAdmin {
		list = func() ([]entities.Application, int, error) { return u.Lister.List(ctx, limit, offset) }
	}
	data, total, err := list()
	if err != nil {
		errMsg := entities.InternalServerError(err, nil)
		return ListOutput{
			Error: &errMsg,
		}
	}

	paginator := entities.NewPaginator(page, limit, total)
	return ListOutput{
		Data: data,
		Page: &paginator,
	}
}
