package token

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/token/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/token/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/token/repositories"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
	"google.golang.org/grpc"
)

func Init(db *sql.DB, deps contracts.Dependencies, publicURL string, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:        mux,
		CreateUseCase: useCases.NewCreateTokenUseCase(repository, deps.Members, deps.Guard, deps.Tx, publicURL),
		ListUseCase:   useCases.NewListTokensUseCase(repository, deps.Guard),
		RevokeUseCase: useCases.NewRevokeTokenUseCase(repository, deps.Guard, deps.Tx),
	})
}

func InitGRPC(db *sql.DB, deps contracts.Dependencies, publicURL string, registrar grpc.ServiceRegistrar) error {
	repository := repositories.NewRepository(db)

	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:     registrar,
		CreateUseCase: useCases.NewCreateTokenUseCase(repository, deps.Members, deps.Guard, deps.Tx, publicURL),
		ListUseCase:   useCases.NewListTokensUseCase(repository, deps.Guard),
		RevokeUseCase: useCases.NewRevokeTokenUseCase(repository, deps.Guard, deps.Tx),
	})
}
