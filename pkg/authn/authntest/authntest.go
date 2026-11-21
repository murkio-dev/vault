// Package authntest sobe um Auth falso para os testes: o SigningKeyService do
// identity (o gRPC interno, em loopback) com o JWKS da aplicação do Vault, e
// access tokens assinados como o Auth assina. Fica fora do pacote authn para o
// binário não carregar o pacote testing.
package authntest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/pkg/authn"
	identityv1 "github.com/murkio-dev/vault/pkg/rpc/identityv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Auth struct {
	AppID uuid.UUID

	kid     string
	pub     ed25519.PublicKey
	priv    ed25519.PrivateKey
	server  *grpc.Server
	client  identityv1.SigningKeyServiceClient
	fetches atomic.Int32
}

func New(t testing.TB) *Auth {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	a := &Auth{AppID: uuid.NewV7(), kid: uuid.NewV7().String(), pub: pub, priv: priv, server: grpc.NewServer()}
	identityv1.RegisterSigningKeyServiceServer(a.server, signingKeys{auth: a})

	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = a.server.Serve(listener) }()
	t.Cleanup(a.server.Stop)

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	a.client = identityv1.NewSigningKeyServiceClient(conn)

	return a
}

// Client é o cliente do SigningKeyService falso, para montar um Verifier à mão.
func (a *Auth) Client() identityv1.SigningKeyServiceClient {
	return a.client
}

// Close derruba o identity falso: as buscas seguintes falham.
func (a *Auth) Close() {
	a.server.Stop()
}

// Fetches conta quantas vezes o JWKS foi pedido.
func (a *Auth) Fetches() int32 {
	return a.fetches.Load()
}

func (a *Auth) Verifier() *authn.Verifier {
	return authn.NewVerifier(a.client, a.AppID)
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

type signingKeys struct {
	identityv1.UnimplementedSigningKeyServiceServer
	auth *Auth
}

func (s signingKeys) GetJWKS(_ context.Context, req *identityv1.GetJWKSRequest) (*identityv1.JWKS, error) {
	s.auth.fetches.Add(1)
	if req.GetApplicationId() != s.auth.AppID.String() {
		return nil, status.Error(codes.NotFound, "application not found")
	}
	return &identityv1.JWKS{MaxAgeSeconds: 300, Keys: []*identityv1.JWK{{
		Kty: "OKP", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(s.auth.pub), Kid: s.auth.kid, Alg: authn.Algorithm, Use: "sig",
	}}}, nil
}
