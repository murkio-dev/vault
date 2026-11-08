package secret

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/secret/controllers"
	"github.com/murkio-dev/vault/internal/restful/secret/repositories"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
)

func Init(db *sql.DB, shared contracts.Shared, mux *http.ServeMux) error {
	secrets := repositories.NewRepository(db)
	versions := repositories.NewVersionRepository(db)

	return controllers.RegisterControllers(controllers.RegisterInput{
		Router:          mux,
		CreateUseCase:   useCases.NewCreateSecretUseCase(secrets, secrets, secrets, shared.Values, shared.Guard, shared.Tx),
		ListUseCase:     useCases.NewListSecretsUseCase(secrets, shared.Guard),
		GetUseCase:      useCases.NewGetSecretUseCase(secrets, shared.Guard),
		UpdateUseCase:   useCases.NewUpdateSecretUseCase(secrets, secrets, shared.Values, shared.Guard, shared.Tx),
		RenameUseCase:   useCases.NewRenameSecretUseCase(secrets, secrets, shared.Guard, shared.Tx),
		DeleteUseCase:   useCases.NewDeleteSecretUseCase(secrets, secrets, secrets, shared.Guard, shared.Tx),
		VersionsUseCase: useCases.NewListVersionsUseCase(secrets, versions, shared.Guard),
		RevealUseCase:   useCases.NewRevealSecretUseCase(secrets, shared.Values, shared.Guard, shared.Tx),
	})
}
