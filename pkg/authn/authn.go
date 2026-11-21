// Package authn verifica o access token que o Auth emite para as pessoas (JWT
// EdDSA, assinado com a chave Ed25519 da aplicação do Vault no Auth),
// localmente, com o JWKS público que o identity publica no gRPC interno
// (SigningKeyService.GetJWKS, em IDENTITY_GRPC_ADDR, só loopback). Não chama o
// Auth por requisição: o JWKS fica em cache.
//
// Diferente do messaging, que aceita qualquer aplicação, o Vault confia em uma
// só: a configurada em AUTH_APPLICATION_ID. O aid de um token ainda não
// verificado nunca escolhe de onde buscar chaves, então um token forjado não
// faz o Vault consultar o Auth por aplicações inventadas.
//
// O token vive 15 minutos e não há revogação de sessão aqui. Quem precisa de
// efeito imediato (remover um membro, suspender) confere o banco do Vault a
// cada requisição, depois de verificar o token.
package authn

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"uuid"

	identityv1 "github.com/murkio-dev/vault/pkg/rpc/identityv1"
)

const (
	Issuer    = "enterprise-auth"
	Algorithm = "EdDSA"
	TokenType = "JWT"

	PlatformAdminRole = "admin"
)

const (
	leeway     = 30 * time.Second
	timeout    = 5 * time.Second
	maxToken   = 8 << 10
	minTTL     = 30 * time.Second
	maxTTL     = time.Hour
	defaultTTL = 5 * time.Minute
)

var ErrInvalidToken = errors.New("invalid access token")

type Principal struct {
	UserID    uuid.UUID
	SessionID string
	Roles     []string
	ExpiresAt time.Time
}

func (p Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

type claims struct {
	Issuer        string   `json:"iss"`
	Subject       string   `json:"sub"`
	Audience      string   `json:"aud"`
	ApplicationID string   `json:"aid"`
	SessionID     string   `json:"sid"`
	Roles         []string `json:"roles"`
	IssuedAt      int64    `json:"iat"`
	ExpiresAt     int64    `json:"exp"`
}

type Verifier struct {
	appID uuid.UUID
	keys  *KeySet
	now   func() time.Time
}

// NewVerifier recebe o cliente do SigningKeyService do identity (o gRPC
// interno) e a aplicação do Vault no Auth, a única de onde vêm chaves.
func NewVerifier(identity identityv1.SigningKeyServiceClient, appID uuid.UUID) *Verifier {
	return &Verifier{appID: appID, keys: newKeySet(identity, appID), now: time.Now}
}

// Verify confere o token e devolve quem é. Confere o formato, o algoritmo
// (só EdDSA), a assinatura contra a chave do kid, o emissor, a audiência e o
// aid (que precisam ser a aplicação do Vault), o vencimento e a emissão com
// uma folga de 30s. Nenhum erro diz o motivo: dizer qual foi ajudaria quem
// tenta forjar um token. Exemplo: token de usuário final de outra aplicação do
// Auth falha, mesmo assinado pela mesma chave.
func (v *Verifier) Verify(ctx context.Context, token string) (Principal, error) {

	if token == "" || len(token) > maxToken {
		return Principal{}, ErrInvalidToken
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Principal{}, ErrInvalidToken
	}

	var h header
	if err := decodeJSON(parts[0], &h); err != nil || h.Alg != Algorithm || h.Typ != TokenType || h.Kid == "" {
		return Principal{}, ErrInvalidToken
	}

	var c claims
	if err := decodeJSON(parts[1], &c); err != nil {
		return Principal{}, ErrInvalidToken
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Principal{}, ErrInvalidToken
	}

	key, err := v.keys.Key(ctx, h.Kid)
	if err != nil {
		return Principal{}, err
	}

	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), signature) {
		return Principal{}, ErrInvalidToken
	}

	now := v.now()
	expires := time.Unix(c.ExpiresAt, 0)
	app := v.appID.String()
	if c.Issuer != Issuer || c.Audience != app || c.ApplicationID != app || c.ExpiresAt == 0 ||
		now.After(expires.Add(leeway)) || time.Unix(c.IssuedAt, 0).After(now.Add(leeway)) {
		return Principal{}, ErrInvalidToken
	}

	userID, err := uuid.Parse(c.Subject)
	if err != nil || userID == (uuid.UUID{}) {
		return Principal{}, ErrInvalidToken
	}

	return Principal{UserID: userID, SessionID: c.SessionID, Roles: c.Roles, ExpiresAt: expires}, nil
}

