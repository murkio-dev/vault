package dotenv_test

import (
	"os"
	"path/filepath"
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
	t.Run("Quando a porta do gRPC é definida.", func(t *testing.T) { whenGRPCPortIsSet(t) })
	t.Run("Quando os caminhos são relativos.", func(t *testing.T) { whenPathsAreRelative(t) })
	t.Run("Quando há um .env na pasta atual.", func(t *testing.T) { whenEnvFileExists(t) })
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

func whenGRPCPortIsSet(t *testing.T) {
	setEnv(t, nil)
	cfg, err := dotenv.New()
	require.NoError(t, err)
	assert.Equal(t, "50052", cfg.Server.GRPCPort, "padrão")

	for _, value := range []string{"0", "70000", "abc", "8083"} {
		setEnv(t, map[string]string{"GRPC_PORT": value})

		_, err := dotenv.New()

		assert.Error(t, err, value)
	}
}

func whenPathsAreRelative(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	setEnv(t, map[string]string{"SQLITE_PATH": "dados/vault.db", "LOG_PATH": ""})

	cfg, err := dotenv.New()

	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(cfg.Database.SQLitePath), "o banco não depende da pasta de onde o serviço foi chamado")
	assert.Equal(t, filepath.Join(dir, "dados", "vault.db"), cfg.Database.SQLitePath)
	assert.Equal(t, ".log", filepath.Ext(cfg.Log.Path), "sem LOG_PATH o log é <binário>.log")
	assert.Equal(t, dir, filepath.Dir(cfg.Log.Path))

	absolute := filepath.Join(dir, "fora", "vault.db")
	setEnv(t, map[string]string{"SQLITE_PATH": absolute, "LOG_PATH": filepath.Join(dir, "x.log")})
	cfg, err = dotenv.New()
	require.NoError(t, err)
	assert.Equal(t, absolute, cfg.Database.SQLitePath, "caminho absoluto passa como está")
	assert.Equal(t, filepath.Join(dir, "x.log"), cfg.Log.Path)
}

func whenEnvFileExists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("VAULT_TEST_ONLY_FILE=do-arquivo\nVAULT_TEST_BOTH=do-arquivo\n"), 0o600))
	t.Setenv("VAULT_TEST_BOTH", "do-processo")
	_ = os.Unsetenv("VAULT_TEST_ONLY_FILE")
	t.Cleanup(func() { _ = os.Unsetenv("VAULT_TEST_ONLY_FILE") })

	require.NoError(t, dotenv.Load())

	assert.Equal(t, "do-arquivo", os.Getenv("VAULT_TEST_ONLY_FILE"), "o .env preenche o que falta")
	assert.Equal(t, "do-processo", os.Getenv("VAULT_TEST_BOTH"), "o ambiente do processo vale mais que o arquivo")
}
