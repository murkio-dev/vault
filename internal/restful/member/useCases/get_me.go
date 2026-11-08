package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type GetMeUseCase struct {
	Guard contracts.IAccessGuard
}

func NewGetMeUseCase(guard contracts.IAccessGuard) GetMeUseCase {
	return GetMeUseCase{Guard: guard}
}

// DTOS

type GetMeInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
}

type GetMeOutput struct {
	Data  entities.Member `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

func (u GetMeUseCase) Perform(ctx context.Context, input GetMeInput) GetMeOutput {

	guard := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionApplicationRead})
	member, denied := guard.Member, guard.Error
	if denied != nil {
		return GetMeOutput{Error: denied}
	}

	if member.ID == (uuid.UUID{}) {
		notFound := entities.NotFoundError("Resource")
		return GetMeOutput{Error: &notFound}
	}

	return GetMeOutput{Data: member}
}
