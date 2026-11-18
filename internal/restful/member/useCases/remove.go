package useCases

import (
	"context"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	membercontracts "github.com/murkio-dev/vault/internal/restful/member/contracts"
)

// Builder

type RemoveMemberUseCase struct {
	Repository membercontracts.IMemberRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
}

func NewRemoveMemberUseCase(repository membercontracts.IMemberRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx) RemoveMemberUseCase {
	return RemoveMemberUseCase{Repository: repository, Guard: guard, Tx: tx}
}

// DTOS

type RemoveInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
}

type RemoveOutput struct {
	Error *entities.Error `json:"error,omitempty"`
}

// UseCase

// Perform remove o membro: os tokens dele são revogados e os segredos apagados
// (as versões cifradas e a auditoria ficam). Não remove o último admin.
func (u RemoveMemberUseCase) Perform(ctx context.Context, input RemoveInput) RemoveOutput {

	if denied := u.Guard.Perform(ctx, entities.AuthorizeInput{Requester: input.Requester, Application: input.Application, Action: entities.ActionMemberRemove}).Error; denied != nil {
		return RemoveOutput{Error: denied}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditMemberRemoved, entities.AuditAllowed,
		entities.AuditTarget{Type: "member", ID: input.Member.String()}, nil)

	err := u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		member, err := u.Repository.FindById(ctx, input.Application, input.Member)
		if err != nil {
			return err
		}

		if member.Role == entities.RoleAdmin && member.Active() {
			if err := ensureAnotherAdmin(ctx, u.Repository, input.Application); err != nil {
				return err
			}
		}

		return u.Repository.Delete(ctx, input.Application, input.Member)
	})
	if err != nil {
		return RemoveOutput{Error: failure(err)}
	}

	return RemoveOutput{}
}
