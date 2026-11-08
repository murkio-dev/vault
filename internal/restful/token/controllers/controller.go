package controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/token/services"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
	"github.com/murkio-dev/vault/pkg/utils"
)

type RegisterInput struct {
	Router        *http.ServeMux
	CreateUseCase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	RevokeUseCase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]
}

const base = "/api/v1/applications/{app}/members/{member}/tokens"

func RegisterControllers(input RegisterInput) error {
	err := utils.AssertNotNilParams(map[string]any{
		"input.Router":        input.Router,
		"input.CreateUseCase": input.CreateUseCase,
		"input.ListUseCase":   input.ListUseCase,
		"input.RevokeUseCase": input.RevokeUseCase,
	})
	if err != nil {
		return err
	}

	utils.Handle(input.Router, "POST "+base, create(input.CreateUseCase))
	utils.Handle(input.Router, "GET "+base, list(input.ListUseCase))
	utils.Handle(input.Router, "POST "+base+"/{id}/revoke", revoke(input.RevokeUseCase))

	return nil
}

func create(usecase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, name, expiresAt, e := services.ParseCreate(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.CreateInput{
			Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Name: name, ExpiresAt: expiresAt,
		})

		// A resposta leva o token: só JSON.
		utils.RespondJSON(w, r, output, http.StatusCreated)
	}
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, e := services.ParsePath(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member})

		utils.RespondList(w, r, output, http.StatusOK)
	}
}

func revoke(usecase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, token, e := services.ParseRevoke(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.RevokeInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Token: token})

		utils.Respond(w, r, output, http.StatusNoContent)
	}
}
