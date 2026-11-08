// Package authntest sobe um Auth falso para os testes: o JWKS da aplicação do
// Vault e access tokens assinados como o Auth assina. Fica fora do pacote
// authn para o binário não carregar o pacote testing.
package authntest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/pkg/authn"
)

type Auth struct {
	Server *httptest.Server
	AppID  uuid.UUID

	kid  string
	priv ed25519.PrivateKey
}

func New(t testing.TB) *Auth {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	a := &Auth{AppID: uuid.NewV7(), kid: uuid.NewV7().String(), priv: priv}
	a.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/applications/"+a.AppID.String()+"/jwks.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "OKP", "crv": "Ed25519", "x": base64.RawURLEncoding.EncodeToString(pub), "kid": a.kid, "alg": authn.Algorithm, "use": "sig",
		}}})
	}))
	t.Cleanup(a.Server.Close)

	return a
}

func (a *Auth) Verifier() *authn.Verifier {
	return authn.NewVerifier(a.Server.URL, a.AppID)
}

func (a *Auth) Token(t testing.TB, user uuid.UUID, roles []string, ttl time.Duration) string {
	t.Helper()
	return a.TokenFor(t, a.AppID, user, roles, ttl)
}

func (a *Auth) TokenFor(t testing.TB, app, user uuid.UUID, roles []string, ttl time.Duration) string {
	t.Helper()

	now := time.Now()
	h, _ := json.Marshal(map[string]string{"alg": authn.Algorithm, "typ": authn.TokenType, "kid": a.kid})
	c, _ := json.Marshal(map[string]any{
		"iss": authn.Issuer, "sub": user.String(), "aud": app.String(), "aid": app.String(),
		"sid": uuid.NewV7().String(), "roles": roles, "iat": now.Unix(), "exp": now.Add(ttl).Unix(),
	})

	input := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(a.priv, []byte(input)))
}
