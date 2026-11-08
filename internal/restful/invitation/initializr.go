package invitation

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/invitation/controllers"
	"github.com/murkio-dev/vault/internal/restful/invitation/repositories"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
)

func Init(db *sql.DB, shared contracts.Shared, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return controllers.RegisterControllers(controllers.RegisterInput{
		Router:        mux,
		CreateUseCase: useCases.NewCreateInvitationUseCase(repository, shared.Outbox, shared.Guard, shared.Tx),
		ListUseCase:   useCases.NewListInvitationsUseCase(repository, shared.Guard),
		RevokeUseCase: useCases.NewRevokeInvitationUseCase(repository, shared.Guard, shared.Tx),
		AcceptUseCase: useCases.NewAcceptInvitationUseCase(repository, repository, shared.MemberCreator, shared.MemberUserFinder, shared.Guard, shared.Tx, shared.AuditCreator),
	})
}
