package authn_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/authn/authntest"
	"github.com/stretchr/testify/assert"
)

type fakeTokens struct {
	actor entities.Actor
	err   error
}

func (f fakeTokens) Authenticate(context.Context, string) (entities.Actor, error) {
	return f.actor, f.err
}

func TestAuthenticate(t *testing.T) {
	t.Run("Quando a pessoa chega com um JWT válido.", func(t *testing.T) { whenUserHasValidJWT(t) })
	t.Run("Quando a pessoa tem a role admin do Auth.", func(t *testing.T) { whenUserIsPlatformAdmin(t) })
	t.Run("Quando não há credencial ou ela é inválida.", func(t *testing.T) { whenCredentialIsMissingOrInvalid(t) })
	t.Run("Quando o token do SDK chega fora do runtime.", func(t *testing.T) { whenSDKTokenIsUsedOutsideRuntime(t) })
	t.Run("Quando o JWT chega no runtime.", func(t *testing.T) { whenJWTIsUsedInRuntime(t) })
	t.Run("Quando o token do SDK é válido.", func(t *testing.T) { whenSDKTokenIsValid(t) })
	t.Run("Quando o token do SDK é revogado.", func(t *testing.T) { whenSDKTokenIsRevoked(t) })
	t.Run("Quando a rota está fora de /api.", func(t *testing.T) { whenRouteIsOutsideApi(t) })
	t.Run("Quando a credencial crua é identificada.", func(t *testing.T) { whenRawCredentialIsIdentified(t) })
	t.Run("Quando falta o verificador da credencial.", func(t *testing.T) { whenIdentifyHasNoVerifier(t) })
}

func serve(handler http.Handler, path, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func capture(seen *entities.Actor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = entities.ActorFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
}

func whenUserHasValidJWT(t *testing.T) {
	auth := authntest.New(t)
	user := uuid.NewV7()
	var seen entities.Actor
	handler := authn.Authenticate(auth.Verifier(), nil, capture(&seen))

	rec := serve(handler, "/api/v1/applications", "Bearer "+auth.Token(t, user, nil, time.Minute))

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, entities.ActorUser, seen.Kind)
	assert.Equal(t, user, seen.UserID)
	assert.False(t, seen.PlatformAdmin)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func whenUserIsPlatformAdmin(t *testing.T) {
	auth := authntest.New(t)
	var seen entities.Actor
	handler := authn.Authenticate(auth.Verifier(), nil, capture(&seen))

	serve(handler, "/api/v1/applications", "Bearer "+auth.Token(t, uuid.NewV7(), []string{"admin"}, time.Minute))

	assert.True(t, seen.PlatformAdmin)
}

func whenCredentialIsMissingOrInvalid(t *testing.T) {
	auth := authntest.New(t)
	called := false
	handler := authn.Authenticate(auth.Verifier(), nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	for _, header := range []string{"", "Bearer ", "Bearer lixo", "Basic abc", "bearer " + auth.Token(t, uuid.NewV7(), nil, time.Minute),
		// chave de outro serviço, a pública reservada do Vault e o formato antigo (vt_) não valem
		"Bearer sk_id_abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrst", "Bearer sk_ms_abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrst", "Bearer pk_vt_abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrst", "Bearer vt_abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrst"} {
		rec := serve(handler, "/api/v1/applications", header)

		assert.Equal(t, http.StatusUnauthorized, rec.Code, header)
		assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
		assert.Contains(t, rec.Body.String(), entities.ErrTypeUnauthorized)
	}
	assert.False(t, called, "sem credencial, o handler nunca roda")

	expired := serve(handler, "/api/v1/applications", "Bearer "+auth.Token(t, uuid.NewV7(), nil, -time.Minute))
	assert.Equal(t, http.StatusUnauthorized, expired.Code)
}

func whenSDKTokenIsUsedOutsideRuntime(t *testing.T) {
	auth := authntest.New(t)
	tokens := fakeTokens{actor: entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7()}}
	handler := authn.Authenticate(auth.Verifier(), tokens, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("o token do SDK não administra nada")
	}))

	rec := serve(handler, "/api/v1/applications", "Bearer sk_vt_abc")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func whenJWTIsUsedInRuntime(t *testing.T) {
	auth := authntest.New(t)
	handler := authn.Authenticate(auth.Verifier(), fakeTokens{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("o runtime só aceita o token do SDK")
	}))

	rec := serve(handler, "/api/v1/runtime/abc/env", "Bearer "+auth.Token(t, uuid.NewV7(), nil, time.Minute))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func whenSDKTokenIsValid(t *testing.T) {
	auth := authntest.New(t)
	member := uuid.NewV7()
	tokens := fakeTokens{actor: entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7(), TokenID: uuid.NewV7(), MemberID: member}}
	var seen entities.Actor
	handler := authn.Authenticate(auth.Verifier(), tokens, capture(&seen))

	rec := serve(handler, "/api/v1/runtime/abc/env", "Bearer sk_vt_abc")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, entities.ActorToken, seen.Kind)
	assert.Equal(t, member, seen.MemberID)
}

