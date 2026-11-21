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
	apikeyscontracts "github.com/murkio-dev/vault/internal/restful/apikeys/contracts"
	membercontracts "github.com/murkio-dev/vault/internal/restful/member/contracts"
)

// Builder

type CreateApiKeyUseCase struct {
	Repository apikeyscontracts.IApiKeyRepository
	Members    membercontracts.IMemberRepository
	Guard      contracts.IAccessGuard
	Tx         contracts.IAuditedTx
	PublicURL  string
}

func NewCreateApiKeyUseCase(repository apikeyscontracts.IApiKeyRepository, members membercontracts.IMemberRepository, guard contracts.IAccessGuard, tx contracts.IAuditedTx, publicURL string) CreateApiKeyUseCase {
	return CreateApiKeyUseCase{Repository: repository, Members: members, Guard: guard, Tx: tx, PublicURL: strings.TrimRight(publicURL, "/")}
}

// DTOS

type CreateInput struct {
	Requester   entities.ApiHeader
	Application uuid.UUID
	Member      uuid.UUID
	Name        string
	Type        entities.KeyType
	ExpiresAt   *time.Time
}

type CreateOutput struct {
	Data  entities.ApiKeyCreated `json:"data"`
	Error *entities.Error        `json:"error,omitempty"`
}

// UseCase

// Perform gera a chave do membro. O valor aparece só nesta resposta; o banco
// guarda o hash. Um membro suspenso não recebe chave nova (409).
func (u CreateApiKeyUseCase) Perform(ctx context.Context, input CreateInput) CreateOutput {

	owner, denied := authorizeOwner(u.Guard, ctx, input.Requester, input.Application, input.Member, entities.ActionApiKeyCreate)
	if denied != nil {
		return CreateOutput{Error: denied}
	}

	ctx = entities.WithActor(ctx, input.Requester.Actor)

	member, e := activeMember(ctx, u.Members, input.Application, owner)
	if e != nil {
		return CreateOutput{Error: e}
	}

	key, plain, err := entities.NewApiKey(member, input.Name, input.Type, input.ExpiresAt)
	if err != nil {
		invalid := entities.ValidationError("The api key is invalid.", entities.InvalidParam{Field: "name, type or expires_at", Reason: err.Error()})
		return CreateOutput{Error: &invalid}
	}

	entry := entities.NewAuditLog(input.Requester, input.Application, entities.AuditApiKeyCreated, entities.AuditAllowed,
		entities.AuditTarget{Type: "api_key", ID: key.ID.String()},
		map[string]any{"name": key.Name, "type": key.Type, "prefix": key.Prefix, "member_id": owner.String()})

	err = u.Tx.Run(ctx, entry, func(ctx context.Context) error {
		_, err := u.Repository.Create(ctx, key)
		return err
	})
	if err != nil {
		return CreateOutput{Error: failure(err)}
	}

	return CreateOutput{Data: created(key, plain, u.PublicURL)}
}

func created(key entities.ApiKey, plain, publicURL string) entities.ApiKeyCreated {
	return entities.ApiKeyCreated{ApiKey: key, Key: plain, URI: publicURL + "/" + key.UriID}
}

func failure(err error) *entities.Error {
	if errors.Is(err, sql.ErrNoRows) {
		notFound := entities.NotFoundError("Resource")
		return &notFound
	}
	internal := entities.InternalServerError(err, nil)
	return &internal
}

// authorizeOwner autoriza a ação sobre as chaves de um membro, troca "me" pelo
// membro do ator e confere que o membro existe na aplicação. Devolve o dono
// resolvido.
func authorizeOwner(guard contracts.IAccessGuard, ctx context.Context, requester entities.ApiHeader, application, member uuid.UUID, action entities.Action) (uuid.UUID, *entities.Error) {
	output := guard.Perform(ctx, entities.AuthorizeInput{Requester: requester, Application: application, Action: action, Owner: member, OwnerMustExist: true})
	return output.Owner, output.Error
}

// ownedKey autoriza a ação e acha a chave do membro. A chave tem de ser do
// membro pedido (senão 404), então um developer não mexe na de outro
// apontando o id.
func ownedKey(guard contracts.IAccessGuard, repository apikeyscontracts.IApiKeyRepository, ctx context.Context, requester entities.ApiHeader, application, member, id uuid.UUID, action entities.Action) (entities.ApiKey, *entities.Error) {
	owner, denied := authorizeOwner(guard, ctx, requester, application, member, action)
	if denied != nil {
		return entities.ApiKey{}, denied
	}

	key, err := repository.FindById(ctx, application, id)
	if err != nil || key.MemberID != owner {
		notFound := entities.NotFoundError("Resource")
		return entities.ApiKey{}, &notFound
	}

	return key, nil
}

// activeMember acha o membro e recusa o suspenso (409): ele não recebe chave
// nova, nem por rotação.
func activeMember(ctx context.Context, members membercontracts.IMemberRepository, application, member uuid.UUID) (entities.Member, *entities.Error) {
	found, err := members.FindById(ctx, application, member)
	if err != nil {
		return entities.Member{}, failure(err)
	}
	if !found.Active() {
		conflict := entities.ConflictError("Member suspended", "A suspended member cannot get new api keys.")
		return entities.Member{}, &conflict
	}
	return found, nil
}
