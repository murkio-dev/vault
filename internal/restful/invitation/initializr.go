package invitation

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/invitation/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/invitation/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/invitation/repositories"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	"google.golang.org/grpc"
)

func Init(db *sql.DB, deps contracts.Dependencies, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)
	outbox := repositories.NewOutboxRepository(db)

	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:        mux,
		CreateUseCase: useCases.NewCreateInvitationUseCase(repository, outbox, deps.Guard, deps.Tx),
		ListUseCase:   useCases.NewListInvitationsUseCase(repository, deps.Guard),
		RevokeUseCase: useCases.NewRevokeInvitationUseCase(repository, deps.Guard, deps.Tx),
		AcceptUseCase: useCases.NewAcceptInvitationUseCase(repository, deps.Members, deps.Guard, deps.Tx, deps.Audit),
	})
}

func InitGRPC(db *sql.DB, deps contracts.Dependencies, registrar grpc.ServiceRegistrar) error {
	repository := repositories.NewRepository(db)
	outbox := repositories.NewOutboxRepository(db)

	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:     registrar,
		CreateUseCase: useCases.NewCreateInvitationUseCase(repository, outbox, deps.Guard, deps.Tx),
		ListUseCase:   useCases.NewListInvitationsUseCase(repository, deps.Guard),
		RevokeUseCase: useCases.NewRevokeInvitationUseCase(repository, deps.Guard, deps.Tx),
		AcceptUseCase: useCases.NewAcceptInvitationUseCase(repository, deps.Members, deps.Guard, deps.Tx, deps.Audit),
	})
}
