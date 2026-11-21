package apikeys

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/apikeys/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/apikeys/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/apikeys/repositories"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
	"google.golang.org/grpc"
)

func Init(db *sql.DB, deps contracts.Dependencies, publicURL string, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  useCases.NewCreateApiKeyUseCase(repository, deps.Members, deps.Guard, deps.Tx, publicURL),
		ListUseCase:    useCases.NewListApiKeysUseCase(repository, deps.Guard),
		GetUseCase:     useCases.NewGetApiKeyByIDUseCase(repository, deps.Guard),
		UpdateUseCase:  useCases.NewUpdateApiKeyByIDUseCase(repository, deps.Guard, deps.Tx),
		UpgradeUseCase: useCases.NewUpgradeApiKeyByIDUseCase(repository, deps.Guard, deps.Tx),
		DeleteUseCase:  useCases.NewDeleteApiKeyByIDUseCase(repository, deps.Guard, deps.Tx),
		RotateUseCase:  useCases.NewRotateApiKeyByIDUseCase(repository, deps.Members, deps.Guard, deps.Tx, publicURL),
	})
}

func InitGRPC(db *sql.DB, deps contracts.Dependencies, publicURL string, registrar grpc.ServiceRegistrar) error {
	repository := repositories.NewRepository(db)

	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:      registrar,
		CreateUseCase:  useCases.NewCreateApiKeyUseCase(repository, deps.Members, deps.Guard, deps.Tx, publicURL),
		ListUseCase:    useCases.NewListApiKeysUseCase(repository, deps.Guard),
		GetUseCase:     useCases.NewGetApiKeyByIDUseCase(repository, deps.Guard),
		UpdateUseCase:  useCases.NewUpdateApiKeyByIDUseCase(repository, deps.Guard, deps.Tx),
		UpgradeUseCase: useCases.NewUpgradeApiKeyByIDUseCase(repository, deps.Guard, deps.Tx),
		DeleteUseCase:  useCases.NewDeleteApiKeyByIDUseCase(repository, deps.Guard, deps.Tx),
		RotateUseCase:  useCases.NewRotateApiKeyByIDUseCase(repository, deps.Members, deps.Guard, deps.Tx, publicURL),
	})
}
