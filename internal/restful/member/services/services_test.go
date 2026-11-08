package services_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê a aplicação do caminho.", func(t *testing.T) { whenParsingTheApplication(t) })
	t.Run("Quando lê a listagem.", func(t *testing.T) { whenParsingTheList(t) })
	t.Run("Quando lê aplicação e membro.", func(t *testing.T) { whenParsingApplicationAndMember(t) })
	t.Run("Quando lê a atualização.", func(t *testing.T) { whenParsingTheUpdate(t) })
}

// Functions

func request(body string, path map[string]string, query string) *http.Request {
	req := httptest.NewRequest(http.MethodPatch, "/?"+query, strings.NewReader(body))
	for name, value := range path {
		req.SetPathValue(name, value)
	}
	return req
}

func whenParsingTheApplication(t *testing.T) {
	id := uuid.NewV7()

	got, e := services.ParseApplication(request("", map[string]string{"app": id.String()}, ""))
	_, bad := services.ParseApplication(request("", map[string]string{"app": "x"}, ""))

	assert.Nil(t, e)
	assert.Equal(t, id, got)
	assert.NotNil(t, bad)
}

func whenParsingTheList(t *testing.T) {
	id := uuid.NewV7()

	app, page, limit, e := services.ParseList(request("", map[string]string{"app": id.String()}, "page=2&limit=5"))
	_, _, _, badApp := services.ParseList(request("", map[string]string{"app": "x"}, ""))
	_, _, _, badPage := services.ParseList(request("", map[string]string{"app": id.String()}, "page=0"))

	assert.Nil(t, e)
	assert.Equal(t, id, app)
	assert.Equal(t, [2]int{2, 5}, [2]int{page, limit})
	assert.NotNil(t, badApp)
	assert.NotNil(t, badPage)
}

func whenParsingApplicationAndMember(t *testing.T) {
	app, member := uuid.NewV7(), uuid.NewV7()

	gotApp, gotMember, e := services.ParseMember(request("", map[string]string{"app": app.String(), "member": member.String()}, ""))
	_, _, me := services.ParseMember(request("", map[string]string{"app": app.String(), "member": "me"}, ""))
	_, _, badApp := services.ParseMember(request("", map[string]string{"app": "x", "member": member.String()}, ""))

	assert.Nil(t, e)
	assert.Equal(t, [2]uuid.UUID{app, member}, [2]uuid.UUID{gotApp, gotMember})
	assert.NotNil(t, me, "remover exige o id: me não vale")
	assert.NotNil(t, badApp)
}

func whenParsingTheUpdate(t *testing.T) {
	path := map[string]string{"app": uuid.NewV7().String(), "member": uuid.NewV7().String()}

	_, _, role, status, e := services.ParseUpdate(request(`{"role":"admin"}`, path, ""))
	_, _, _, _, empty := services.ParseUpdate(request(``, path, ""))
	_, _, _, _, unknown := services.ParseUpdate(request(`{"role":"admin","x":1}`, path, ""))
	_, _, _, _, badPath := services.ParseUpdate(request(`{"role":"admin"}`, map[string]string{"app": "x", "member": "y"}, ""))

	assert.Nil(t, e)
	if assert.NotNil(t, role) {
		assert.Equal(t, entities.RoleAdmin, *role)
	}
	assert.Nil(t, status)
	assert.NotNil(t, empty)
	assert.NotNil(t, unknown)
	assert.NotNil(t, badPath)
}
