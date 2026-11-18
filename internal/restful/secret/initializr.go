package secret

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/secret/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/secret/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/secret/repositories"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"google.golang.org/grpc"
)

func Init(db *sql.DB, deps contracts.Dependencies, mux *http.ServeMux) error {
	secrets := repositories.NewRepository(db)
	versions := repositories.NewVersionRepository(db)

	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:          mux,
		CreateUseCase:   useCases.NewCreateSecretUseCase(secrets, deps.Values, deps.Guard, deps.Tx),
		ListUseCase:     useCases.NewListSecretsUseCase(secrets, deps.Guard),
		GetUseCase:      useCases.NewGetSecretUseCase(secrets, deps.Guard),
		UpdateUseCase:   useCases.NewUpdateSecretUseCase(secrets, deps.Values, deps.Guard, deps.Tx),
		RenameUseCase:   useCases.NewRenameSecretUseCase(secrets, deps.Guard, deps.Tx),
		DeleteUseCase:   useCases.NewDeleteSecretUseCase(secrets, deps.Guard, deps.Tx),
		VersionsUseCase: useCases.NewListVersionsUseCase(secrets, versions, deps.Guard),
		RevealUseCase:   useCases.NewRevealSecretUseCase(secrets, deps.Values, deps.Guard, deps.Tx),
	})
}

func InitGRPC(db *sql.DB, deps contracts.Dependencies, registrar grpc.ServiceRegistrar) error {
	secrets := repositories.NewRepository(db)
	versions := repositories.NewVersionRepository(db)

	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:       registrar,
		CreateUseCase:   useCases.NewCreateSecretUseCase(secrets, deps.Values, deps.Guard, deps.Tx),
		ListUseCase:     useCases.NewListSecretsUseCase(secrets, deps.Guard),
		GetUseCase:      useCases.NewGetSecretUseCase(secrets, deps.Guard),
		UpdateUseCase:   useCases.NewUpdateSecretUseCase(secrets, deps.Values, deps.Guard, deps.Tx),
		RenameUseCase:   useCases.NewRenameSecretUseCase(secrets, deps.Guard, deps.Tx),
		DeleteUseCase:   useCases.NewDeleteSecretUseCase(secrets, deps.Guard, deps.Tx),
		VersionsUseCase: useCases.NewListVersionsUseCase(secrets, versions, deps.Guard),
		RevealUseCase:   useCases.NewRevealSecretUseCase(secrets, deps.Values, deps.Guard, deps.Tx),
	})
}
