package grpcx_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/authn/authntest"
	"github.com/murkio-dev/vault/pkg/grpcx"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	runtimeMethod = "/murkio.vault.v1.VaultService/ReadEnv"
	otherMethod   = "/murkio.vault.v1.ApplicationService/ListApplications"
)

// Main test

func TestAuthenticate(t *testing.T) {
	t.Run("Quando o JWT de pessoa chama um serviço comum.", func(t *testing.T) { whenPersonCallsRegularService(t) })
	t.Run("Quando o JWT de pessoa chama o runtime.", func(t *testing.T) { whenPersonCallsRuntime(t) })
	t.Run("Quando o token do SDK chama o runtime.", func(t *testing.T) { whenSDKTokenCallsRuntime(t) })
	t.Run("Quando o token do SDK chama um serviço comum.", func(t *testing.T) { whenSDKTokenCallsRegularService(t) })
	t.Run("Quando o token do SDK é recusado pelo banco.", func(t *testing.T) { whenSDKTokenIsRejected(t) })
	t.Run("Quando a credencial é inválida.", func(t *testing.T) { whenCredentialIsInvalid(t) })
	t.Run("Quando falta o verificador ou o autenticador.", func(t *testing.T) { whenAuthenticatorsAreNil(t) })
	t.Run("Quando o método vem sem informação.", func(t *testing.T) { whenInfoIsNil(t) })
}

// Functions

func withBearer(value string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", value))
}

func call(interceptor grpc.UnaryServerInterceptor, ctx context.Context, method string) (entities.Actor, bool, error) {
	var actor entities.Actor
	called := false

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		called = true
		actor = entities.ActorFrom(ctx)
		return nil, nil
	})

	return actor, called, err
}

func assertUnauthenticated(t *testing.T, called bool, err error) {
	t.Helper()

	assert.False(t, called, "a credencial recusada nunca chega ao handler")
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func whenPersonCallsRegularService(t *testing.T) {
	auth := authntest.New(t)
	user := uuid.NewV7()
	token := auth.Token(t, user, []string{"admin"}, time.Minute)

	actor, called, err := call(grpcx.Authenticate(auth.Verifier(), nil), withBearer("Bearer "+token), otherMethod)

	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, user, actor.UserID, "o handler recebe o ator no ctx")
}

func whenPersonCallsRuntime(t *testing.T) {
	auth := authntest.New(t)
	token := auth.Token(t, uuid.NewV7(), nil, time.Minute)

	_, called, err := call(grpcx.Authenticate(auth.Verifier(), nil), withBearer("Bearer "+token), runtimeMethod)

	assertUnauthenticated(t, called, err)
}

func whenSDKTokenCallsRuntime(t *testing.T) {
	tokens := &pkgmocks.TokenAuthenticator{Actor: entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7(), TokenID: uuid.NewV7()}}
	raw := authn.TokenPrefix + "abc"

	actor, called, err := call(grpcx.Authenticate(nil, tokens), withBearer("bearer "+raw), runtimeMethod)

	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, tokens.Actor.TokenID, actor.TokenID)
	assert.Equal(t, []string{raw}, tokens.Tokens, "o prefixo do esquema não diferencia maiúsculas")
}

func whenSDKTokenCallsRegularService(t *testing.T) {
	tokens := &pkgmocks.TokenAuthenticator{Actor: entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7()}}

	_, called, err := call(grpcx.Authenticate(nil, tokens), withBearer("Bearer "+authn.TokenPrefix+"abc"), otherMethod)

	assertUnauthenticated(t, called, err)
	assert.Empty(t, tokens.Tokens, "o token do SDK nem é conferido fora do runtime")
}

func whenSDKTokenIsRejected(t *testing.T) {
	tokens := &pkgmocks.TokenAuthenticator{Err: errors.New("revogado")}

	_, called, err := call(grpcx.Authenticate(nil, tokens), withBearer("Bearer "+authn.TokenPrefix+"abc"), runtimeMethod)

	assertUnauthenticated(t, called, err)
	assert.NotContains(t, status.Convert(err).Message(), "revogado", "sem motivo na resposta")
}

func whenCredentialIsInvalid(t *testing.T) {
	auth := authntest.New(t)
	interceptor := grpcx.Authenticate(auth.Verifier(), nil)
	two := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer a", "authorization", "Bearer b"))

	cases := map[string]context.Context{
		"sem metadata":      context.Background(),
		"sem authorization": metadata.NewIncomingContext(context.Background(), metadata.Pairs("x", "y")),
		"dois headers":      two,
		"esquema errado":    withBearer("Basic abcdef"),
		"só o esquema":      withBearer("Bearer "),
		"curto demais":      withBearer("Bear"),
		"JWT inválido":      withBearer("Bearer isso-nao-e-jwt"),
	}
	for name, ctx := range cases {
		_, called, err := call(interceptor, ctx, otherMethod)
		assert.False(t, called, name)
		assert.Equal(t, codes.Unauthenticated, status.Code(err), name)
	}
}

func whenAuthenticatorsAreNil(t *testing.T) {
	auth := authntest.New(t)
	token := auth.Token(t, uuid.NewV7(), nil, time.Minute)

	_, called, err := call(grpcx.Authenticate(nil, nil), withBearer("Bearer "+token), otherMethod)
	assertUnauthenticated(t, called, err)

	_, called, err = call(grpcx.Authenticate(nil, nil), withBearer("Bearer "+authn.TokenPrefix+"abc"), runtimeMethod)
	assertUnauthenticated(t, called, err)
}

func whenInfoIsNil(t *testing.T) {
	auth := authntest.New(t)
	token := auth.Token(t, uuid.NewV7(), nil, time.Minute)
	called := false

	_, err := grpcx.Authenticate(auth.Verifier(), nil)(withBearer("Bearer "+token), nil, nil, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})

	assert.NoError(t, err, "sem info vale a regra do JWT de pessoa")
	assert.True(t, called)
}
