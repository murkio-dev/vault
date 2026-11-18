package useCases

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	membercontracts "github.com/murkio-dev/vault/internal/restful/member/contracts"
	tokencontracts "github.com/murkio-dev/vault/internal/restful/token/contracts"
)

// Builder

type CreateTokenUseCase struct {
	Repository tokencontracts.ITokenRepository
	Members    membercontracts.IMemberRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
	PublicURL  string
}

func NewCreateTokenUseCase(repository tokencontracts.ITokenRepository, members membercontracts.IMemberRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx, publicURL string) CreateTokenUseCase {
	return CreateTokenUseCase{Repository: repository, Members: members, Guard: guard, Tx: tx, PublicURL: strings.TrimRight(publicURL, "/")}
}

// DTOS

type CreateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Name        string
	ExpiresAt   *time.Time
}

type CreateOutput struct {
	Data  entities.AccessTokenCreated `json:"data"`
	Error *entities.Error             `json:"error,omitempty"`
}

// UseCase

// Perform gera o token do membro. O token aparece só nesta resposta; o banco
// guarda o hash. Um membro suspenso não recebe token novo (409).
func (u CreateTokenUseCase) Perform(ctx context.Context, input CreateInput) CreateOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionTokenCreate)
	if denied != nil {
		return CreateOutput{Error: denied}
	}

	ctx = entities.WithActor(ctx, input.Requester.Actor)

	member, err := u.Members.FindById(ctx, input.Application, owner)
	if err != nil {
		return CreateOutput{Error: failure(err)}
	}
	if !member.Active() {
		conflict := entities.ConflictError("Member suspended", "A suspended member cannot get new tokens.")
		return CreateOutput{Error: &conflict}
	}

	token, raw, err := entities.NewAccessToken(member, input.Name, input.ExpiresAt)
	if err != nil {
		invalid := entities.ValidationError("The token is invalid.", entities.InvalidParam{Field: "name or expires_at", Reason: err.Error()})
		return CreateOutput{Error: &invalid}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditTokenCreated, entities.AuditAllowed,
		entities.AuditTarget{Type: "token", ID: token.ID.String()},
		map[string]any{"name": token.Name, "prefix": token.Prefix, "member_id": owner.String()})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		_, err := u.Repository.Create(ctx, token)
		return err
	})
	if err != nil {
		return CreateOutput{Error: failure(err)}
	}

	return CreateOutput{Data: entities.AccessTokenCreated{AccessToken: token, Token: raw, URI: u.PublicURL + "/" + token.UriID}}
}

func failure(err error) *entities.Error {
	if errors.Is(err, sql.ErrNoRows) {
		notFound := entities.NotFoundError("Resource")
		return &notFound
	}
	internal := entities.InternalServerError(err, nil)
	return &internal
}

// authorizeOwner autoriza a ação sobre a tabela de um membro, troca "me" pelo
// membro do ator e confere que o membro existe na aplicação. Devolve o dono
// resolvido.
func authorizeOwner(guard contracts.IAccessGuard, ctx context.Context, requester entities.ApiHeader, application, member uuid.UUID, action entities.Action) (uuid.UUID, *entities.Error) {
	output := guard.Perform(ctx, entities.AuthorizeInput{Requester: requester, Application: application, Action: action, Owner: member, OwnerMustExist: true})
	return output.Owner, output.Error
}
