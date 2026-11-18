package member

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/member/grpc_controllers"
	"github.com/murkio-dev/vault/internal/restful/member/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/member/repositories"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
	"google.golang.org/grpc"
)

func Init(db *sql.DB, deps contracts.Dependencies, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:        mux,
		ListUseCase:   useCases.NewListMembersUseCase(repository, deps.Guard),
		GetMeUseCase:  useCases.NewGetMeUseCase(deps.Guard),
		UpdateUseCase: useCases.NewUpdateMemberUseCase(repository, deps.Guard, deps.Tx),
		RemoveUseCase: useCases.NewRemoveMemberUseCase(repository, deps.Guard, deps.Tx),
	})
}

func InitGRPC(db *sql.DB, deps contracts.Dependencies, registrar grpc.ServiceRegistrar) error {
	repository := repositories.NewRepository(db)

	return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{
		Registrar:     registrar,
		ListUseCase:   useCases.NewListMembersUseCase(repository, deps.Guard),
		GetMeUseCase:  useCases.NewGetMeUseCase(deps.Guard),
		UpdateUseCase: useCases.NewUpdateMemberUseCase(repository, deps.Guard, deps.Tx),
		RemoveUseCase: useCases.NewRemoveMemberUseCase(repository, deps.Guard, deps.Tx),
	})
}
