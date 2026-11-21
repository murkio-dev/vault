package entities_test

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApiKey(t *testing.T) {
	t.Run("Quando gera cada tipo de chave.", func(t *testing.T) { whenKeyTypesAreGenerated(t) })
	t.Run("Quando a chave é inválida.", func(t *testing.T) { whenKeyIsInvalid(t) })
	t.Run("Quando calcula o fim da chave rotacionada.", func(t *testing.T) { whenRotationDeadlineIsComputed(t) })
}

func keyOwner() entities.Member {
	return entities.Member{ID: uuid.NewV7(), ApplicationID: uuid.NewV7(), UserID: uuid.NewV7(), UriID: "uri"}
}

func whenKeyTypesAreGenerated(t *testing.T) {
	owner := keyOwner()

	secret, secretRaw, err := entities.NewApiKey(owner, " ci ", entities.KeySecret, nil)
	require.NoError(t, err)
	public, publicRaw, err := entities.NewApiKey(owner, "web", entities.KeyPublishable, nil)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(secretRaw, entities.SecretKeyPrefix))
	assert.True(t, strings.HasPrefix(publicRaw, entities.PublicKeyPrefix))
	assert.Equal(t, secretRaw[:entities.KeyDisplayLength], secret.Prefix)
	assert.Equal(t, entities.HashToken(publicRaw), public.Hash, "guarda o hash, nunca o valor")
	assert.Equal(t, "ci", secret.Name, "o nome sai sem espaço nas pontas")
	assert.Equal(t, [3]uuid.UUID{owner.ApplicationID, owner.ID, owner.UserID}, [3]uuid.UUID{secret.ApplicationID, secret.MemberID, secret.UserID})
}

func whenKeyIsInvalid(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	far := time.Now().Add(entities.MaxKeyLifetime + time.Hour)

	_, _, badName := entities.NewApiKey(keyOwner(), "", entities.KeySecret, nil)
	_, _, badType := entities.NewApiKey(keyOwner(), "ci", entities.KeyType("admin"), nil)
	_, _, badPast := entities.NewApiKey(keyOwner(), "ci", entities.KeySecret, &past)
	_, _, badFar := entities.NewApiKey(keyOwner(), "ci", entities.KeySecret, &far)

	assert.ErrorIs(t, badName, entities.ErrInvalidKeyName)
	assert.ErrorIs(t, badType, entities.ErrInvalidKeyType)
	assert.ErrorIs(t, badPast, entities.ErrInvalidKeyExpiresAt)
	assert.ErrorIs(t, badFar, entities.ErrInvalidKeyExpiresAt)
}

func whenRotationDeadlineIsComputed(t *testing.T) {
	now := time.Now().UTC()
	soon := now.Add(time.Hour)

	open := entities.ApiKey{}
	expiring := entities.ApiKey{ExpiresAt: &soon}

	assert.True(t, open.RotationDeadline(now).Equal(now.Add(entities.RotationOverlap)), "sem vencimento, vale mais 72h")
	assert.True(t, expiring.RotationDeadline(now).Equal(soon), "nunca estende um vencimento mais cedo")
}
