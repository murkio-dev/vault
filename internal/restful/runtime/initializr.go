package runtime

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/runtime/controllers"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
)

func Init(shared contracts.Shared, mux *http.ServeMux) error {
	return controllers.RegisterControllers(controllers.RegisterInput{
		Router:     mux,
		EnvUseCase: useCases.NewReadEnvUseCase(shared.SecretLister, shared.Values, shared.Guard, shared.Tx, shared.AuditCreator),
	})
}
