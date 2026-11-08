package services

import (
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

func ParseList(r *http.Request) (uuid.UUID, entities.AuditFilter, *entities.Error) {
	application, e := utils.PathUUID(r, "app")
	if e != nil {
		return uuid.UUID{}, entities.AuditFilter{}, e
	}
	filter, e := ParseFilter(r)
	return application, filter, e
}

func ParseFilter(r *http.Request) (entities.AuditFilter, *entities.Error) {
	query := r.URL.Query()
	var filter entities.AuditFilter

	bad := func(field, reason string) (entities.AuditFilter, *entities.Error) {
		e := entities.ValidationError("The query parameter "+field+" is invalid.", entities.InvalidParam{Field: field, Reason: reason})
		return entities.AuditFilter{}, &e
	}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return bad("limit", "must be a positive integer")
		}
		filter.Limit = limit
	}
	if raw := query.Get("cursor"); raw != "" {
		cursor, err := uuid.Parse(raw)
		if err != nil {
			return bad("cursor", "is not a valid cursor")
		}
		filter.Cursor = cursor
	}
	if raw := query.Get("actor_id"); raw != "" {
		actor, err := uuid.Parse(raw)
		if err != nil {
			return bad("actor_id", "must be a uuid")
		}
		filter.ActorID = &actor
	}
	if raw := query.Get("outcome"); raw != "" {
		outcome := entities.AuditOutcome(raw)
		if outcome != entities.AuditAllowed && outcome != entities.AuditDenied {
			return bad("outcome", "must be allowed or denied")
		}
		filter.Outcome = outcome
	}
	for _, field := range []struct {
		name string
		dest *string
		max  int
	}{{"action", &filter.Action, 64}, {"target_id", &filter.TargetID, 64}} {
		value := query.Get(field.name)
		if len(value) > field.max {
			return bad(field.name, "is too long")
		}
		*field.dest = value
	}
	for _, field := range []struct {
		name string
		dest **time.Time
	}{{"from", &filter.From}, {"to", &filter.To}} {
		if raw := query.Get(field.name); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return bad(field.name, "must be an RFC 3339 date")
			}
			*field.dest = &parsed
		}
	}

	return filter, nil
}
