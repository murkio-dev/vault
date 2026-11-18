package http_controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/token/services"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
	"github.com/murkio-dev/vault/pkg/httpx"
)

type RegisterInput struct {
	Router        *http.ServeMux
	CreateUseCase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	RevokeUseCase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]
}

const base = "/api/v1/applications/{app}/members/{member}/tokens"

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Router":        input.Router,
		"input.CreateUseCase": input.CreateUseCase,
		"input.ListUseCase":   input.ListUseCase,
		"input.RevokeUseCase": input.RevokeUseCase,
	})
	if err != nil {
		return err
	}

	httpx.Handle(input.Router, "POST "+base, create(input.CreateUseCase))
	httpx.Handle(input.Router, "GET "+base, list(input.ListUseCase))
	httpx.Handle(input.Router, "POST "+base+"/{id}/revoke", revoke(input.RevokeUseCase))

	return nil
}

func create(usecase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, name, expiresAt, e := services.ParseCreate(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.CreateInput{
			Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Name: name, ExpiresAt: expiresAt,
		})

		// A resposta leva o token: só JSON.
		httpx.RespondJSON(w, r, output, http.StatusCreated)
	}
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, e := services.ParsePath(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member})

		httpx.RespondList(w, r, output, http.StatusOK)
	}
}

func revoke(usecase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, token, e := services.ParseRevoke(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.RevokeInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Token: token})

		httpx.Respond(w, r, output, http.StatusNoContent)
	}
}
