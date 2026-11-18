package application

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/application/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/application/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/application/repositories"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	"google.golang.org/grpc"
)

func Init(db *sql.DB, deps contracts.Dependencies, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  useCases.NewCreateApplicationUseCase(repository, deps.Members, deps.Guard, deps.Tx),
		GetByIDUseCase: useCases.NewGetApplicationByIDUseCase(repository, deps.Guard),
		ListUseCase:    useCases.NewListApplicationsUseCase(repository),
		UpdateUseCase:  useCases.NewUpdateApplicationUseCase(repository, deps.Guard, deps.Tx),
		UpgradeUseCase: useCases.NewUpgradeApplicationUseCase(repository, deps.Guard, deps.Tx),
		DeleteUseCase:  useCases.NewDeleteApplicationUseCase(repository, deps.Guard, deps.Tx),
	})
}

func InitGRPC(db *sql.DB, deps contracts.Dependencies, registrar grpc.ServiceRegistrar) error {
	repository := repositories.NewRepository(db)

	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:      registrar,
		CreateUseCase:  useCases.NewCreateApplicationUseCase(repository, deps.Members, deps.Guard, deps.Tx),
		GetByIDUseCase: useCases.NewGetApplicationByIDUseCase(repository, deps.Guard),
		ListUseCase:    useCases.NewListApplicationsUseCase(repository),
		UpdateUseCase:  useCases.NewUpdateApplicationUseCase(repository, deps.Guard, deps.Tx),
		DeleteUseCase:  useCases.NewDeleteApplicationUseCase(repository, deps.Guard, deps.Tx),
	})
}
