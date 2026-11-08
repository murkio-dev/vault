package authn

import (
	"context"
	"net/http"
	"strings"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

const (
	apiPrefix     = "/api/"
	runtimePrefix = "/api/v1/runtime/"
	TokenPrefix   = "vt_"
)

type TokenAuthenticator interface {
	Authenticate(ctx context.Context, raw string) (entities.Actor, error)
}

// Authenticate identifica quem faz a requisição e põe o ator no contexto.
//
//   - Fora de /api passa direto (nenhuma rota do Vault vive ali).
//   - /api/v1/runtime/* exige o token do SDK (vt_...), e só ele.
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
			utils.RespondError(w, r, entities.UnauthorizedError())
			return
		}

		next.ServeHTTP(w, r.WithContext(entities.WithActor(r.Context(), actor)))
	})
}

func identify(r *http.Request, verifier *Verifier, tokens TokenAuthenticator) (entities.Actor, bool) {

	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found || raw == "" {
		return entities.Actor{}, false
	}

	isSDKToken := strings.HasPrefix(raw, TokenPrefix)
	isRuntime := strings.HasPrefix(r.URL.Path, runtimePrefix)
	if isSDKToken != isRuntime {
		return entities.Actor{}, false
	}

	if isSDKToken {
		if tokens == nil {
			return entities.Actor{}, false
		}
		actor, err := tokens.Authenticate(r.Context(), raw)
		if err != nil || actor.Kind != entities.ActorToken {
			return entities.Actor{}, false
		}
		return actor, true
	}

	principal, err := verifier.Verify(r.Context(), raw)
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
