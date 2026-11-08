package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

const (
	DefaultLimit = 100
	MaxLimit     = 200
)

// Builder

type ListAuditLogsUseCase struct {
	Lister contracts.IAuditLogLister
	Guard  contracts.IAccessGuard
}

func NewListAuditLogsUseCase(lister contracts.IAuditLogLister, guard contracts.IAccessGuard) ListAuditLogsUseCase {
	return ListAuditLogsUseCase{Lister: lister, Guard: guard}
}

// DTOS

type ListInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Filter      entities.AuditFilter
}

type CursorPage struct {
	NextCursor *string `json:"next_cursor"`
}

type ListOutput struct {
	Data  []entities.AuditLog `json:"data"`
	Error *entities.Error     `json:"error,omitempty"`
	Page  *CursorPage         `json:"page,omitempty"`
}

// UseCase

func (u ListAuditLogsUseCase) Perform(ctx context.Context, input ListInput) ListOutput {

	guard := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionAuditRead, Owner: entities.SelfOwner})
	member, denied := guard.Member, guard.Error
	if denied != nil {
		return ListOutput{Error: denied}
	}

	filter := input.Filter
	filter.ApplicationID = input.Application
	if filter.Limit < 1 || filter.Limit > MaxLimit {
		filter.Limit = DefaultLimit
	}

	if !input.Requester.Actor.PlatformAdmin && member.Role != entities.RoleAdmin {
		self := input.Requester.Actor.UserID
		filter.ActorID = &self
	}

	// Pede uma linha a mais do que o limite: se ela vier, há próxima página e o
	// cursor é o id da última linha devolvida (a trilha anda do mais novo para o
	// mais antigo, e o UUIDv7 é ordenado no tempo).
	logs, err := u.Lister.List(ctx, filter, filter.Limit+1)
	if err != nil {
		internal := entities.InternalServerError(err, nil)
		return ListOutput{Error: &internal}
	}

	page := CursorPage{}
	if len(logs) > filter.Limit {
		logs = logs[:filter.Limit]
		next := logs[len(logs)-1].ID.String()
		page.NextCursor = &next
	}

	return ListOutput{Data: logs, Page: &page}
}
