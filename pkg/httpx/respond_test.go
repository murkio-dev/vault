package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type item struct {
	Name string `json:"name" xml:"name" yaml:"name"`
}

type listOutput struct {
	Data   []item           `json:"data,omitempty"`
	Errors []entities.Error `json:"errors,omitempty"`
}

type singleOutput struct {
	Data  item            `json:"data,omitempty"`
	Error *entities.Error `json:"error,omitempty"`
}

// Main test

func TestRespond(t *testing.T) {
	t.Run("Quando a resposta tem um item só.", func(t *testing.T) { whenRespondHasOneItem(t) })
	t.Run("Quando a resposta tem um lote.", func(t *testing.T) { whenRespondHasABatch(t) })
	t.Run("Quando a listagem tem um item só.", func(t *testing.T) { whenListHasOneItem(t) })
	t.Run("Quando o sucesso é 204.", func(t *testing.T) { whenSuccessIsNoContent(t) })
	t.Run("Quando só há erros.", func(t *testing.T) { whenThereAreOnlyErrors(t) })
	t.Run("Quando há dados e erros.", func(t *testing.T) { whenThereAreDataAndErrors(t) })
	t.Run("Quando o erro vem num ponteiro.", func(t *testing.T) { whenTheErrorIsAPointer(t) })
	t.Run("Quando o output não é uma struct.", func(t *testing.T) { whenTheOutputIsNotAStruct(t) })
	t.Run("Quando responde um erro direto.", func(t *testing.T) { whenRespondingAnError(t) })
	t.Run("Quando a resposta ignora o Accept.", func(t *testing.T) { whenRespondJSONIgnoresAccept(t) })
	t.Run("Quando o output não serializa.", func(t *testing.T) { whenTheOutputCannotBeRendered(t) })
}

// Functions

func call(accept string, respond func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/things", nil)
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	recorder := httptest.NewRecorder()
	respond(recorder, request)
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func whenRespondHasOneItem(t *testing.T) {
	recorder := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, listOutput{Data: []item{{Name: "a"}}}, http.StatusCreated)
	})

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.Equal(t, map[string]any{"name": "a"}, decode(t, recorder)["data"], "um item vem como objeto, não como lista de um")
}

func whenRespondHasABatch(t *testing.T) {
	recorder := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, listOutput{Data: []item{{Name: "a"}, {Name: "b"}}}, http.StatusOK)
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Len(t, decode(t, recorder)["data"], 2, "lote real mantém a lista")
}

func whenListHasOneItem(t *testing.T) {
	recorder := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.RespondList(w, r, listOutput{Data: []item{{Name: "a"}}}, http.StatusOK)
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Len(t, decode(t, recorder)["data"], 1, "listagem nunca desembrulha")
}

func whenSuccessIsNoContent(t *testing.T) {
	recorder := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, listOutput{}, http.StatusNoContent)
	})

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Empty(t, recorder.Body.String())
	assert.Empty(t, recorder.Header().Get("Content-Type"))
}

func whenThereAreOnlyErrors(t *testing.T) {
	notFound := entities.NotFoundError("Resource")
	one := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, listOutput{Errors: []entities.Error{notFound}}, http.StatusOK)
	})
	two := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, listOutput{Errors: []entities.Error{notFound, entities.UnauthorizedError()}}, http.StatusOK)
	})
	noContent := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, listOutput{Errors: []entities.Error{notFound}}, http.StatusNoContent)
	})

	assert.Equal(t, http.StatusNotFound, one.Code, "o status é o do primeiro erro")
	assert.Contains(t, decode(t, one), "error", "um erro só vem como objeto")
	assert.NotContains(t, decode(t, one), "errors")
	assert.Equal(t, http.StatusNotFound, two.Code)
	assert.Len(t, decode(t, two)["errors"], 2, "vários erros mantêm a lista")
	assert.Equal(t, http.StatusNotFound, noContent.Code, "o erro vence o 204")
}

func whenThereAreDataAndErrors(t *testing.T) {
	recorder := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, listOutput{Data: []item{{Name: "a"}}, Errors: []entities.Error{entities.NotFoundError("Resource")}}, http.StatusOK)
	})

	assert.Equal(t, http.StatusMultiStatus, recorder.Code, "sucesso parcial é 207")
	body := decode(t, recorder)
	assert.Len(t, body["data"], 1, "o lote misto não desembrulha")
	assert.Len(t, body["errors"], 1)
}

func whenTheErrorIsAPointer(t *testing.T) {
	forbidden := entities.NotFoundError("Resource")
	failed := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, singleOutput{Error: &forbidden}, http.StatusOK)
	})
	succeeded := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, singleOutput{Data: item{Name: "a"}}, http.StatusOK)
	})
	empty := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, singleOutput{}, http.StatusOK)
	})
	var none *singleOutput
	nilPointer := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, none, http.StatusOK)
	})

	assert.Equal(t, http.StatusNotFound, failed.Code)
	assert.Equal(t, http.StatusOK, succeeded.Code)
	assert.Equal(t, http.StatusOK, empty.Code, "sem erro e sem dado ainda é sucesso")
	assert.Equal(t, http.StatusOK, nilPointer.Code, "ponteiro nil não derruba a resposta")
}

func whenTheOutputIsNotAStruct(t *testing.T) {
	recorder := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.Respond(w, r, []string{"a", "b"}, http.StatusAccepted)
	})

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.JSONEq(t, `["a","b"]`, recorder.Body.String())
}

func whenRespondingAnError(t *testing.T) {
	unauthorized := entities.UnauthorizedError()
	recorder := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.RespondError(w, r, unauthorized)
	})

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	body := decode(t, recorder)
	errorBody, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/api/v1/things", errorBody["instance"], "sem instance, vale o caminho da requisição")

	withInstance := unauthorized
	withInstance.Instance = "/outro"
	kept := call("", func(w http.ResponseWriter, r *http.Request) {
		httpx.RespondError(w, r, withInstance)
	})
	assert.Equal(t, "/outro", decode(t, kept)["error"].(map[string]any)["instance"])
}

func whenRespondJSONIgnoresAccept(t *testing.T) {
	output := listOutput{Data: []item{{Name: "a"}}}

	plain := call("text/plain", func(w http.ResponseWriter, r *http.Request) { httpx.Respond(w, r, output, http.StatusOK) })
	forced := call("text/plain", func(w http.ResponseWriter, r *http.Request) { httpx.RespondJSON(w, r, output, http.StatusOK) })

	assert.Equal(t, "text/plain; charset=utf-8", plain.Header().Get("Content-Type"))
	assert.Equal(t, "application/json", forced.Header().Get("Content-Type"), "valor sensível nunca sai em outro formato")
	assert.Equal(t, map[string]any{"name": "a"}, decode(t, forced)["data"])
}

func whenTheOutputCannotBeRendered(t *testing.T) {
	type broken struct {
		Data []item   `json:"data"`
		Fn   chan int `json:"fn"`
	}

	for _, accept := range []string{"", "application/xml", "text/yaml", "text/plain"} {
		recorder := call(accept, func(w http.ResponseWriter, r *http.Request) {
			httpx.Respond(w, r, broken{Data: []item{{Name: "a"}}, Fn: make(chan int)}, http.StatusOK)
		})

		assert.Equal(t, http.StatusInternalServerError, recorder.Code, accept)
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"), "o erro de renderização sempre sai em JSON")
		assert.NotContains(t, recorder.Body.String(), "unsupported", accept, "a causa não vai para o cliente")
	}
}
