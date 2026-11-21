package authn

import (
	"context"
	"net/http"
	"strings"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/httpx"
)

const (
	apiPrefix     = "/api/"
	runtimePrefix = "/api/v1/runtime/"
	TokenPrefix   = entities.SecretKeyPrefix
)

type TokenAuthenticator interface {
	Authenticate(ctx context.Context, raw string) (entities.Actor, error)
}

// Authenticate identifica quem faz a requisição e põe o ator no contexto.
//
//   - Fora de /api passa direto (nenhuma rota do Vault vive ali).
//   - /api/v1/runtime/* exige o token do SDK (sk_vt_...), e só ele.
//   - O resto de /api exige o JWT do Auth, e só ele: um token do SDK não
//     administra nada.
//
// Credencial ausente ou inválida responde 401 sempre igual, sem motivo. A
// resposta de toda a API sai com Cache-Control: no-store, porque nada que o
// Vault devolve pode ficar em cache de proxy ou de navegador.
func Authenticate(verifier *Verifier, tokens TokenAuthenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, apiPrefix) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-store")

		actor, ok := identify(r, verifier, tokens)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer`)
			httpx.RespondError(w, r, entities.UnauthorizedError())
			return
		}

		next.ServeHTTP(w, r.WithContext(entities.WithActor(r.Context(), actor)))
	})
}

func identify(r *http.Request, verifier *Verifier, tokens TokenAuthenticator) (entities.Actor, bool) {

	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return entities.Actor{}, false
	}

	return Identify(r.Context(), raw, strings.HasPrefix(r.URL.Path, runtimePrefix), verifier, tokens)
}

// Identify decide quem é o dono de uma credencial já separada do "Bearer ". É
// a regra única das duas entregas (o middleware HTTP e o interceptor gRPC),
// para que nenhuma delas aceite uma credencial que a outra recusa.
// Recebe o valor cru da credencial e sdkOnly, que diz o que o destino aceita:
// true é uma rota ou serviço de runtime, que só aceita o token do SDK (sk_vt_...);
// false é o resto, que só aceita o JWT de pessoa emitido pelo Auth.
// Devolve o ator e true quando a credencial vale para o destino. Credencial
// vazia, do tipo errado para o destino, inválida, revogada ou sem o verificador
// correspondente (verifier ou tokens nil) devolve o ator zero e false.
// Exemplo: "sk_vt_abc" com sdkOnly false devolve false sem consultar o banco, e um
// JWT válido com sdkOnly true também: um token do SDK não administra nada e o
// JWT de pessoa não lê o runtime.
// O token do SDK é conferido no banco a cada chamada, então revogar vale na hora.
func Identify(ctx context.Context, raw string, sdkOnly bool, verifier *Verifier, tokens TokenAuthenticator) (entities.Actor, bool) {

	if raw == "" {
		return entities.Actor{}, false
	}

	isSDKToken := strings.HasPrefix(raw, TokenPrefix)
	if isSDKToken != sdkOnly {
		return entities.Actor{}, false
	}

	if isSDKToken {
		if tokens == nil {
			return entities.Actor{}, false
		}
		actor, err := tokens.Authenticate(ctx, raw)
		if err != nil || actor.Kind != entities.ActorToken {
			return entities.Actor{}, false
		}
		return actor, true
	}

	if verifier == nil {
		return entities.Actor{}, false
	}
	principal, err := verifier.Verify(ctx, raw)
	if err != nil {
		return entities.Actor{}, false
	}

	return entities.Actor{
		Kind:          entities.ActorUser,
		UserID:        principal.UserID,
		SessionID:     principal.SessionID,
		PlatformAdmin: principal.HasRole(PlatformAdminRole),
	}, true
}
