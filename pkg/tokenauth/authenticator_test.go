package tokenauth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/token/mocks"
	"github.com/murkio-dev/vault/pkg/tokenauth"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestAuthenticator(t *testing.T) {
	t.Run("Quando o token é válido.", func(t *testing.T) { whenTokenIsValid(t) })
	t.Run("Quando o formato do token é inválido.", func(t *testing.T) { whenTokenFormatIsInvalid(t) })
	t.Run("Quando o token não vale mais.", func(t *testing.T) { whenTokenIsNotValidAnymore(t) })
	t.Run("Quando o banco falha.", func(t *testing.T) { whenRepositoryFails(t) })
	t.Run("Quando gravar o último uso falha.", func(t *testing.T) { whenTouchFails(t) })
}

// Functions

func validToken() string { return entities.SDKTokenPrefix + strings.Repeat("A", 43) }

func found(token entities.AccessToken) *mocks.TokenRepo {
	return &mocks.TokenRepo{FindActiveByHashFunc: func(context.Context, string) (entities.AccessToken, error) {
		return token, nil
	}}
}

func whenTokenIsValid(t *testing.T) {
	token := mocks.NewToken()
	token.UserID = uuid.NewV7()
	repo := found(token)

	actor, err := tokenauth.NewAuthenticator(repo, repo).Authenticate(context.Background(), validToken())

	assert.NoError(t, err)
	assert.Equal(t, entities.ActorToken, actor.Kind)
	assert.Equal(t, token.UserID, actor.UserID)
	assert.Equal(t, token.MemberID, actor.MemberID)
	assert.Equal(t, token.ApplicationID, actor.ApplicationID)
	assert.Equal(t, token.ID, actor.TokenID)
	assert.Equal(t, []string{entities.HashToken(validToken())}, repo.Hashes, "a busca é pelo hash, nunca pelo token")
	assert.Equal(t, token.ApplicationID, repo.Application(), "o último uso é gravado na aplicação do token")
	if assert.Len(t, repo.Touched, 1) {
		assert.Equal(t, token.ID, repo.Touched[0].ID)
		assert.NotNil(t, repo.Touched[0].LastUsedAt)
	}
}

func whenTokenFormatIsInvalid(t *testing.T) {
	repo := &mocks.TokenRepo{FindActiveByHashFunc: func(context.Context, string) (entities.AccessToken, error) {
		t.Error("formato inválido nem chega ao banco")
		return entities.AccessToken{}, nil
	}}

	for name, raw := range map[string]string{"vazio": "", "sem prefixo": "abc", "jwt": "eyJhbGciOiJFZERTQSJ9.e30.x", "gigante": entities.SDKTokenPrefix + strings.Repeat("A", 200)} {
		_, err := tokenauth.NewAuthenticator(repo, repo).Authenticate(context.Background(), raw)

		assert.Error(t, err, name)
	}
}

func whenTokenIsNotValidAnymore(t *testing.T) {
	repo := &mocks.TokenRepo{}

	actor, err := tokenauth.NewAuthenticator(repo, repo).Authenticate(context.Background(), validToken())

	assert.Error(t, err, "o repositório só devolve o token que vale agora")
	assert.False(t, actor.Authenticated())
	assert.Empty(t, repo.Touched, "token recusado não marca uso")
}

func whenRepositoryFails(t *testing.T) {
	repo := &mocks.TokenRepo{FindActiveByHashFunc: func(context.Context, string) (entities.AccessToken, error) {
		return entities.AccessToken{}, errors.New("segredo-interno")
	}}

	_, err := tokenauth.NewAuthenticator(repo, repo).Authenticate(context.Background(), validToken())

	assert.Error(t, err, "erro de banco também nega")
}

func whenTouchFails(t *testing.T) {
	repo := found(mocks.NewToken())
	repo.TouchLastUsedFunc = func(context.Context, entities.AccessToken) (entities.AccessToken, error) {
		return entities.AccessToken{}, errors.New("disk full")
	}

	actor, err := tokenauth.NewAuthenticator(repo, repo).Authenticate(context.Background(), validToken())

	assert.NoError(t, err, "o último uso é informação, não segurança")
	assert.True(t, actor.Authenticated())
}
