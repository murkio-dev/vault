package recovery_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/recovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecovery(t *testing.T) {
	t.Run("Quando o handler dá panic antes de responder.", func(t *testing.T) { whenHandlerPanicsBeforeResponding(t) })
	t.Run("Quando o handler dá panic depois de responder.", func(t *testing.T) { whenHandlerPanicsAfterResponding(t) })
	t.Run("Quando o handler não dá panic.", func(t *testing.T) { whenHandlerDoesNotPanic(t) })
}

func newLogger(t *testing.T) (*entities.Domain, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault.log")
	return entities.NewLogger(context.Background(), path), path
}

func whenHandlerPanicsBeforeResponding(t *testing.T) {
	logs, path := newLogger(t)
	handler := recovery.Recovery(logs, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("segredo-que-nao-pode-vazar")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "segredo-que-nao-pode-vazar", "o panic vai para o log, nunca para o cliente")

	logged, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(logged), "panic recovered")
	assert.Contains(t, string(logged), "segredo-que-nao-pode-vazar")
}

func whenHandlerPanicsAfterResponding(t *testing.T) {
	logs, path := newLogger(t)
	handler := recovery.Recovery(logs, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		panic("depois")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusCreated, rec.Code, "o status que já saiu não é trocado")
	logged, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(logged), "panic recovered")
}

func whenHandlerDoesNotPanic(t *testing.T) {
	logs, path := newLogger(t)
	handler := recovery.Recovery(logs, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "sem panic, nada é logado")
}
