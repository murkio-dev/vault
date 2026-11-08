package audit

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/audit/controllers"
	"github.com/murkio-dev/vault/internal/restful/audit/repositories"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
)

func Init(db *sql.DB, shared contracts.Shared, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return controllers.RegisterControllers(controllers.RegisterInput{
		Router:      mux,
		ListUseCase: useCases.NewListAuditLogsUseCase(repository, shared.Guard),
	})
}
