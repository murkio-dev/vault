package controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/services"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
	"github.com/murkio-dev/vault/pkg/utils"
)

type RegisterInput struct {
	Router      *http.ServeMux
	ListUseCase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := utils.AssertNotNilParams(map[string]any{
		"input.Router":      input.Router,
		"input.ListUseCase": input.ListUseCase,
	})
	if err != nil {
		return err
	}

	utils.Handle(input.Router, "GET /api/v1/applications/{app}/audit-logs", list(input.ListUseCase))

	return nil
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, filter, e := services.ParseList(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{Requester: entities.ExtractApiHeader(r), Application: application, Filter: filter})

		utils.RespondList(w, r, output, http.StatusOK)
	}
}
