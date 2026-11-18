package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/murkio-dev/vault/pkg/httpx"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestHandle(t *testing.T) {
	t.Run("Quando a rota é registrada com e sem barra final.", func(t *testing.T) { whenRouteIsRegisteredWithAndWithoutSlash(t) })
}

// Functions

func whenRouteIsRegisteredWithAndWithoutSlash(t *testing.T) {
	mux := http.NewServeMux()
	calls := 0
	httpx.Handle(mux, "GET /api/v1/x", func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) })
	httpx.Handle(mux, "GET /api/v1/y/{$}", func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) })

	for _, path := range []string{"/api/v1/x", "/api/v1/x/", "/api/v1/y/"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		assert.Equal(t, http.StatusNoContent, rec.Code, path)
	}
	assert.Equal(t, 3, calls)
}
