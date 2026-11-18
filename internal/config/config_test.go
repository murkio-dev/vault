package config_test

import (
	"path/filepath"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	values := map[string]string{
		"APPLICATION_PORT":    "8083",
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
	t.Run("Quando a porta HTTP é inválida.", func(t *testing.T) { whenPortIsInvalid(t) })
	t.Run("Quando a porta do gRPC é definida.", func(t *testing.T) { whenGRPCPortIsSet(t) })
	t.Run("Quando os caminhos são relativos.", func(t *testing.T) { whenPathsAreRelative(t) })
	t.Run("Quando os caminhos são absolutos.", func(t *testing.T) { whenPathsAreAbsolute(t) })
	t.Run("Quando os caminhos não são definidos.", func(t *testing.T) { whenPathsUseTheDefaults(t) })
}

func whenConfigIsValid(t *testing.T) {
	setEnv(t, nil)

	cfg, err := config.New()

	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", cfg.Auth.URL)
	assert.NotEqual(t, uuid.UUID{}, cfg.Auth.AuthApplicationID())
}

func whenRequiredKeyIsMissing(t *testing.T) {
	setEnv(t, map[string]string{"ENCRYPTION_KEY": ""})

	_, err := config.New()

	assert.Error(t, err)
}

func whenAuthURLIsInvalid(t *testing.T) {
	for _, bad := range []string{"http://auth.example.com", "https://user:pass@auth.example.com", "https://auth.example.com/?x=1", "ftp://auth", "não é url"} {
		setEnv(t, map[string]string{"AUTH_URL": bad})

		_, err := config.New()

		assert.Error(t, err, bad)
	}

	for _, good := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "https://auth.example.com"} {
		setEnv(t, map[string]string{"AUTH_URL": good})

		_, err := config.New()

		assert.NoError(t, err, good)
	}
}

func whenApplicationIDIsInvalid(t *testing.T) {
	for _, bad := range []string{"abc", "00000000-0000-0000-0000-000000000000"} {
		setEnv(t, map[string]string{"AUTH_APPLICATION_ID": bad})

		_, err := config.New()

		assert.Error(t, err, bad)
	}
}

func whenThereArePreviousKeys(t *testing.T) {
	setEnv(t, map[string]string{"ENCRYPTION_KEYS_PREVIOUS": " a , ,b "})

	cfg, err := config.New()

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, cfg.Security.PreviousKeys())
}

func whenPortIsInvalid(t *testing.T) {
	for _, value := range []string{"0", "70000", "abc", ":8083"} {
		setEnv(t, map[string]string{"APPLICATION_PORT": value})

		_, err := config.New()

		assert.Error(t, err, value)
	}
}

func whenGRPCPortIsSet(t *testing.T) {
	setEnv(t, nil)
	cfg, err := config.New()
	require.NoError(t, err)
	assert.Equal(t, "50052", cfg.Server.GRPCPort, "padrão")

	for _, value := range []string{"0", "70000", "abc", "8083"} {
		setEnv(t, map[string]string{"GRPC_PORT": value})

		_, err := config.New()

		assert.Error(t, err, value)
	}
}

func whenPathsAreRelative(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	setEnv(t, map[string]string{"SQLITE_PATH": "dados/outro.db", "LOG_PATH": "logs/x.log"})

	cfg, err := config.New()

	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(cfg.Database.SQLitePath), "o banco não depende da pasta de onde o serviço foi chamado")
	assert.Equal(t, filepath.Join(dir, "dados", "outro.db"), cfg.Database.SQLitePath, "relativo vale a partir da pasta do binário")
	assert.Equal(t, filepath.Join(dir, "logs", "x.log"), cfg.Log.Path)
}

func whenPathsAreAbsolute(t *testing.T) {
	dir := t.TempDir()
	database, logFile := filepath.Join(dir, "estado", "cofre.db"), filepath.Join(dir, "estado", "cofre.log")
	setEnv(t, map[string]string{"SQLITE_PATH": database, "LOG_PATH": logFile})

	cfg, err := config.New()

	require.NoError(t, err)
	assert.Equal(t, database, cfg.Database.SQLitePath, "absoluto passa como está")
	assert.Equal(t, logFile, cfg.Log.Path)

	setEnv(t, map[string]string{"SQLITE_PATH": "file::memory:?cache=shared"})
	memory, err := config.New()
	require.NoError(t, err)
	assert.Equal(t, "file::memory:?cache=shared", memory.Database.SQLitePath, "URI file: não vira caminho")
}

func whenPathsUseTheDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	setEnv(t, map[string]string{"SQLITE_PATH": "", "LOG_PATH": "   "})

	cfg, err := config.New()

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "vault.db"), cfg.Database.SQLitePath, "sem valor, o banco é o vault.db ao lado do binário")
	assert.Equal(t, filepath.Join(dir, "vault.log"), cfg.Log.Path, "sem valor, o log é o vault.log ao lado do binário")
}
