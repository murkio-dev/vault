package runtime

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/runtime/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/runtime/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	"google.golang.org/grpc"
)

func Init(deps contracts.Dependencies, mux *http.ServeMux) error {
	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:     mux,
		EnvUseCase: newReadEnvUseCase(deps),
	})
}

func InitGRPC(deps contracts.Dependencies, registrar grpc.ServiceRegistrar) error {
	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:  registrar,
		EnvUseCase: newReadEnvUseCase(deps),
	})
}

func newReadEnvUseCase(deps contracts.Dependencies) useCases.ReadEnvUseCase {
	return useCases.NewReadEnvUseCase(deps.Secrets, deps.Values, deps.Guard, deps.Tx, deps.Audit)
}