func decodeJSON(part string, out any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

type KeySet struct {
	identity identityv1.SigningKeyServiceClient
	appID    uuid.UUID
	// minRefresh é o intervalo mínimo entre buscas: um kid desconhecido (chave
	// nova, rotação) busca de novo, mas um token com kid inventado não vira uma
	// busca por requisição.
	minRefresh time.Duration

	mu        sync.Mutex
	keys      map[string]ed25519.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
}

func newKeySet(identity identityv1.SigningKeyServiceClient, appID uuid.UUID) *KeySet {
	return &KeySet{identity: identity, appID: appID, minRefresh: 30 * time.Second, ttl: defaultTTL}
}

// Key devolve a chave pública kid. Com o Auth fora, as chaves já em cache
// continuam valendo; sem nenhuma, o token é recusado: falha fechada, nunca
// aceita sem conferir. Um kid desconhecido busca o JWKS de novo (chave nova,
// rotação), mas no máximo uma vez por minRefresh, para um token com kid
// inventado não virar uma busca por requisição. Uma busca que falha também
// conta como busca, então o Auth fora não é martelado.
func (k *KeySet) Key(ctx context.Context, kid string) (ed25519.PublicKey, error) {

	k.mu.Lock()
	defer k.mu.Unlock()

	age := time.Since(k.fetchedAt)
	key, known := k.keys[kid]

	if k.fetchedAt.IsZero() || age > k.ttl || (!known && age > k.minRefresh) {
		// Uma busca que falha também conta como busca: com o Auth fora, a próxima
		// tentativa espera minRefresh.
		keys, ttl, err := k.fetch(ctx)
		k.fetchedAt = time.Now()
		if err == nil {
			k.keys, k.ttl = keys, ttl
		}
		key, known = k.keys[kid]
	}

	if !known {
		return nil, ErrInvalidToken
	}
	return key, nil
}

func (k *KeySet) fetch(ctx context.Context) (map[string]ed25519.PublicKey, time.Duration, error) {

	if k.identity == nil {
		return nil, 0, errors.New("jwks: identity client is not set")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := k.identity.GetJWKS(ctx, &identityv1.GetJWKSRequest{ApplicationId: k.appID.String()})
	if err != nil {
		return nil, 0, err
	}

	keys := map[string]ed25519.PublicKey{}
	for _, jwk := range response.GetKeys() {
		if jwk.GetKty() != "OKP" || jwk.GetCrv() != "Ed25519" || jwk.GetKid() == "" || (jwk.GetAlg() != "" && jwk.GetAlg() != Algorithm) || (jwk.GetUse() != "" && jwk.GetUse() != "sig") {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(jwk.GetX())
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		keys[jwk.GetKid()] = ed25519.PublicKey(raw)
	}

	return keys, cacheTTL(response.GetMaxAgeSeconds()), nil
}

// cacheTTL é o max_age_seconds do JWKS, limitado entre 30s e 1h. O Auth publica
// 5 minutos; zero ou negativo vale o padrão. O piso de 30s impede um Auth mal
// configurado de forçar uma busca por requisição, e o teto de 1h impede que uma
// rotação de chave demore a valer.
// Exemplo: 300 devolve 5min; 1 devolve 30s.
func cacheTTL(seconds int32) time.Duration {
	if seconds <= 0 {
		return defaultTTL
	}
	return min(max(time.Duration(seconds)*time.Second, minTTL), maxTTL)
}
