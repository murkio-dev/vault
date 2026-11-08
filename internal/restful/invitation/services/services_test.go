package services_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/invitation/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê a aplicação do caminho.", func(t *testing.T) { whenParsingTheApplication(t) })
	t.Run("Quando lê a revogação.", func(t *testing.T) { whenParsingTheRevoke(t) })
	t.Run("Quando lê a criação.", func(t *testing.T) { whenParsingTheCreate(t) })
	t.Run("Quando lê o aceite.", func(t *testing.T) { whenParsingTheAccept(t) })
}

// Functions

func request(body string, path map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	for name, value := range path {
		req.SetPathValue(name, value)
	}
	return req
}

func whenParsingTheApplication(t *testing.T) {
	id := uuid.NewV7()

	got, e := services.ParseApplication(request("", map[string]string{"app": id.String()}))
	_, bad := services.ParseApplication(request("", map[string]string{"app": "x"}))

	assert.Nil(t, e)
	assert.Equal(t, id, got)
	assert.NotNil(t, bad)
}

func whenParsingTheRevoke(t *testing.T) {
	app, id := uuid.NewV7(), uuid.NewV7()

	gotApp, gotID, e := services.ParseRevoke(request("", map[string]string{"app": app.String(), "id": id.String()}))
	_, _, badID := services.ParseRevoke(request("", map[string]string{"app": app.String(), "id": "x"}))
	_, _, badApp := services.ParseRevoke(request("", map[string]string{"app": "x", "id": id.String()}))

	assert.Nil(t, e)
	assert.Equal(t, [2]uuid.UUID{app, id}, [2]uuid.UUID{gotApp, gotID})
	assert.NotNil(t, badID)
	assert.NotNil(t, badApp)
}

func whenParsingTheCreate(t *testing.T) {
	path := map[string]string{"app": uuid.NewV7().String()}

	_, email, role, e := services.ParseCreate(request(`{"email":"a@example.com","role":"admin"}`, path))
	_, _, _, unknown := services.ParseCreate(request(`{"email":"a@example.com","role":"admin","x":1}`, path))
	_, _, _, empty := services.ParseCreate(request(``, path))
	_, _, _, badApp := services.ParseCreate(request(`{"email":"a@example.com","role":"admin"}`, map[string]string{"app": "x"}))

	assert.Nil(t, e)
	assert.Equal(t, "a@example.com", email)
	assert.Equal(t, entities.RoleAdmin, role)
	assert.NotNil(t, unknown)
	assert.NotNil(t, empty)
	assert.NotNil(t, badApp)
}

func whenParsingTheAccept(t *testing.T) {
	token, e := services.ParseAccept(request(`{"token":"inv_abc"}`, nil))
	_, unknown := services.ParseAccept(request(`{"token":"inv_abc","x":1}`, nil))
	_, empty := services.ParseAccept(request(``, nil))

	assert.Nil(t, e)
	assert.Equal(t, "inv_abc", token)
	assert.NotNil(t, unknown)
	assert.NotNil(t, empty)
}
