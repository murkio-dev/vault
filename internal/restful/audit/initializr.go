package audit

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/audit/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/audit/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/audit/repositories"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
	"google.golang.org/grpc"
)

func Init(db *sql.DB, deps contracts.Dependencies, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:      mux,
		ListUseCase: useCases.NewListAuditLogsUseCase(repository, deps.Guard),
	})
}

func InitGRPC(db *sql.DB, deps contracts.Dependencies, registrar grpc.ServiceRegistrar) error {
	repository := repositories.NewRepository(db)

	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:   registrar,
		ListUseCase: useCases.NewListAuditLogsUseCase(repository, deps.Guard),
	})
}
