package useCases

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
)

// Builder

type UpdateMemberUseCase struct {
	Finder       contracts.IMemberFinder
	Upgrader     contracts.IMemberUpgrader
	AdminCounter contracts.IMemberAdminCounter
	Guard        contracts.IAccessGuard
	Tx           contracts.IAuditedTx
}

func NewUpdateMemberUseCase(finder contracts.IMemberFinder, upgrader contracts.IMemberUpgrader, adminCounter contracts.IMemberAdminCounter, guard contracts.IAccessGuard, tx contracts.IAuditedTx) UpdateMemberUseCase {
	return UpdateMemberUseCase{Finder: finder, Upgrader: upgrader, AdminCounter: adminCounter, Guard: guard, Tx: tx}
}

// DTOS

type UpdateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Role        *entities.Role
	Status      *entities.MemberStatus
}

type UpdateOutput struct {
	Data  entities.Member `json:"data"`
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform muda o papel e/ou o status de um membro. Não deixa a aplicação sem
// admin ativo: rebaixar ou suspender o último admin é conflito (409). A
// contagem e a mudança estão na mesma transação, então dois pedidos
// simultâneos não conseguem tirar os dois últimos admins.
func (u UpdateMemberUseCase) Perform(ctx context.Context, input UpdateInput) UpdateOutput {

	if input.Role == nil && input.Status == nil {
		invalid := entities.ValidationError("Send at least one of role or status.")
		return UpdateOutput{Error: &invalid}
	}
	if input.Role != nil && !input.Role.Valid() {
		invalid := entities.ValidationError("The field role is invalid.", entities.InvalidParam{Field: "role", Reason: "must be admin or developer"})
		return UpdateOutput{Error: &invalid}
	}
	if input.Status != nil && !input.Status.Valid() {
		invalid := entities.ValidationError("The field status is invalid.", entities.InvalidParam{Field: "status", Reason: "must be active or suspended"})
		return UpdateOutput{Error: &invalid}
	}

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionMemberUpdate}).Error; denied != nil {
		return UpdateOutput{Error: denied}
	}

	metadata := map[string]any{}
	if input.Role != nil {
		metadata["role"] = string(*input.Role)
	}
	if input.Status != nil {
		metadata["status"] = string(*input.Status)
	}
	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditMemberUpdated, entities.AuditAllowed,
		entities.AuditTarget{Type: "member", ID: input.Member.String()}, metadata)

	var updated entities.Member
	err := u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		member, err := u.Finder.FindById(ctx, input.Application, input.Member)
		if err != nil {
			return err
		}

		wasAdmin := member.Role == entities.RoleAdmin && member.Active()
		change := entities.Member{ID: member.ID, ApplicationID: member.ApplicationID}
		if input.Role != nil {
			change.Role, member.Role = *input.Role, *input.Role
		}
		if input.Status != nil {
			change.Status, member.Status = *input.Status, *input.Status
		}

		if wasAdmin && !(member.Role == entities.RoleAdmin && member.Active()) {
			if err := u.ensureAnotherAdmin(ctx, input.Application); err != nil {
				return err
			}
		}

		updated, err = u.Upgrader.Upgrade(ctx, change)
		return err
	})
	if err != nil {
		return UpdateOutput{Error: failure(err)}
	}

	return UpdateOutput{Data: updated}
}

// ensureAnotherAdmin conta os admins ativos da aplicação (CountActiveAdmins) e recusa se este for o único.
func (u UpdateMemberUseCase) ensureAnotherAdmin(ctx context.Context, application uuid.UUID) error {
	return ensureAnotherAdmin(ctx, u.AdminCounter, application)
}

func ensureAnotherAdmin(ctx context.Context, counter contracts.IMemberAdminCounter, application uuid.UUID) error {
	admins, err := counter.CountActiveAdmins(ctx, application)
	if err != nil {
		return err
	}
	if admins <= 1 {
		return errLastAdmin
	}
	return nil
}

var errLastAdmin = errors.New("last admin")

func failure(err error) *entities.Error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		notFound := entities.NotFoundError("Resource")
		return &notFound
	case errors.Is(err, errLastAdmin):
		conflict := entities.ConflictError("Last admin", "An application needs at least one active admin.")
		return &conflict
	}
	internal := entities.InternalServerError(err, nil)
	return &internal
}
