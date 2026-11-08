package services

import (
	"net/http"
	"time"
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

func ParseRevoke(r *http.Request) (application, member, token uuid.UUID, e *entities.Error) {
	if application, member, e = ParsePath(r); e != nil {
		return
	}
	token, e = utils.PathUUID(r, "id")
	return
}

type createBody struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func ParseCreate(r *http.Request) (application, member uuid.UUID, name string, expiresAt *time.Time, e *entities.Error) {
	if application, member, e = ParsePath(r); e != nil {
		return
	}
	body, e := utils.BindOne[createBody](r)
	if e != nil {
		return
	}
	return application, member, body.Name, body.ExpiresAt, nil
}
