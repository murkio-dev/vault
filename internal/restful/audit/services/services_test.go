package services_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê a aplicação e os filtros.", func(t *testing.T) { whenParsingApplicationAndFilters(t) })
	t.Run("Quando não há filtros.", func(t *testing.T) { whenThereAreNoFilters(t) })
	t.Run("Quando algum filtro é inválido.", func(t *testing.T) { whenAFilterIsInvalid(t) })
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
