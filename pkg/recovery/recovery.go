package recovery

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/utils"
)

// Recovery transforma um panic num handler numa linha de log (level error, com
// a pilha) e numa resposta 500 no formato RFC 9457 dos outros erros. Sem ele, o
// net/http derruba só a conexão, e o cliente fica sem resposta nem log. Se a
// resposta já começou a sair, só loga: não dá para trocar o status. O panic
// http.ErrAbortHandler é repassado, porque é como o net/http aborta de propósito.
// O texto do panic vai só para o log, nunca para o cliente.
func Recovery(logs *entities.Domain, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracker := &writeTracker{ResponseWriter: w}

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			// O net/http usa este valor para abortar a resposta de propósito.
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}

			_ = logs.Write(entities.Logs{
				Level:   "error",
				Type:    entities.TypeFunctionError,
				Service: "vault",
				Message: "panic recovered",
				Error:   fmt.Sprint(recovered),
				Origin:  string(debug.Stack()),
			})

			if !tracker.written {
				utils.RespondError(tracker, r, entities.InternalServerError(nil, nil))
			}
		}()

		next.ServeHTTP(tracker, r)
	})
}

type writeTracker struct {
	http.ResponseWriter
	written bool
}

func (t *writeTracker) WriteHeader(status int) {
	t.written = true
	t.ResponseWriter.WriteHeader(status)
}

func (t *writeTracker) Write(body []byte) (int, error) {
	t.written = true
	return t.ResponseWriter.Write(body)
}

func (t *writeTracker) Flush() {
	if flusher, ok := t.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
