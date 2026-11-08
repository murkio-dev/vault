// Package authn verifica o access token que o Auth emite para as pessoas (JWT
// EdDSA, assinado com a chave Ed25519 da aplicação do Vault no Auth),
// localmente, com o JWKS público em {AUTH_URL}/v1/applications/{id}/jwks.json.
// Não chama o Auth por requisição: o JWKS fica em cache.
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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"
)

const (
	Issuer    = "enterprise-auth"
	Algorithm = "EdDSA"
	TokenType = "JWT"

	PlatformAdminRole = "admin"
)

const (
	leeway     = 30 * time.Second
	maxJWKS    = 64 << 10
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

func NewVerifier(baseURL string, appID uuid.UUID) *Verifier {
	return &Verifier{appID: appID, keys: newKeySet(baseURL, appID), now: time.Now}
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
	url    string
	client *http.Client
	// minRefresh é o intervalo mínimo entre buscas: um kid desconhecido (chave
	// nova, rotação) busca de novo, mas um token com kid inventado não vira uma
	// busca por requisição.
	minRefresh time.Duration

	mu        sync.Mutex
	keys      map[string]ed25519.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
}

func newKeySet(baseURL string, appID uuid.UUID) *KeySet {
	return &KeySet{
		url: strings.TrimRight(baseURL, "/") + "/v1/applications/" + appID.String() + "/jwks.json",
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		minRefresh: 30 * time.Second,
		ttl:        defaultTTL,
	}
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

type jwks struct {
	Keys []struct {
		Kty string `json:"kty"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Kid string `json:"kid"`
		Alg string `json:"alg"`
		Use string `json:"use"`
	} `json:"keys"`
}

func (k *KeySet) fetch(ctx context.Context) (map[string]ed25519.PublicKey, time.Duration, error) {

	ctx, cancel := context.WithTimeout(ctx, k.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("jwks: status %d", resp.StatusCode)
	}

	var doc jwks
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJWKS)).Decode(&doc); err != nil {
		return nil, 0, fmt.Errorf("jwks: %w", err)
	}

	keys := map[string]ed25519.PublicKey{}
	for _, jwk := range doc.Keys {
		if jwk.Kty != "OKP" || jwk.Crv != "Ed25519" || jwk.Kid == "" || (jwk.Alg != "" && jwk.Alg != Algorithm) || (jwk.Use != "" && jwk.Use != "sig") {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		keys[jwk.Kid] = ed25519.PublicKey(raw)
	}

	return keys, cacheTTL(resp.Header.Get("Cache-Control")), nil
}

// cacheTTL lê o max-age do Cache-Control do JWKS, limitado entre 30s e 1h. O
// Auth publica 5 minutos; sem o header ou com valor inválido, vale o padrão.
// O piso de 30s impede um Auth mal configurado de forçar uma busca por
// requisição, e o teto de 1h impede que uma rotação de chave demore a valer.
// Exemplo: "public, max-age=300" devolve 5min; "max-age=0" devolve 30s.
func cacheTTL(cacheControl string) time.Duration {
	for _, directive := range strings.Split(cacheControl, ",") {
		name, value, found := strings.Cut(strings.TrimSpace(directive), "=")
		if !found || !strings.EqualFold(name, "max-age") {
			continue
		}
		seconds, err := strconv.Atoi(strings.Trim(value, `"`))
		if err != nil || seconds < 0 {
			return defaultTTL
		}
		return min(max(time.Duration(seconds)*time.Second, minTTL), maxTTL)
	}
	return defaultTTL
}
