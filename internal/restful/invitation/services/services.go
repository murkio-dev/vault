package services

import (
	"net/http"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

func ParseApplication(r *http.Request) (uuid.UUID, *entities.Error) {
	return utils.PathUUID(r, "app")
}

func ParseRevoke(r *http.Request) (application, invitation uuid.UUID, e *entities.Error) {
	if application, e = utils.PathUUID(r, "app"); e != nil {
		return
	}
	invitation, e = utils.PathUUID(r, "id")
	return
}

type createBody struct {
	Email string        `json:"email"`
	Role  entities.Role `json:"role"`
}

func ParseCreate(r *http.Request) (application uuid.UUID, email string, role entities.Role, e *entities.Error) {
	if application, e = utils.PathUUID(r, "app"); e != nil {
		return
	}
	body, e := utils.BindOne[createBody](r)
	if e != nil {
		return
	}
	return application, body.Email, body.Role, nil
}

type acceptBody struct {
	Token string `json:"token"`
}

func ParseAccept(r *http.Request) (string, *entities.Error) {
	body, e := utils.BindOne[acceptBody](r)
	if e != nil {
		return "", e
	}
	return body.Token, nil
}
