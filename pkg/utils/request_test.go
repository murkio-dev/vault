package utils_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestRequestHelpers(t *testing.T) {
	t.Run("Quando o UUID do caminho é válido ou não.", func(t *testing.T) { whenPathUUIDIsParsed(t) })
	t.Run("Quando o membro é me ou um id.", func(t *testing.T) { whenPathMemberIsParsed(t) })
	t.Run("Quando o corpo é JSON estrito.", func(t *testing.T) { whenBodyIsStrictJSON(t) })
	t.Run("Quando o corpo é opcional.", func(t *testing.T) { whenBodyIsOptional(t) })
	t.Run("Quando a paginação é lida.", func(t *testing.T) { whenPaginationIsRead(t) })
	t.Run("Quando a janela da página é aparada.", func(t *testing.T) { whenPageWindowIsClamped(t) })
	t.Run("Quando a rota é registrada com e sem barra final.", func(t *testing.T) { whenRouteIsRegisteredWithAndWithoutSlash(t) })
}

// Functions

func requestWithPath(name, value string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue(name, value)
	return req
}

func whenPathUUIDIsParsed(t *testing.T) {
	id := uuid.NewV7()

	got, e := utils.PathUUID(requestWithPath("app", id.String()), "app")
	assert.Nil(t, e)
	assert.Equal(t, id, got)

	for _, value := range []string{"", "abc", "00000000-0000-0000-0000-000000000000"} {
		_, e := utils.PathUUID(requestWithPath("app", value), "app")
		if assert.NotNil(t, e, value) {
			assert.Equal(t, entities.ErrTypeValidation, e.Type)
			assert.NotContains(t, e.Detail, "abc", "o erro não repete o valor")
		}
	}
}

func whenPathMemberIsParsed(t *testing.T) {
	id := uuid.NewV7()

	me, e := utils.PathMember(requestWithPath("member", "me"))
	assert.Nil(t, e)
	assert.Equal(t, entities.SelfOwner, me)

	byID, e := utils.PathMember(requestWithPath("member", id.String()))
	assert.Nil(t, e)
	assert.Equal(t, id, byID)

	_, e = utils.PathMember(requestWithPath("member", "eu"))
	assert.NotNil(t, e)
}

type body struct {
	Name string `json:"name"`
}

func newBodyRequest(content string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(content))
}

func whenBodyIsStrictJSON(t *testing.T) {
	one, e := utils.BindOne[body](newBodyRequest(`{"name":"a"}`))
	assert.Nil(t, e)
	assert.Equal(t, "a", one.Name)

	single, e := utils.BindOne[body](newBodyRequest(`[{"name":"b"}]`))
	assert.Nil(t, e, "uma lista de um item vale como um item")
	assert.Equal(t, "b", single.Name)

	for name, content := range map[string]string{
		"desconhecido": `{"name":"a","x":1}`, "lista de dois": `[{"name":"a"},{"name":"b"}]`, "yaml": "name: a\n",
		"xml": `<body><name>a</name></body>`, "vazio": ``, "texto": `"a"`, "quebrado": `{"name":`,
	} {
		_, e := utils.BindOne[body](newBodyRequest(content))
		assert.NotNil(t, e, name)
	}

	list, err := utils.BindJSON[body](newBodyRequest(`[{"name":"a"},{"name":"b"}]`))
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	_, err = utils.BindJSON[body](newBodyRequest(""))
	assert.Error(t, err, "corpo vazio")
}

func whenBodyIsOptional(t *testing.T) {
	empty, e := utils.BindOptional[body](httptest.NewRequest(http.MethodPost, "/", nil))
	assert.Nil(t, e)
	assert.Equal(t, body{}, empty)

	given, e := utils.BindOptional[body](newBodyRequest(`{"name":"a"}`))
	assert.Nil(t, e)
	assert.Equal(t, "a", given.Name)

	_, e = utils.BindOptional[body](newBodyRequest(`{"name":"a","x":1}`))
	assert.NotNil(t, e)
}

func whenPaginationIsRead(t *testing.T) {
	page, limit, e := utils.Pagination(httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Nil(t, e)
	assert.Equal(t, 1, page)
	assert.Equal(t, entities.MaxPageSize, limit)

	page, limit, e = utils.Pagination(httptest.NewRequest(http.MethodGet, "/?page=4&limit=10", nil))
	assert.Nil(t, e)
	assert.Equal(t, [2]int{4, 10}, [2]int{page, limit})

	for _, query := range []string{"?page=0", "?page=x", "?limit=0", "?limit=-3"} {
		_, _, e := utils.Pagination(httptest.NewRequest(http.MethodGet, "/"+query, nil))
		assert.NotNil(t, e, query)
	}
}

func whenPageWindowIsClamped(t *testing.T) {
	page, limit, offset := utils.PageWindow(0, 0)
	assert.Equal(t, [3]int{1, entities.MaxPageSize, 0}, [3]int{page, limit, offset})

	_, limit, _ = utils.PageWindow(1, entities.MaxPageSize+50)
	assert.Equal(t, entities.MaxPageSize, limit)

	page, limit, offset = utils.PageWindow(3, 20)
	assert.Equal(t, [3]int{3, 20, 40}, [3]int{page, limit, offset})
}

func whenRouteIsRegisteredWithAndWithoutSlash(t *testing.T) {
	mux := http.NewServeMux()
	calls := 0
	utils.Handle(mux, "GET /api/v1/x", func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) })
	utils.Handle(mux, "GET /api/v1/y/{$}", func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) })

	for _, path := range []string{"/api/v1/x", "/api/v1/x/", "/api/v1/y/"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		assert.Equal(t, http.StatusNoContent, rec.Code, path)
	}
	assert.Equal(t, 3, calls)
}
