package services_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/services"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê a aplicação e os filtros.", func(t *testing.T) { whenParsingApplicationAndFilters(t) })
	t.Run("Quando não há filtros.", func(t *testing.T) { whenThereAreNoFilters(t) })
	t.Run("Quando algum filtro é inválido.", func(t *testing.T) { whenAFilterIsInvalid(t) })
	t.Run("Quando lê o pedido gRPC com filtros.", func(t *testing.T) { whenParsingGRPCRequest(t) })
	t.Run("Quando o pedido gRPC não tem filtros.", func(t *testing.T) { whenGRPCRequestHasNoFilters(t) })
	t.Run("Quando o pedido gRPC é inválido.", func(t *testing.T) { whenGRPCRequestIsInvalid(t) })
	t.Run("Quando o cursor é válido.", func(t *testing.T) { whenCursorIsValid(t) })
}

// Functions

func whenParsingApplicationAndFilters(t *testing.T) {
	app, actor := uuid.NewV7(), uuid.NewV7()
	req := httptest.NewRequest("GET", "/?limit=5&actor_id="+actor.String()+"&outcome=allowed&action=env.read&from=2026-01-01T00:00:00Z", nil)
	req.SetPathValue("app", app.String())

	gotApp, filter, e := services.ParseList(req)

	assert.Nil(t, e)
	assert.Equal(t, app, gotApp)
	assert.Equal(t, 5, filter.Limit)
	assert.Equal(t, entities.AuditAllowed, filter.Outcome)
	assert.Equal(t, "env.read", filter.Action)
	if assert.NotNil(t, filter.ActorID) {
		assert.Equal(t, actor, *filter.ActorID)
	}
	assert.NotNil(t, filter.From)
	assert.Nil(t, filter.To)
}

func whenThereAreNoFilters(t *testing.T) {
	filter, e := services.ParseFilter(httptest.NewRequest("GET", "/", nil))

	assert.Nil(t, e)
	assert.Equal(t, entities.AuditFilter{}, filter)
}

func whenAFilterIsInvalid(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("app", "x")
	_, _, badApp := services.ParseList(req)
	assert.NotNil(t, badApp)

	for _, query := range []string{"limit=0", "limit=x", "cursor=x", "actor_id=x", "outcome=talvez", "from=ontem", "to=2020", "action=" + strings.Repeat("a", 65), "target_id=" + strings.Repeat("a", 65)} {
		_, e := services.ParseFilter(httptest.NewRequest("GET", "/?"+query, nil))
		if assert.NotNil(t, e, query) {
			assert.Equal(t, entities.ErrTypeValidation, e.Type, query)
		}
	}
}

func whenParsingGRPCRequest(t *testing.T) {
	app, actor, cursor := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	gotApp, filter, e := services.ParseGRPCList(&vaultv1.ListAuditLogsRequest{
		ApplicationId: app.String(), Limit: 5, Cursor: cursor.String(), ActorId: actor.String(), Outcome: "denied",
		Action: "env.read", TargetId: "alvo", From: timestamppb.New(from), To: timestamppb.New(to),
	})

	assert.Nil(t, e)
	assert.Equal(t, app, gotApp)
	assert.Equal(t, 5, filter.Limit)
	assert.Equal(t, cursor, filter.Cursor)
	assert.Equal(t, entities.AuditDenied, filter.Outcome)
	assert.Equal(t, "env.read", filter.Action)
	assert.Equal(t, "alvo", filter.TargetID)
	if assert.NotNil(t, filter.ActorID) {
		assert.Equal(t, actor, *filter.ActorID)
	}
	if assert.NotNil(t, filter.From) && assert.NotNil(t, filter.To) {
		assert.True(t, from.Equal(*filter.From))
		assert.True(t, to.Equal(*filter.To))
	}
	assert.Equal(t, uuid.UUID{}, filter.ApplicationID, "a aplicação do filtro é posta pelo use case, depois de autorizar")
}

func whenGRPCRequestHasNoFilters(t *testing.T) {
	app := uuid.NewV7()

	gotApp, filter, e := services.ParseGRPCList(&vaultv1.ListAuditLogsRequest{ApplicationId: app.String()})

	assert.Nil(t, e)
	assert.Equal(t, app, gotApp)
	assert.Equal(t, entities.AuditFilter{}, filter, "campo ausente não filtra")
}

func whenGRPCRequestIsInvalid(t *testing.T) {
	app := uuid.NewV7().String()
	tooLong := strings.Repeat("a", 65)

	requests := map[string]*vaultv1.ListAuditLogsRequest{
		"application_id": {ApplicationId: "nao-e-uuid"},
		"sem aplicação":  {},
		"limit":          {ApplicationId: app, Limit: -1},
		"cursor":         {ApplicationId: app, Cursor: "' OR 1=1 --"},
		"actor_id":       {ApplicationId: app, ActorId: "nao-e-uuid"},
		"outcome":        {ApplicationId: app, Outcome: "talvez"},
		"action":         {ApplicationId: app, Action: tooLong},
		"target_id":      {ApplicationId: app, TargetId: tooLong},
	}
	for name, request := range requests {
		gotApp, filter, e := services.ParseGRPCList(request)

		assert.Equal(t, uuid.UUID{}, gotApp, name)
		assert.Equal(t, entities.AuditFilter{}, filter, name)
		if assert.NotNil(t, e, name) {
			assert.Equal(t, entities.ErrTypeValidation, e.Type, name)
			assert.NotContains(t, fmt.Sprint(e.Detail, e.InvalidParams), "OR 1=1", "o erro não repete o valor recebido")
		}
	}

	_, atLimit, e := services.ParseGRPCList(&vaultv1.ListAuditLogsRequest{ApplicationId: app, Action: strings.Repeat("a", 64)})
	assert.Nil(t, e, "64 bytes é o teto, e vale")
	assert.Len(t, atLimit.Action, 64)
}

func whenCursorIsValid(t *testing.T) {
	cursor := uuid.NewV7()

	filter, e := services.ParseFilter(httptest.NewRequest("GET", "/?cursor="+cursor.String(), nil))

	assert.Nil(t, e)
	assert.Equal(t, cursor, filter.Cursor)
}
