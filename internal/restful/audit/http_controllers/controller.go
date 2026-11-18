package http_controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/services"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
	"github.com/murkio-dev/vault/pkg/httpx"
)

type RegisterInput struct {
	Router      *http.ServeMux
	ListUseCase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Router":      input.Router,
		"input.ListUseCase": input.ListUseCase,
	})
	if err != nil {
		return err
	}

	httpx.Handle(input.Router, "GET /api/v1/applications/{app}/audit-logs", list(input.ListUseCase))

	return nil
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, filter, e := services.ParseList(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{Requester: entities.ExtractApiHeader(r), Application: application, Filter: filter})

		httpx.RespondList(w, r, output, http.StatusOK)
	}
}
