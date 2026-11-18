package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/murkio-dev/vault/pkg/httpx"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestFormats(t *testing.T) {
	t.Run("Quando o Accept escolhe o formato.", func(t *testing.T) { whenAcceptPicksTheFormat(t) })
	t.Run("Quando o Accept traz qualidade.", func(t *testing.T) { whenAcceptHasQuality(t) })
	t.Run("Quando o formato é JSON.", func(t *testing.T) { whenTheFormatIsJSON(t) })
	t.Run("Quando o formato é XML.", func(t *testing.T) { whenTheFormatIsXML(t) })
	t.Run("Quando o formato é YAML.", func(t *testing.T) { whenTheFormatIsYAML(t) })
	t.Run("Quando o formato é tabela.", func(t *testing.T) { whenTheFormatIsATable(t) })
}

// Functions

func respondWith(accept string, output any) (string, string) {
	recorder := call(accept, func(w http.ResponseWriter, r *http.Request) {
		httpx.RespondList(w, r, output, http.StatusOK)
	})
	return recorder.Header().Get("Content-Type"), recorder.Body.String()
}

func sample() map[string]any {
	return map[string]any{
		"data": []any{map[string]any{"name": "a", "count": 2, "ratio": 1.5, "active": true, "none": nil, "tags": []any{"x", "y"}, "owner": map[string]any{"id": "u1"}}},
		"page": map[string]any{"page": 1, "limit": 10},
	}
}

func whenAcceptPicksTheFormat(t *testing.T) {
	cases := map[string]string{
		"":                                  "application/json",
		"application/json":                  "application/json",
		"application/xml":                   "application/xml; charset=utf-8",
		"text/xml":                          "application/xml; charset=utf-8",
		"APPLICATION/XML":                   "application/xml; charset=utf-8",
		"application/x-yaml":                "application/x-yaml; charset=utf-8",
		"application/yaml":                  "application/x-yaml; charset=utf-8",
		"text/yaml":                         "application/x-yaml; charset=utf-8",
		"text/plain":                        "text/plain; charset=utf-8",
		"image/png":                         "application/json",
		"*/*":                               "application/json",
		", ,application/xml":                "application/xml; charset=utf-8",
		"text/html, application/xml; x=1":   "application/xml; charset=utf-8",
		"application/xml;charset=utf-8":     "application/xml; charset=utf-8",
		"application/xml, application/yaml": "application/xml; charset=utf-8",
	}

	for accept, want := range cases {
		contentType, _ := respondWith(accept, sample())

		assert.Equal(t, want, contentType, "Accept %q", accept)
	}
}

func whenAcceptHasQuality(t *testing.T) {
	cases := map[string]string{
		"application/xml;q=0.5, application/json":       "application/json",
		"application/json;q=0.2, application/xml;q=0.9": "application/xml; charset=utf-8",
		"application/xml;q=0":                           "application/json",
		"application/xml;q=abc":                         "application/xml; charset=utf-8",
		"application/xml;q=0.8, text/yaml;q=0.8":        "application/xml; charset=utf-8",
	}

	for accept, want := range cases {
		contentType, _ := respondWith(accept, sample())

		assert.Equal(t, want, contentType, "Accept %q", accept)
	}
}

func whenTheFormatIsJSON(t *testing.T) {
	_, body := respondWith("application/json", sample())

	assert.Contains(t, body, `"name":"a"`)
	assert.True(t, strings.HasSuffix(body, "\n"), "uma linha por resposta")
}

func whenTheFormatIsXML(t *testing.T) {
	_, body := respondWith("application/xml", sample())

	assert.True(t, strings.HasPrefix(body, `<?xml version="1.0" encoding="UTF-8"?>`))
	assert.Contains(t, body, "<response>")
	assert.Contains(t, body, "<name>a</name>")
	assert.Contains(t, body, "<count>2</count>", "inteiro sem ponto decimal")
	assert.Contains(t, body, "<ratio>1.5</ratio>")
	assert.Contains(t, body, "<active>true</active>")
	assert.Contains(t, body, "<none></none>", "nulo vira elemento vazio")
	assert.Equal(t, 1, strings.Count(body, "<data>"), "o item da lista repete o nome da chave")
	assert.Contains(t, body, "<tags>x</tags>")
	assert.Contains(t, body, "<tags>y</tags>", "lista de valores repete o elemento")
	assert.Contains(t, body, "<id>u1</id>", "objeto aninhado")

	_, scalars := respondWith("application/xml", map[string]any{"data": []any{"a", "b"}, "nothing": nil})
	assert.Contains(t, scalars, "<data>a</data>")

	_, root := respondWith("application/xml", []any{"a", map[string]any{"k": "v"}})
	assert.Contains(t, root, "<item>a</item>", "raiz que é lista usa item")
	assert.Contains(t, root, "<k>v</k>")
}

func whenTheFormatIsYAML(t *testing.T) {
	_, body := respondWith("text/yaml", sample())

	assert.Contains(t, body, "name: a")
	assert.Contains(t, body, "count: 2")
	assert.Contains(t, body, "ratio: 1.5")
	assert.Contains(t, body, "limit: 10")
}

func whenTheFormatIsATable(t *testing.T) {
	_, both := respondWith("text/plain", sample())

	assert.Contains(t, both, "DATA\n", "com mais de uma seção, cada uma ganha o título")
	assert.Contains(t, both, "PAGE\n")
	header := strings.Split(both, "\n")[1]
	assert.Equal(t, []string{"active", "count", "name", "none", "owner", "ratio", "tags"}, strings.Fields(header), "colunas em ordem alfabética")
	assert.Contains(t, both, `{"id":"u1"}`, "objeto aninhado vira JSON na célula")
	assert.Contains(t, both, `["x","y"]`)
	assert.Contains(t, both, "1.5")
	assert.Contains(t, both, "field  value", "objeto vira duas colunas")

	_, only := respondWith("text/plain", map[string]any{"data": []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}}})
	assert.NotContains(t, only, "DATA", "uma seção só não leva título")
	assert.Equal(t, "name\na\nb\n", only)

	_, scalars := respondWith("text/plain", map[string]any{"data": []any{"a", "b"}})
	assert.Equal(t, "value\na\nb\n", scalars)

	_, object := respondWith("text/plain", map[string]any{"data": map[string]any{"name": "a"}})
	assert.Equal(t, "field  value\nname   a\n", object)

	_, single := respondWith("text/plain", map[string]any{"error": map[string]any{"title": "x"}, "errors": nil})
	assert.Contains(t, single, "title", "chave nula é ignorada")

	_, unknown := respondWith("text/plain", map[string]any{"other": 1})
	assert.Empty(t, unknown, "sem data, error, errors ou page, não há o que mostrar")

	_, list := respondWith("text/plain", []string{"a", "b"})
	assert.Equal(t, "value\na\nb\n", list, "raiz que não é objeto vira a seção data")

	_, scalar := respondWith("text/plain", "texto")
	assert.Equal(t, "value\ntexto\n", scalar)
}
