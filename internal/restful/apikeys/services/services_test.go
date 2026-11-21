package services_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê o dono na query.", func(t *testing.T) { whenParsingTheOwner(t) })
	t.Run("Quando lê o id da chave.", func(t *testing.T) { whenParsingTheID(t) })
	t.Run("Quando lê a criação.", func(t *testing.T) { whenParsingTheCreate(t) })
	t.Run("Quando lê a troca de nome.", func(t *testing.T) { whenParsingTheUpdate(t) })
}

// Functions

func request(query, body string, path map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys?"+query, strings.NewReader(body))
	for name, value := range path {
		req.SetPathValue(name, value)
	}
	return req
}

func whenParsingTheOwner(t *testing.T) {
	app, member := uuid.NewV7(), uuid.NewV7()

	gotApp, gotMember, e := services.ParseOwner(request("application_id="+app.String()+"&member_id="+member.String(), "", nil))
	_, me, _ := services.ParseOwner(request("application_id="+app.String()+"&member_id=me", "", nil))
	_, absent, _ := services.ParseOwner(request("application_id="+app.String(), "", nil))
	_, _, noApp := services.ParseOwner(request("member_id=me", "", nil))
	_, _, badMember := services.ParseOwner(request("application_id="+app.String()+"&member_id=x", "", nil))

	assert.Nil(t, e)
	assert.Equal(t, [2]uuid.UUID{app, member}, [2]uuid.UUID{gotApp, gotMember})
	assert.Equal(t, entities.SelfOwner, me)
	assert.Equal(t, entities.SelfOwner, absent, "sem member_id, vale o membro de quem chama")
	assert.NotNil(t, noApp, "application_id é obrigatório")
	assert.NotNil(t, badMember)
}

func whenParsingTheID(t *testing.T) {
	query := "application_id=" + uuid.NewV7().String()
	id := uuid.NewV7()

	_, _, got, e := services.ParseByID(request(query, "", map[string]string{"id": id.String()}))
	_, _, _, bad := services.ParseByID(request(query, "", map[string]string{"id": "x"}))
	_, _, _, noApp := services.ParseByID(request("", "", map[string]string{"id": id.String()}))

	assert.Nil(t, e)
	assert.Equal(t, id, got)
	assert.NotNil(t, bad)
	assert.NotNil(t, noApp)
}

func whenParsingTheCreate(t *testing.T) {
	query := "application_id=" + uuid.NewV7().String()

	data, e := services.ParseCreate(request(query, `{"name":"ci","type":"secret","expires_at":"2099-01-01T00:00:00Z"}`, nil))
	none, e2 := services.ParseCreate(request(query, `{"name":"ci","type":"publishable"}`, nil))
	_, badDate := services.ParseCreate(request(query, `{"name":"ci","expires_at":"amanhã"}`, nil))
	_, unknown := services.ParseCreate(request(query, `{"name":"ci","x":1}`, nil))
	_, noApp := services.ParseCreate(request("", `{"name":"ci"}`, nil))

	assert.Nil(t, e)
	assert.Nil(t, e2)
	assert.Equal(t, "ci", data.Name)
	assert.Equal(t, entities.KeySecret, data.Type)
	assert.NotNil(t, data.ExpiresAt)
	assert.Equal(t, entities.KeyPublishable, none.Type)
	assert.Nil(t, none.ExpiresAt)
	assert.NotNil(t, badDate)
	assert.NotNil(t, unknown)
	assert.NotNil(t, noApp)
}

func whenParsingTheUpdate(t *testing.T) {
	query := "application_id=" + uuid.NewV7().String()
	path := map[string]string{"id": uuid.NewV7().String()}

	_, _, _, name, e := services.ParseUpdate(request(query, `{"name":"deploy"}`, path))
	_, _, _, _, unknown := services.ParseUpdate(request(query, `{"type":"secret"}`, path))

	assert.Nil(t, e)
	assert.Equal(t, "deploy", name)
	assert.NotNil(t, unknown, "só o nome muda")
}
