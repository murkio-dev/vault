package utils

import (
	"errors"
	"net/http"
	"strconv"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

func PathUUID(r *http.Request, name string) (uuid.UUID, *entities.Error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil || id == (uuid.UUID{}) {
		invalid := entities.ValidationError("The path parameter "+name+" is invalid.",
			entities.InvalidParam{Field: name, Reason: "must be a valid, non-empty uuid"})
		return uuid.UUID{}, &invalid
	}
	return id, nil
}

// PathMember lê o {member} do caminho: um UUID, ou a palavra "me", que vira
// entities.SelfOwner. O marcador viaja até o use case de autorização, que o
// troca pelo membro do próprio ator, então "meus segredos" e "os segredos do
// membro X" passam pela mesma regra de permissão. Qualquer outro texto é erro
// de validação que não repete o valor recebido.
// Exemplo: ".../members/me/secrets" devolve entities.SelfOwner.
func PathMember(r *http.Request) (uuid.UUID, *entities.Error) {
	if r.PathValue("member") == "me" {
		return entities.SelfOwner, nil
	}
	return PathUUID(r, "member")
}

// BindOne lê o corpo como JSON estrito (BindJSON) e exige exatamente um item.
// Serve às rotas que operam num recurso só: um objeto vale, uma lista de um
// item também (a regra de lote do api.md), e uma lista de dois ou mais é erro
// de validação. O erro devolvido nunca repete o corpo, que pode ter segredo.
// Exemplo: {"role":"admin"} e [{"role":"admin"}] valem; [{...},{...}] falha.
func BindOne[T any](r *http.Request) (T, *entities.Error) {
	var zero T

	items, err := BindJSON[T](r)
	if err != nil {
		invalid := entities.InvalidRequestBodyError(errors.New("the request body must be a JSON object with known fields"))
		return zero, &invalid
	}
	if len(items) != 1 {
		invalid := entities.InvalidRequestBodyError(errors.New("expected exactly one object"))
		return zero, &invalid
	}

	return items[0], nil
}

func Pagination(r *http.Request) (page, limit int, e *entities.Error) {
	page, limit = 1, entities.MaxPageSize

	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			invalid := entities.ValidationError("The query parameter page is invalid.", entities.InvalidParam{Field: "page", Reason: "must be a positive integer"})
			return 0, 0, &invalid
		}
		page = parsed
	}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			invalid := entities.ValidationError("The query parameter limit is invalid.", entities.InvalidParam{Field: "limit", Reason: "must be a positive integer"})
			return 0, 0, &invalid
		}
		limit = parsed
	}

	return page, limit, nil
}

func PageWindow(page, limit int) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > entities.MaxPageSize {
		limit = entities.MaxPageSize
	}
	return page, limit, (page - 1) * limit
}

func BindOptional[T any](r *http.Request) (T, *entities.Error) {
	var zero T

	if r.Body == nil || r.ContentLength == 0 {
		return zero, nil
	}

	return BindOne[T](r)
}
