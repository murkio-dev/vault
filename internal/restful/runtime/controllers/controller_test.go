package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/runtime/controllers"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando lê o ambiente com e sem barra final.", func(t *testing.T) { whenEnvAcceptsTrailingSlash(t) })
	t.Run("Quando filtra por nomes.", func(t *testing.T) { whenEnvIsFiltered(t) })
	t.Run("Quando a resposta leva valores.", func(t *testing.T) { whenResponseCarriesValues(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
}

// Functions

func newServer(t *testing.T) (*http.ServeMux, *pkgmocks.UseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]) {
	t.Helper()
	fake := &pkgmocks.UseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]{}
	mux := http.NewServeMux()
	require.NoError(t, controllers.RegisterControllers(controllers.RegisterInput{Router: mux, EnvUseCase: fake}))
	return mux, fake
}

func get(mux *http.ServeMux, path, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func whenRegisterIsSuccessful(t *testing.T) {
	mux, _ := newServer(t)

	assert.NotNil(t, mux)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	assert.Error(t, controllers.RegisterControllers(controllers.RegisterInput{Router: http.NewServeMux()}))
}

func whenEnvAcceptsTrailingSlash(t *testing.T) {
	mux, fake := newServer(t)

	assert.Equal(t, http.StatusOK, get(mux, "/api/v1/runtime/abc123/env", "").Code)
	assert.Equal(t, http.StatusOK, get(mux, "/api/v1/runtime/abc123/env/", "").Code)
	if assert.Len(t, fake.Inputs, 2) {
		assert.Equal(t, "abc123", fake.Inputs[0].UriID)
		assert.Empty(t, fake.Inputs[0].Names)
	}
}

func whenEnvIsFiltered(t *testing.T) {
	mux, fake := newServer(t)

	get(mux, "/api/v1/runtime/abc123/env?names=A,B", "")

	if assert.Len(t, fake.Inputs, 1) {
		assert.Equal(t, []string{"A", "B"}, fake.Inputs[0].Names)
	}
}

func whenResponseCarriesValues(t *testing.T) {
	mux, fake := newServer(t)
	fake.PerformFunc = func(context.Context, useCases.ReadEnvInput) useCases.ReadEnvOutput {
		return useCases.ReadEnvOutput{Data: useCases.Environment{UriID: "abc", Variables: []entities.RuntimeVariable{{Name: "A", Value: "valor-secreto"}}}}
	}

	for _, accept := range []string{"application/x-yaml", "application/xml", "text/plain"} {
		rec := get(mux, "/api/v1/runtime/abc/env", accept)

		assert.Equal(t, http.StatusOK, rec.Code, accept)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", accept+": o runtime ignora o Accept")
		assert.Contains(t, rec.Body.String(), "valor-secreto")
	}
}

func whenUseCaseReturnsErrors(t *testing.T) {
	mux, fake := newServer(t)
	unauthorized := entities.UnauthorizedError()
	fake.PerformFunc = func(context.Context, useCases.ReadEnvInput) useCases.ReadEnvOutput {
		return useCases.ReadEnvOutput{Error: &unauthorized}
	}

	assert.Equal(t, http.StatusUnauthorized, get(mux, "/api/v1/runtime/abc/env", "").Code)
}
