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

func ParseList(r *http.Request) (application uuid.UUID, page, limit int, e *entities.Error) {
	if application, e = utils.PathUUID(r, "app"); e != nil {
		return
	}
	page, limit, e = utils.Pagination(r)
	return
}

func ParseMember(r *http.Request) (application, member uuid.UUID, e *entities.Error) {
	if application, e = utils.PathUUID(r, "app"); e != nil {
		return
	}
	member, e = utils.PathUUID(r, "member")
	return
}

type updateBody struct {
	Role   *entities.Role         `json:"role"`
	Status *entities.MemberStatus `json:"status"`
}

func ParseUpdate(r *http.Request) (application, member uuid.UUID, role *entities.Role, status *entities.MemberStatus, e *entities.Error) {
	if application, member, e = ParseMember(r); e != nil {
		return
	}
	body, e := utils.BindOne[updateBody](r)
	if e != nil {
		return
	}
	return application, member, body.Role, body.Status, nil
}
