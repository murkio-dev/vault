package services

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

func ParseApplications(r *http.Request) ([]entities.Application, *entities.Error) {
	applications, err := utils.BindBody[entities.Application](r)
	if err != nil {
		e := entities.InvalidRequestBodyError(err)
		return nil, &e
	}
	if len(applications) == 0 {
		e := entities.InvalidRequestBodyError(errors.New("empty payload"))
		return nil, &e
	}
	return applications, nil
}

func ParseApplicationsForChange(r *http.Request) ([]entities.Application, *entities.Error) {
	applications, e := ParseApplications(r)
	if e != nil {
		return nil, e
	}

	if r.PathValue("id") == "" {
		return applications, nil
	}

	id, e := ParsePathID(r)
	if e != nil {
		return nil, e
	}
	if len(applications) != 1 {
		e := entities.InvalidRequestBodyError(errors.New("expected exactly one application"))
		return nil, &e
	}
	applications[0].ID = id

	return applications, nil
}

func ParsePathID(r *http.Request) (uuid.UUID, *entities.Error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		e := entities.InvalidApplicationIDFormatError(err)
		return uuid.UUID{}, &e
	}
	return id, nil
}

func ParseIDs(r *http.Request) ([]uuid.UUID, *entities.Error) {
	if r.PathValue("id") != "" {
		id, e := ParsePathID(r)
		if e != nil {
			return nil, e
		}
		return []uuid.UUID{id}, nil
	}

	ids, err := utils.BindIDs(r)
	if err != nil {
		e := entities.InvalidRequestBodyError(err)
		return nil, &e
	}
	return ids, nil
}

func ParsePagination(r *http.Request) (page, limit int, e *entities.Error) {
	page, limit = 1, entities.MaxPageSize

	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			e := entities.InvalidPageError(fmt.Errorf("invalid page %q", raw))
			return 0, 0, &e
		}
		page = parsed
	}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			e := entities.InvalidLimitError(fmt.Errorf("invalid limit %q", raw))
			return 0, 0, &e
		}
		limit = parsed
	}

	return page, limit, nil
}

func ApplicationIDs(applications []entities.Application) []uuid.UUID {
	ids := make([]uuid.UUID, len(applications))
	for i, application := range applications {
		ids[i] = application.ID
	}
	return ids
}

func DuplicatedIDs(ids []uuid.UUID) map[uuid.UUID]bool {
	seen := make(map[uuid.UUID]int, len(ids))
	for _, id := range ids {
		seen[id]++
	}

	duplicated := make(map[uuid.UUID]bool)
	for id, count := range seen {
		if count > 1 {
			duplicated[id] = true
		}
	}
	return duplicated
}
