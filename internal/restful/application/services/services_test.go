package services_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParse(t *testing.T) {
	t.Run("Quando lê as aplicações do corpo.", func(t *testing.T) { whenParsingApplications(t) })
	t.Run("Quando lê a mudança com e sem id no caminho.", func(t *testing.T) { whenParsingChanges(t) })
	t.Run("Quando lê os ids.", func(t *testing.T) { whenParsingIDs(t) })
	t.Run("Quando lê a paginação.", func(t *testing.T) { whenParsingPagination(t) })
	t.Run("Quando há ids repetidos.", func(t *testing.T) { whenThereAreDuplicatedIDs(t) })
}

// Functions

func request(method, body string, id string) *http.Request {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if id != "" {
		req.SetPathValue("id", id)
	}
	return req
}

func whenParsingApplications(t *testing.T) {
	one, e := services.ParseApplications(request(http.MethodPost, `{"name":"minha-app"}`, ""))
	many, _ := services.ParseApplications(request(http.MethodPost, `[{"name":"a-app"},{"name":"b-app"}]`, ""))

	assert.Nil(t, e)
	assert.Len(t, one, 1)
	assert.Len(t, many, 2)
	for _, body := range []string{``, `{"name":`, `[]`} {
		_, bad := services.ParseApplications(request(http.MethodPost, body, ""))
		assert.NotNil(t, bad, body)
	}
}

func whenParsingChanges(t *testing.T) {
	id := uuid.NewV7()

	withoutID, e := services.ParseApplicationsForChange(request(http.MethodPut, `{"id":"`+id.String()+`","name":"minha-app"}`, ""))
	withID, e2 := services.ParseApplicationsForChange(request(http.MethodPut, `{"name":"minha-app"}`, id.String()))
	_, two := services.ParseApplicationsForChange(request(http.MethodPut, `[{"name":"a-app"},{"name":"b-app"}]`, id.String()))
	_, badID := services.ParseApplicationsForChange(request(http.MethodPut, `{"name":"a-app"}`, "x"))

	assert.Nil(t, e)
	assert.Nil(t, e2)
	assert.Equal(t, id, withoutID[0].ID)
	assert.Equal(t, id, withID[0].ID, "o id do caminho vale")
	assert.NotNil(t, two, "com id no caminho, só um item")
	assert.NotNil(t, badID)
}

func whenParsingIDs(t *testing.T) {
	id := uuid.NewV7()

	fromPath, e := services.ParseIDs(request(http.MethodDelete, ``, id.String()))
	fromBody, e2 := services.ParseIDs(request(http.MethodDelete, `["`+id.String()+`"]`, ""))
	_, badPath := services.ParseIDs(request(http.MethodDelete, ``, "x"))
	_, badBody := services.ParseIDs(request(http.MethodDelete, `{`, ""))

	assert.Nil(t, e)
	assert.Nil(t, e2)
	assert.Equal(t, []uuid.UUID{id}, fromPath)
	assert.Equal(t, []uuid.UUID{id}, fromBody)
	assert.NotNil(t, badPath)
	assert.NotNil(t, badBody)
}

func whenParsingPagination(t *testing.T) {
	page, limit, e := services.ParsePagination(httptest.NewRequest(http.MethodGet, "/?page=2&limit=7", nil))
	defPage, defLimit, _ := services.ParsePagination(httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Nil(t, e)
	assert.Equal(t, [2]int{2, 7}, [2]int{page, limit})
	assert.Equal(t, [2]int{1, entities.MaxPageSize}, [2]int{defPage, defLimit})
	for _, query := range []string{"?page=0", "?page=x", "?limit=0", "?limit=y"} {
		_, _, bad := services.ParsePagination(httptest.NewRequest(http.MethodGet, "/"+query, nil))
		assert.NotNil(t, bad, query)
	}
}

func whenThereAreDuplicatedIDs(t *testing.T) {
	a, b := uuid.NewV7(), uuid.NewV7()

	duplicated := services.DuplicatedIDs([]uuid.UUID{a, b, a})

	assert.Equal(t, map[uuid.UUID]bool{a: true}, duplicated)
	assert.Equal(t, []uuid.UUID{a, b}, services.ApplicationIDs([]entities.Application{{ID: a}, {ID: b}}))
}
