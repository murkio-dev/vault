package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestBind(t *testing.T) {
	t.Run("Quando o corpo é lido pelo Content-Type.", func(t *testing.T) { whenBodyFormatComesFromContentType(t) })
	t.Run("Quando o corpo é lido pelo primeiro caractere.", func(t *testing.T) { whenBodyFormatIsSniffed(t) })
	t.Run("Quando o corpo de BindBody é inválido.", func(t *testing.T) { whenBindBodyIsInvalid(t) })
	t.Run("Quando os ids vêm em JSON.", func(t *testing.T) { whenIDsComeAsJSON(t) })
	t.Run("Quando os ids vêm em XML ou YAML.", func(t *testing.T) { whenIDsComeAsXMLOrYAML(t) })
	t.Run("Quando os ids são inválidos.", func(t *testing.T) { whenIDsAreInvalid(t) })
	t.Run("Quando o JSON estrito é inválido.", func(t *testing.T) { whenStrictJSONIsInvalid(t) })
}

// Functions

func bodyRequest(contentType, body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request
}

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func whenBodyFormatComesFromContentType(t *testing.T) {
	json, err := httpx.BindBody[item](bodyRequest("application/json", `[{"name":"a"},{"name":"b"}]`))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}, {Name: "b"}}, json)

	object, err := httpx.BindBody[item](bodyRequest("application/json; charset=utf-8", `{"name":"a"}`))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}}, object, "um objeto vira lista de um")

	xmlList, err := httpx.BindBody[item](bodyRequest("application/xml", `<items><item><name>a</name></item><item><name>b</name></item></items>`))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}, {Name: "b"}}, xmlList)

	xmlEmpty, err := httpx.BindBody[item](bodyRequest("text/xml", `<item></item>`))
	assert.NoError(t, err)
	assert.Len(t, xmlEmpty, 1)

	yamlList, err := httpx.BindBody[item](bodyRequest("application/x-yaml", "- name: a\n- name: b\n"))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}, {Name: "b"}}, yamlList)

	yamlObject, err := httpx.BindBody[item](bodyRequest("text/yaml", "name: a\n"))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}}, yamlObject)
}

func whenBodyFormatIsSniffed(t *testing.T) {
	json, err := httpx.BindBody[item](bodyRequest("", `  [{"name":"a"}]  `))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}}, json)

	object, err := httpx.BindBody[item](bodyRequest("text/plain", `{"name":"a"}`))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}}, object)

	xml, err := httpx.BindBody[item](bodyRequest("", `<items><item><name>a</name></item></items>`))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}}, xml)

	yaml, err := httpx.BindBody[item](bodyRequest("", "name: a\n"))
	assert.NoError(t, err)
	assert.Equal(t, []item{{Name: "a"}}, yaml)
}

func whenBindBodyIsInvalid(t *testing.T) {
	nilBody := httptest.NewRequest(http.MethodPost, "/", nil)
	nilBody.Body = nil
	unreadable := httptest.NewRequest(http.MethodPost, "/", failingBody{})

	cases := map[string]*http.Request{
		"sem corpo":       nilBody,
		"ilegível":        unreadable,
		"vazio":           bodyRequest("application/json", ""),
		"só espaços":      bodyRequest("", "   \n"),
		"json quebrado":   bodyRequest("application/json", `[{"name":`),
		"objeto quebrado": bodyRequest("application/json", `{"name":`),
		"json de texto":   bodyRequest("application/json", `"texto"`),
		"xml quebrado":    bodyRequest("application/xml", `<item><name>`),
		"yaml quebrado":   bodyRequest("application/x-yaml", "name: [a"),
	}
	for name, request := range cases {
		items, err := httpx.BindBody[item](request)

		assert.Error(t, err, name)
		assert.Nil(t, items, name)
	}
}

func whenIDsComeAsJSON(t *testing.T) {
	first, second := uuid.NewV7(), uuid.NewV7()

	list, err := httpx.BindIDs(bodyRequest("", `["`+first.String()+`","`+second.String()+`"]`))
	assert.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, list)

	object, err := httpx.BindIDs(bodyRequest("", `{"id":"`+first.String()+`"}`))
	assert.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first}, object)

	objects, err := httpx.BindIDs(bodyRequest("", `[{"id":"`+first.String()+`"},{"id":"`+second.String()+`"}]`))
	assert.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, objects)
}

func whenIDsComeAsXMLOrYAML(t *testing.T) {
	first, second := uuid.NewV7(), uuid.NewV7()

	xmlList, err := httpx.BindIDs(bodyRequest("", `<ids><id>`+first.String()+`</id><id>`+second.String()+`</id></ids>`))
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, xmlList)

	yamlList, err := httpx.BindIDs(bodyRequest("", "- "+first.String()+"\n- "+second.String()+"\n"))
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, yamlList)

	yamlObjects, err := httpx.BindIDs(bodyRequest("", "- id: "+first.String()+"\n- id: "+second.String()+"\n"))
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, yamlObjects)

	yamlObject, err := httpx.BindIDs(bodyRequest("", "id: "+first.String()+"\n"))
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first}, yamlObject)
}

func whenIDsAreInvalid(t *testing.T) {
	nilBody := httptest.NewRequest(http.MethodPost, "/", nil)
	nilBody.Body = nil

	cases := map[string]*http.Request{
		"sem corpo":     nilBody,
		"ilegível":      httptest.NewRequest(http.MethodPost, "/", failingBody{}),
		"vazio":         bodyRequest("", ""),
		"formato solto": bodyRequest("", "isto não é um id"),
		"lista vazia":   bodyRequest("", `[]`),
		"json quebrado": bodyRequest("", `[`),
		"id inválido":   bodyRequest("", `["nao-e-uuid"]`),
		"xml quebrado":  bodyRequest("", `<ids><id>`),
	}
	for name, request := range cases {
		ids, err := httpx.BindIDs(request)

		assert.Error(t, err, name)
		assert.Empty(t, ids, name)
	}
}

func whenStrictJSONIsInvalid(t *testing.T) {
	nilBody := httptest.NewRequest(http.MethodPost, "/", nil)
	nilBody.Body = nil

	cases := map[string]*http.Request{
		"sem corpo":          nilBody,
		"ilegível":           httptest.NewRequest(http.MethodPost, "/", failingBody{}),
		"vazio":              bodyRequest("", ""),
		"campo desconhecido": bodyRequest("", `{"name":"a","extra":1}`),
		"lista com campo":    bodyRequest("", `[{"name":"a","extra":1}]`),
		"lista quebrada":     bodyRequest("", `[{"name":`),
		"objeto quebrado":    bodyRequest("", `{"name":`),
		"texto":              bodyRequest("", `"a"`),
		"yaml":               bodyRequest("application/x-yaml", "name: a\n"),
	}
	for name, request := range cases {
		items, err := httpx.BindJSON[item](request)

		assert.Error(t, err, name)
		assert.Nil(t, items, name)
	}
}
