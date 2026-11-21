package authn_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/authn/authntest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifier(t *testing.T) {
	t.Run("Quando o token é válido.", func(t *testing.T) { whenTokenIsValid(t) })
	t.Run("Quando o token está vencido.", func(t *testing.T) { whenTokenIsExpired(t) })
	t.Run("Quando o token é de outra aplicação.", func(t *testing.T) { whenTokenIsFromAnotherApplication(t) })
	t.Run("Quando a assinatura foi adulterada.", func(t *testing.T) { whenSignatureIsTampered(t) })
	t.Run("Quando o token é lixo.", func(t *testing.T) { whenTokenIsGarbage(t) })
	t.Run("Quando o Auth está fora.", func(t *testing.T) { whenAuthIsDown(t) })
	t.Run("Quando o identity não conhece a aplicação.", func(t *testing.T) { whenApplicationIsUnknown(t) })
	t.Run("Quando há várias verificações seguidas.", func(t *testing.T) { whenKeysAreCached(t) })
}

func whenTokenIsValid(t *testing.T) {
	auth := authntest.New(t)
	user := uuid.NewV7()

	principal, err := auth.Verifier().Verify(context.Background(), auth.Token(t, user, []string{"admin"}, time.Minute))

	require.NoError(t, err)
	assert.Equal(t, user, principal.UserID)
	assert.True(t, principal.HasRole(authn.PlatformAdminRole))
	assert.False(t, principal.HasRole("outra"))
}

func whenTokenIsExpired(t *testing.T) {
	auth := authntest.New(t)

	_, err := auth.Verifier().Verify(context.Background(), auth.Token(t, uuid.NewV7(), nil, -time.Minute))

	assert.ErrorIs(t, err, authn.ErrInvalidToken)
}

func whenTokenIsFromAnotherApplication(t *testing.T) {
	auth := authntest.New(t)

	_, err := auth.Verifier().Verify(context.Background(), auth.TokenFor(t, uuid.NewV7(), uuid.NewV7(), nil, time.Minute))

	assert.ErrorIs(t, err, authn.ErrInvalidToken, "o token de um usuário final de outra aplicação não abre o Vault")
}

func whenSignatureIsTampered(t *testing.T) {
	auth := authntest.New(t)
	token := auth.Token(t, uuid.NewV7(), nil, time.Minute)
	parts := strings.Split(token, ".")
	forged := parts[0] + "." + parts[1] + "." + strings.Repeat("A", len(parts[2]))

	_, err := auth.Verifier().Verify(context.Background(), forged)

	assert.ErrorIs(t, err, authn.ErrInvalidToken)
}

func whenTokenIsGarbage(t *testing.T) {
	verifier := authntest.New(t).Verifier()

	for _, token := range []string{"", "a.b", "a.b.c", "sk_vt_abc", strings.Repeat("a", 9<<10)} {
		_, err := verifier.Verify(context.Background(), token)
		assert.ErrorIs(t, err, authn.ErrInvalidToken)
	}
}

func whenAuthIsDown(t *testing.T) {
	auth := authntest.New(t)
	token := auth.Token(t, uuid.NewV7(), nil, time.Minute)
	verifier := auth.Verifier()
	auth.Close()

	_, err := verifier.Verify(context.Background(), token)

	assert.ErrorIs(t, err, authn.ErrInvalidToken, "falha fechada: sem as chaves, nada é aceito")
}

func whenApplicationIsUnknown(t *testing.T) {
	auth := authntest.New(t)
	other := uuid.NewV7()

	_, err := authn.NewVerifier(auth.Client(), other).Verify(context.Background(), auth.TokenFor(t, other, uuid.NewV7(), nil, time.Minute))
	_, noClient := authn.NewVerifier(nil, auth.AppID).Verify(context.Background(), auth.Token(t, uuid.NewV7(), nil, time.Minute))

	assert.ErrorIs(t, err, authn.ErrInvalidToken, "o identity não tem chaves dessa aplicação")
	assert.ErrorIs(t, noClient, authn.ErrInvalidToken, "sem cliente do identity, nada é aceito")
}

func whenKeysAreCached(t *testing.T) {
	auth := authntest.New(t)
	verifier := auth.Verifier()

	for range 5 {
		_, err := verifier.Verify(context.Background(), auth.Token(t, uuid.NewV7(), nil, time.Minute))
		require.NoError(t, err)
	}

	assert.EqualValues(t, 1, auth.Fetches(), "o JWKS fica em cache")
}
