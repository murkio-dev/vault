package useCases

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

var (
	errHasReferences = errors.New("secret is referenced")
	errTableFull     = errors.New("secret table is full")
)

// failure traduz o erro de um repositório na resposta da API. O erro original
// nunca vai para o cliente (nem o texto do driver, nem nada que o valor tenha
// tocado).
func failure(err error) *entities.Error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		notFound := entities.NotFoundError("Resource")
		return &notFound
	case sqlite.IsUniqueViolation(err):
		conflict := entities.ConflictError("Secret already exists", "This member already has a secret with that name.",
			entities.InvalidParam{Field: "name", Reason: "already in use"})
		return &conflict
	case errors.Is(err, errTableFull):
		conflict := entities.ConflictError("Secret table is full", fmt.Sprintf("A member can have at most %d secrets.", entities.MaxSecretsPerTable))
		return &conflict
	case errors.Is(err, errHasReferences):
		conflict := entities.ConflictError("Secret is referenced", "Other secrets point to this one. Delete or change them first.")
		return &conflict
	}
	internal := entities.InternalServerError(err, nil)
	return &internal
}

func invalid(field, reason string) *entities.Error {
	e := entities.ValidationError("The field "+field+" is invalid.", entities.InvalidParam{Field: field, Reason: reason})
	return &e
}

// authorizeOwner é o primeiro passo de toda ação sobre a tabela de um membro:
// autoriza a ação sobre o dono, troca "me" pelo membro do ator e confere que o
// membro existe na aplicação. Devolve o dono resolvido.
func authorizeOwner(guard contracts.IAccessGuard, ctx context.Context, requester entities.ApiHeader, application, member uuid.UUID, action entities.Action) (uuid.UUID, *entities.Error) {
	output := guard.Perform(ctx, entities.AuthorizeInput{Requester: requester, Application: application, Action: action, Owner: member, OwnerMustExist: true})
	return output.Owner, output.Error
}
