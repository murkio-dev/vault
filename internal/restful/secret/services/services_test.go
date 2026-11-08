package services_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê os ids do caminho.", func(t *testing.T) { whenParsingPaths(t) })
	t.Run("Quando lê a criação.", func(t *testing.T) { whenParsingTheCreate(t) })
	t.Run("Quando lê a troca de valor e a renomeação.", func(t *testing.T) { whenParsingUpdateAndRename(t) })
	t.Run("Quando lê o reveal.", func(t *testing.T) { whenParsingTheReveal(t) })
}

// Functions

func request(body string, path map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	for name, value := range path {
		req.SetPathValue(name, value)
	}
	return req
}

func full() map[string]string {
	return map[string]string{"app": uuid.NewV7().String(), "member": "me", "id": uuid.NewV7().String()}
}

func whenParsingPaths(t *testing.T) {
	path := full()

	app, member, e := services.ParsePath(request("", path))
	_, _, secret, e2 := services.ParseSecretPath(request("", path))
	_, _, badMember := services.ParsePath(request("", map[string]string{"app": path["app"], "member": "eu"}))
	_, _, _, badSecret := services.ParseSecretPath(request("", map[string]string{"app": path["app"], "member": "me", "id": "x"}))

	assert.Nil(t, e)
	assert.Nil(t, e2)
	assert.NotEqual(t, uuid.UUID{}, app)
	assert.Equal(t, entities.SelfOwner, member)
	assert.Equal(t, path["id"], secret.String())
	assert.NotNil(t, badMember)
	assert.NotNil(t, badSecret)
}

func whenParsingTheCreate(t *testing.T) {
	path := full()

	_, _, one, e := services.ParseCreate(request(`{"name":"A","type":"string","value":"v"}`, path))
	_, _, many, _ := services.ParseCreate(request(`[{"name":"A","type":"string","value":"v"},{"name":"B","type":"url","value":"https://x.io"}]`, path))

	assert.Nil(t, e)
	assert.Len(t, one, 1)
	assert.Len(t, many, 2)
	for name, body := range map[string]string{"desconhecido": `{"name":"A","x":1}`, "lista vazia": `[]`, "vazio": ``, "yaml": "name: A\n"} {
		_, _, _, bad := services.ParseCreate(request(body, path))
		assert.NotNil(t, bad, name)
	}
}

func whenParsingUpdateAndRename(t *testing.T) {
	path := full()

	_, _, _, value, e := services.ParseUpdate(request(`{"value":"novo"}`, path))
	_, _, _, name, e2 := services.ParseRename(request(`{"name":"NOVO"}`, path))
	_, _, _, _, badUpdate := services.ParseUpdate(request(`{"value":"v","x":1}`, path))
	_, _, _, _, badRename := services.ParseRename(request(``, path))

	assert.Nil(t, e)
	assert.Nil(t, e2)
	assert.Equal(t, "novo", value)
	assert.Equal(t, "NOVO", name)
	assert.NotNil(t, badUpdate)
	assert.NotNil(t, badRename)
}

func whenParsingTheReveal(t *testing.T) {
	path := full()

	_, _, _, none, e := services.ParseReveal(request(``, path))
	_, _, _, version, e2 := services.ParseReveal(request(`{"version":4}`, path))
	_, _, _, _, bad := services.ParseReveal(request(`{"version":"x"}`, path))

	assert.Nil(t, e)
	assert.Nil(t, e2)
	assert.Zero(t, none)
	assert.Equal(t, 4, version)
	assert.NotNil(t, bad)
}
