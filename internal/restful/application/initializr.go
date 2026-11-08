package application

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/application/controllers"
	"github.com/murkio-dev/vault/internal/restful/application/repositories"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
)

func Init(db *sql.DB, shared contracts.Shared, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return controllers.RegisterControllers(controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  useCases.NewCreateApplicationUseCase(repository, repository, shared.MemberCreator, shared.Guard, shared.Tx),
		GetByIDUseCase: useCases.NewGetApplicationByIDUseCase(repository, shared.Guard),
		ListUseCase:    useCases.NewListApplicationsUseCase(repository, repository),
		UpdateUseCase:  useCases.NewUpdateApplicationUseCase(repository, shared.Guard, shared.Tx),
		UpgradeUseCase: useCases.NewUpgradeApplicationUseCase(repository, shared.Guard, shared.Tx),
		DeleteUseCase:  useCases.NewDeleteApplicationUseCase(repository, shared.Guard, shared.Tx),
	})
}