func whenSDKTokenIsRevoked(t *testing.T) {
	auth := authntest.New(t)
	handler := authn.Authenticate(auth.Verifier(), fakeTokens{err: errors.New("revoked")}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("token revogado não passa")
	}))

	rec := serve(handler, "/api/v1/runtime/abc/env", "Bearer sk_vt_abc")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.NotContains(t, rec.Body.String(), "revoked", "o motivo não vai para o cliente")

	noTokens := authn.Authenticate(auth.Verifier(), nil, http.NotFoundHandler())
	assert.Equal(t, http.StatusUnauthorized, serve(noTokens, "/api/v1/runtime/abc/env", "Bearer sk_vt_abc").Code)
}

func whenRouteIsOutsideApi(t *testing.T) {
	auth := authntest.New(t)
	handler := authn.Authenticate(auth.Verifier(), nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := serve(handler, "/v1/fora-da-api", "")

	assert.Equal(t, http.StatusOK, rec.Code, "fora de /api não pede credencial")
}

func whenRawCredentialIsIdentified(t *testing.T) {
	auth := authntest.New(t)
	user := uuid.NewV7()
	jwt := auth.Token(t, user, nil, time.Minute)
	sdkActor := entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7(), TokenID: uuid.NewV7()}
	tokens := fakeTokens{actor: sdkActor}
	ctx := context.Background()

	person, personOK := authn.Identify(ctx, jwt, false, auth.Verifier(), tokens)
	sdk, sdkOK := authn.Identify(ctx, "sk_vt_qualquer", true, auth.Verifier(), tokens)
	_, jwtInRuntime := authn.Identify(ctx, jwt, true, auth.Verifier(), tokens)
	_, sdkOutsideRuntime := authn.Identify(ctx, "sk_vt_qualquer", false, auth.Verifier(), tokens)
	_, empty := authn.Identify(ctx, "", false, auth.Verifier(), tokens)
	_, garbage := authn.Identify(ctx, "nao.e.jwt", false, auth.Verifier(), tokens)
	_, wrongKind := authn.Identify(ctx, "sk_vt_qualquer", true, auth.Verifier(), fakeTokens{actor: entities.Actor{Kind: entities.ActorUser, UserID: user}})

	assert.True(t, personOK)
	assert.Equal(t, entities.ActorUser, person.Kind)
	assert.Equal(t, user, person.UserID)
	assert.True(t, sdkOK)
	assert.Equal(t, sdkActor, sdk)
	assert.False(t, jwtInRuntime, "o JWT de pessoa não lê o runtime")
	assert.False(t, sdkOutsideRuntime, "o token do SDK não administra nada")
	assert.False(t, empty)
	assert.False(t, garbage)
	assert.False(t, wrongKind, "o autenticador de tokens só pode devolver um ator de token")
}

func whenIdentifyHasNoVerifier(t *testing.T) {
	auth := authntest.New(t)
	jwt := auth.Token(t, uuid.NewV7(), nil, time.Minute)

	_, withoutVerifier := authn.Identify(context.Background(), jwt, false, nil, fakeTokens{})
	_, withoutTokens := authn.Identify(context.Background(), "sk_vt_qualquer", true, auth.Verifier(), nil)

	assert.False(t, withoutVerifier, "sem verificador, recusa em vez de entrar em pânico")
	assert.False(t, withoutTokens)
}
