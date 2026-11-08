package dotenv_test

import (
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/pkg/dotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	values := map[string]string{
		"ENVIRONMENT":         "dev",
		"SERVER_PORT":         ":8083",
		"SQLITE_PATH":         "vault.db",
		"AUTH_URL":            "https://auth.example.com",
		"AUTH_APPLICATION_ID": uuid.NewV7().String(),
		"ENCRYPTION_Key":      "",
		"ENCRYPTION_KEY":      "a2V5LWRlLXRlc3RlLWRlLTMyLWJ5dGVzLWFxdWk=",
	}
	for key, value := range overrides {
		values[key] = value
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}

func TestNew(t *testing.T) {
	t.Run("Quando a config é válida.", func(t *testing.T) { whenConfigIsValid(t) })
	t.Run("Quando falta uma chave obrigatória.", func(t *testing.T) { whenRequiredKeyIsMissing(t) })
	t.Run("Quando o AUTH_URL é inválido.", func(t *testing.T) { whenAuthURLIsInvalid(t) })
	t.Run("Quando o AUTH_APPLICATION_ID é inválido.", func(t *testing.T) { whenApplicationIDIsInvalid(t) })
	t.Run("Quando há chaves antigas.", func(t *testing.T) { whenThereArePreviousKeys(t) })
}

func whenConfigIsValid(t *testing.T) {
	setEnv(t, nil)

	cfg, err := dotenv.New()

	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", cfg.Auth.URL)
	assert.NotEqual(t, uuid.UUID{}, cfg.Auth.AuthApplicationID())
}

func whenRequiredKeyIsMissing(t *testing.T) {
	setEnv(t, map[string]string{"ENCRYPTION_KEY": ""})

	_, err := dotenv.New()

	assert.Error(t, err)
}

func whenAuthURLIsInvalid(t *testing.T) {
	for _, bad := range []string{"http://auth.example.com", "https://user:pass@auth.example.com", "https://auth.example.com/?x=1", "ftp://auth", "não é url"} {
		setEnv(t, map[string]string{"AUTH_URL": bad})

		_, err := dotenv.New()

		assert.Error(t, err, bad)
	}

	for _, good := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "https://auth.example.com"} {
		setEnv(t, map[string]string{"AUTH_URL": good})

		_, err := dotenv.New()

		assert.NoError(t, err, good)
	}
}

func whenApplicationIDIsInvalid(t *testing.T) {
	for _, bad := range []string{"abc", "00000000-0000-0000-0000-000000000000"} {
		setEnv(t, map[string]string{"AUTH_APPLICATION_ID": bad})

		_, err := dotenv.New()

		assert.Error(t, err, bad)
	}
}

func whenThereArePreviousKeys(t *testing.T) {
	setEnv(t, map[string]string{"ENCRYPTION_KEYS_PREVIOUS": " a , ,b "})

	cfg, err := dotenv.New()

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, cfg.Security.PreviousKeys())
}
