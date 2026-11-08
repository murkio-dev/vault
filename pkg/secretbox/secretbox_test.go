package secretbox_test

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func key(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, secretbox.KeySize)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return k
}

func TestBox(t *testing.T) {
	t.Run("Quando cifra e abre com a mesma chave.", func(t *testing.T) { whenSealAndOpen(t) })
	t.Run("Quando o associatedData é outro.", func(t *testing.T) { whenAssociatedDataDiffers(t) })
	t.Run("Quando a chave foi rotacionada.", func(t *testing.T) { whenKeyWasRotated(t) })
	t.Run("Quando a chave é inválida.", func(t *testing.T) { whenKeyIsInvalid(t) })
	t.Run("Quando o valor cifrado é lixo.", func(t *testing.T) { whenSealedIsGarbage(t) })
}

func whenSealAndOpen(t *testing.T) {
	box, err := secretbox.New(key(t))
	require.NoError(t, err)

	first, err := box.Seal([]byte("postgres://user:pass@host/db"), []byte("secret_version:1"))
	require.NoError(t, err)
	second, err := box.Seal([]byte("postgres://user:pass@host/db"), []byte("secret_version:1"))
	require.NoError(t, err)

	assert.NotEqual(t, first, second, "o nonce é aleatório: o mesmo valor não gera o mesmo ciphertext")
	assert.NotContains(t, string(first), "postgres", "o valor não aparece em claro")

	plain, err := box.Open(first, []byte("secret_version:1"))
	require.NoError(t, err)
	assert.Equal(t, "postgres://user:pass@host/db", string(plain))
}

func whenAssociatedDataDiffers(t *testing.T) {
	box, err := secretbox.New(key(t))
	require.NoError(t, err)

	sealed, err := box.Seal([]byte("valor"), []byte("secret_version:1"))
	require.NoError(t, err)

	_, err = box.Open(sealed, []byte("secret_version:2"))
	assert.ErrorIs(t, err, secretbox.ErrInvalidSealed, "um valor copiado para outra linha não abre")
}

func whenKeyWasRotated(t *testing.T) {
	oldKey, newKey := key(t), key(t)
	oldBox, err := secretbox.New(oldKey)
	require.NoError(t, err)
	sealed, err := oldBox.Seal([]byte("valor"), []byte("ad"))
	require.NoError(t, err)

	rotated, err := secretbox.New(newKey, oldKey)
	require.NoError(t, err)

	plain, stale, err := rotated.OpenStale(sealed, []byte("ad"))
	require.NoError(t, err)
	assert.Equal(t, "valor", string(plain))
	assert.True(t, stale, "aberto com a chave antiga: precisa recifrar")

	resealed, err := rotated.Seal(plain, []byte("ad"))
	require.NoError(t, err)
	_, stale, err = rotated.OpenStale(resealed, []byte("ad"))
	require.NoError(t, err)
	assert.False(t, stale)

	withoutOld, err := secretbox.New(newKey)
	require.NoError(t, err)
	_, err = withoutOld.Open(sealed, []byte("ad"))
	assert.ErrorIs(t, err, secretbox.ErrInvalidSealed, "sem a chave antiga, o valor antigo não abre")
}

func whenKeyIsInvalid(t *testing.T) {
	_, err := secretbox.New([]byte("curta"))
	assert.ErrorIs(t, err, secretbox.ErrInvalidKey)

	k := key(t)
	_, err = secretbox.New(k, k)
	assert.ErrorIs(t, err, secretbox.ErrInvalidKey, "antiga igual à ativa é rotação mal feita")

	_, err = secretbox.FromBase64("nao-e-base64!!", nil)
	assert.ErrorIs(t, err, secretbox.ErrInvalidKey)

	_, err = secretbox.FromBase64(base64.StdEncoding.EncodeToString(k), []string{"lixo"})
	assert.ErrorIs(t, err, secretbox.ErrInvalidKey)

	_, err = secretbox.FromBase64("segredo-que-nao-pode-vazar", nil)
	assert.NotContains(t, err.Error(), "segredo-que-nao-pode-vazar", "o erro não repete a chave")
}

func whenSealedIsGarbage(t *testing.T) {
	box, err := secretbox.New(key(t))
	require.NoError(t, err)

	_, err = box.Open([]byte("x"), nil)
	assert.ErrorIs(t, err, secretbox.ErrInvalidSealed)
	_, err = box.Open(make([]byte, 64), nil)
	assert.ErrorIs(t, err, secretbox.ErrInvalidSealed)
}
