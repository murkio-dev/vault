package services_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/token/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê o caminho.", func(t *testing.T) { whenParsingThePath(t) })
	t.Run("Quando lê a revogação.", func(t *testing.T) { whenParsingTheRevoke(t) })
	t.Run("Quando lê a criação.", func(t *testing.T) { whenParsingTheCreate(t) })
}

// Functions

func request(body string, path map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	for name, value := range path {
		req.SetPathValue(name, value)
	}
	return req
}

func whenParsingThePath(t *testing.T) {
	app, member := uuid.NewV7(), uuid.NewV7()

	gotApp, gotMember, e := services.ParsePath(request("", map[string]string{"app": app.String(), "member": member.String()}))
	_, me, _ := services.ParsePath(request("", map[string]string{"app": app.String(), "member": "me"}))
	_, _, bad := services.ParsePath(request("", map[string]string{"app": "x", "member": "me"}))

	assert.Nil(t, e)
	assert.Equal(t, [2]uuid.UUID{app, member}, [2]uuid.UUID{gotApp, gotMember})
	assert.Equal(t, entities.SelfOwner, me)
	assert.NotNil(t, bad)
}

func whenParsingTheRevoke(t *testing.T) {
	path := map[string]string{"app": uuid.NewV7().String(), "member": "me", "id": uuid.NewV7().String()}

	_, _, token, e := services.ParseRevoke(request("", path))
	_, _, _, bad := services.ParseRevoke(request("", map[string]string{"app": path["app"], "member": "me", "id": "x"}))

	assert.Nil(t, e)
	assert.Equal(t, path["id"], token.String())
	assert.NotNil(t, bad)
}

func whenParsingTheCreate(t *testing.T) {
	path := map[string]string{"app": uuid.NewV7().String(), "member": "me"}

	_, _, name, expires, e := services.ParseCreate(request(`{"name":"ci","expires_at":"2099-01-01T00:00:00Z"}`, path))
	_, _, _, none, e2 := services.ParseCreate(request(`{"name":"ci"}`, path))
	_, _, _, _, badDate := services.ParseCreate(request(`{"name":"ci","expires_at":"amanhã"}`, path))
	_, _, _, _, unknown := services.ParseCreate(request(`{"name":"ci","x":1}`, path))

	assert.Nil(t, e)
	assert.Nil(t, e2)
	assert.Equal(t, "ci", name)
	assert.NotNil(t, expires)
	assert.Nil(t, none)
	assert.NotNil(t, badDate)
	assert.NotNil(t, unknown)
}
