package services

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

func ParsePath(r *http.Request) (application, member uuid.UUID, e *entities.Error) {
	if application, e = utils.PathUUID(r, "app"); e != nil {
		return
	}
	member, e = utils.PathMember(r)
	return
}

func ParseSecretPath(r *http.Request) (application, member, secret uuid.UUID, e *entities.Error) {
	if application, member, e = ParsePath(r); e != nil {
		return
	}
	secret, e = utils.PathUUID(r, "id")
	return
}

func ParseCreate(r *http.Request) (application, member uuid.UUID, items []entities.SecretDraft, e *entities.Error) {
	if application, member, e = ParsePath(r); e != nil {
		return
	}

	items, err := utils.BindJSON[entities.SecretDraft](r)
	if err != nil || len(items) == 0 {
		invalid := entities.InvalidRequestBodyError(errors.New("the request body must be a JSON object or list with known fields"))
		return uuid.UUID{}, uuid.UUID{}, nil, &invalid
	}

	return application, member, items, nil
}

type updateBody struct {
	Value string `json:"value"`
}

func ParseUpdate(r *http.Request) (application, member, secret uuid.UUID, value string, e *entities.Error) {
	if application, member, secret, e = ParseSecretPath(r); e != nil {
		return
	}
	body, e := utils.BindOne[updateBody](r)
	if e != nil {
		return
	}
	return application, member, secret, body.Value, nil
}

type renameBody struct {
	Name string `json:"name"`
}

func ParseRename(r *http.Request) (application, member, secret uuid.UUID, name string, e *entities.Error) {
	if application, member, secret, e = ParseSecretPath(r); e != nil {
		return
	}
	body, e := utils.BindOne[renameBody](r)
	if e != nil {
		return
	}
	return application, member, secret, body.Name, nil
}

type revealBody struct {
	Version int `json:"version"`
}

func ParseReveal(r *http.Request) (application, member, secret uuid.UUID, version int, e *entities.Error) {
	if application, member, secret, e = ParseSecretPath(r); e != nil {
		return
	}
	body, e := utils.BindOptional[revealBody](r)
	if e != nil {
		return
	}
	return application, member, secret, body.Version, nil
}
