package services_test

import (
	"net/http/httptest"
	"testing"

	"github.com/murkio-dev/vault/internal/restful/runtime/services"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestParseEnv(t *testing.T) {
	t.Run("Quando lê o uri_id e os nomes.", func(t *testing.T) { whenParsingURIAndNames(t) })
	t.Run("Quando não há filtro.", func(t *testing.T) { whenThereIsNoFilter(t) })
}

// Functions

func whenParsingURIAndNames(t *testing.T) {
	req := httptest.NewRequest("GET", "/?names=A,B", nil)
	req.SetPathValue("uri_id", "abc123")

	uri, names := services.ParseEnv(req)

	assert.Equal(t, "abc123", uri)
	assert.Equal(t, []string{"A", "B"}, names)
}

func whenThereIsNoFilter(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("uri_id", "abc")

	_, names := services.ParseEnv(req)

	assert.Nil(t, names)
}
