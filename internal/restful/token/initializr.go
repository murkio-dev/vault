package token

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/token/controllers"
	"github.com/murkio-dev/vault/internal/restful/token/repositories"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
)

func Init(db *sql.DB, shared contracts.Shared, publicURL string, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return controllers.RegisterControllers(controllers.RegisterInput{
		Router:        mux,
		CreateUseCase: useCases.NewCreateTokenUseCase(repository, shared.MemberFinder, shared.Guard, shared.Tx, publicURL),
		ListUseCase:   useCases.NewListTokensUseCase(repository, shared.Guard),
		RevokeUseCase: useCases.NewRevokeTokenUseCase(repository, repository, shared.Guard, shared.Tx),
	})
}
