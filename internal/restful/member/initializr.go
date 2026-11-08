package member

import (
	"database/sql"
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/restful/member/controllers"
	"github.com/murkio-dev/vault/internal/restful/member/repositories"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
)

func Init(db *sql.DB, shared contracts.Shared, mux *http.ServeMux) error {
	repository := repositories.NewRepository(db)

	return controllers.RegisterControllers(controllers.RegisterInput{
		Router:        mux,
		ListUseCase:   useCases.NewListMembersUseCase(repository, shared.Guard),
		GetMeUseCase:  useCases.NewGetMeUseCase(shared.Guard),
		UpdateUseCase: useCases.NewUpdateMemberUseCase(repository, repository, repository, shared.Guard, shared.Tx),
		RemoveUseCase: useCases.NewRemoveMemberUseCase(repository, repository, repository, shared.Guard, shared.Tx),
	})
}
