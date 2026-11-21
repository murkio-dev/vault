package useCases

import (
	"context"
	"strings"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
	membercontracts "github.com/murkio-dev/vault/internal/restful/member/contracts"
)

// Builder

type RotateApiKeyByIDUseCase struct {
	Repository apikeyscontracts.IApiKeyRepository
	Members    membercontracts.IMemberRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
	PublicURL  string
	Now        func() time.Time
}

func NewRotateApiKeyByIDUseCase(repository apikeyscontracts.IApiKeyRepository, members membercontracts.IMemberRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx, publicURL string) RotateApiKeyByIDUseCase {
	return RotateApiKeyByIDUseCase{Repository: repository, Members: members, Guard: guard, Tx: tx, PublicURL: strings.TrimRight(publicURL, "/"), Now: time.Now}
}

// DTOS

type RotateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	ID          uuid.UUID
}

type RotateOutput struct {
	Data  entities.ApiKeyCreated `json:"data"`
	Error *entities.Error        `json:"error,omitempty"`
}

// UseCase

// Perform gera uma chave nova com o mesmo nome e tipo e faz a antiga vencer em
// entities.RotationOverlap (72h), como no Identity, para dar tempo de trocar a
// chave onde ela está configurada. A nova aparece só nesta resposta. Chave
// apagada é 404; membro suspenso é 409.
func (u RotateApiKeyByIDUseCase) Perform(ctx context.Context, input RotateInput) RotateOutput {

	current, e := ownedKey(u.Guard, u.Repository, ctx, input.Requester, input.Application, input.Member, input.ID, entities.ActionApiKeyCreate)
	if e != nil {
		return RotateOutput{Error: e}
	}
	if current.DeletedAt != nil {
		notFound := entities.NotFoundError("Resource")
		return RotateOutput{Error: &notFound}
	}

	ctx = entities.WithActor(ctx, input.Requester.Actor)

	member, e := activeMember(ctx, u.Members, input.Application, current.MemberID)
	if e != nil {
		return RotateOutput{Error: e}
	}

	next, plain, err := entities.NewApiKey(member, current.Name, current.Type, nil)
	if err != nil {
		return RotateOutput{Error: failure(err)}
	}
	deadline := current.RotationDeadline(u.Now().UTC())

	// Só ids, prefixos e o novo prazo da antiga vão para o log: o valor da chave
	// nunca.
	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditApiKeyRotated, entities.AuditAllowed,
		entities.AuditTarget{Type: "api_key", ID: current.ID.String()},
		map[string]any{"type": current.Type, "prefix": current.Prefix, "new_api_key_id": next.ID.String(), "new_prefix": next.Prefix,
			"previous_expires_at": deadline, "member_id": current.MemberID.String()})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		if _, err := u.Repository.Create(ctx, next); err != nil {
			return err
		}
		return u.Repository.Expire(ctx, input.Application, current.ID, deadline)
	})
	if err != nil {
		return RotateOutput{Error: failure(err)}
	}

	return RotateOutput{Data: created(next, plain, u.PublicURL)}
}
