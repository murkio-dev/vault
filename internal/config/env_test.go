package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/murkio-dev/vault/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestLoad(t *testing.T) {
	t.Run("Quando há um .env na pasta atual.", func(t *testing.T) { whenEnvFileExists(t) })
}

// Functions

func whenEnvFileExists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("VAULT_TEST_ONLY_FILE=do-arquivo\nVAULT_TEST_BOTH=do-arquivo\n"), 0o600))
	t.Setenv("VAULT_TEST_BOTH", "do-processo")
	_ = os.Unsetenv("VAULT_TEST_ONLY_FILE")
	t.Cleanup(func() { _ = os.Unsetenv("VAULT_TEST_ONLY_FILE") })

	require.NoError(t, config.Load())

	assert.Equal(t, "do-arquivo", os.Getenv("VAULT_TEST_ONLY_FILE"), "o .env preenche o que falta")
	assert.Equal(t, "do-processo", os.Getenv("VAULT_TEST_BOTH"), "o ambiente do processo vale mais que o arquivo")
}
