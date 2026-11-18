package services

import (
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/grpcx"
	"github.com/murkio-dev/vault/pkg/httpx"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
)

const maxFilterText = 64

func ParseList(r *http.Request) (uuid.UUID, entities.AuditFilter, *entities.Error) {
	application, e := httpx.PathUUID(r, "app")
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
	}{{"action", &filter.Action, maxFilterText}, {"target_id", &filter.TargetID, maxFilterText}} {
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

// ParseGRPCList lê a aplicação e os filtros de um pedido gRPC da trilha, com
// as mesmas regras do ParseList da entrega HTTP, para as duas entregas
// recusarem os mesmos pedidos.
// Recebe o pedido como chegou. No proto3 o campo ausente chega vazio ou zero, e
// vazio ou zero significa "sem esse filtro" (limit zero usa o padrão do use case).
// Devolve a aplicação e o filtro, ou o erro de validação do primeiro campo
// inválido: application_id e actor_id têm de ser uuid, cursor um uuid, outcome
// "allowed" ou "denied", action e target_id com até 64 bytes e limit não negativo.
// Exemplo: outcome "denied" e limit 10 viram AuditFilter{Outcome: AuditDenied,
// Limit: 10}; outcome "talvez" devolve erro no campo outcome.
// O erro nunca repete o valor recebido.
func ParseGRPCList(req *vaultv1.ListAuditLogsRequest) (uuid.UUID, entities.AuditFilter, *entities.Error) {

	bad := func(field, reason string) (uuid.UUID, entities.AuditFilter, *entities.Error) {
		e := entities.ValidationError("The field "+field+" is invalid.", entities.InvalidParam{Field: field, Reason: reason})
		return uuid.UUID{}, entities.AuditFilter{}, &e
	}

	application, e := grpcx.UUID(req.GetApplicationId(), "application_id")
	if e != nil {
		return uuid.UUID{}, entities.AuditFilter{}, e
	}

	if req.GetLimit() < 0 {
		return bad("limit", "must be a positive integer")
	}
	filter := entities.AuditFilter{Limit: int(req.GetLimit())}

	if raw := req.GetCursor(); raw != "" {
		cursor, err := uuid.Parse(raw)
		if err != nil {
			return bad("cursor", "is not a valid cursor")
		}
		filter.Cursor = cursor
	}
	if raw := req.GetActorId(); raw != "" {
		actor, err := uuid.Parse(raw)
		if err != nil {
			return bad("actor_id", "must be a uuid")
		}
		filter.ActorID = &actor
	}
	if raw := req.GetOutcome(); raw != "" {
		outcome := entities.AuditOutcome(raw)
		if outcome != entities.AuditAllowed && outcome != entities.AuditDenied {
			return bad("outcome", "must be allowed or denied")
		}
		filter.Outcome = outcome
	}
	if len(req.GetAction()) > maxFilterText {
		return bad("action", "is too long")
	}
	if len(req.GetTargetId()) > maxFilterText {
		return bad("target_id", "is too long")
	}
	filter.Action, filter.TargetID = req.GetAction(), req.GetTargetId()

	if req.GetFrom() != nil {
		from := req.GetFrom().AsTime()
		filter.From = &from
	}
	if req.GetTo() != nil {
		to := req.GetTo().AsTime()
		filter.To = &to
	}

	return application, filter, nil
}
