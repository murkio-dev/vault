package controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/invitation/services"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	"github.com/murkio-dev/vault/pkg/utils"
)

type RegisterInput struct {
	Router        *http.ServeMux
	CreateUseCase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	RevokeUseCase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]
	AcceptUseCase contracts.IUseCase[useCases.AcceptInput, useCases.AcceptOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := utils.AssertNotNilParams(map[string]any{
		"input.Router":        input.Router,
		"input.CreateUseCase": input.CreateUseCase,
		"input.ListUseCase":   input.ListUseCase,
		"input.RevokeUseCase": input.RevokeUseCase,
		"input.AcceptUseCase": input.AcceptUseCase,
	})
	if err != nil {
		return err
	}

	utils.Handle(input.Router, "POST /api/v1/applications/{app}/invitations", create(input.CreateUseCase))
	utils.Handle(input.Router, "GET /api/v1/applications/{app}/invitations", list(input.ListUseCase))
	utils.Handle(input.Router, "DELETE /api/v1/applications/{app}/invitations/{id}", revoke(input.RevokeUseCase))
	utils.Handle(input.Router, "POST /api/v1/invitations/accept", accept(input.AcceptUseCase))

	return nil
}

func create(usecase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, email, role, e := services.ParseCreate(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.CreateInput{Requester: entities.ExtractApiHeader(r), Application: application, Email: email, Role: role})

		// A resposta leva o token do convite: só JSON, nunca yaml/xml/tabela.
		utils.RespondJSON(w, r, output, http.StatusCreated)
	}
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, e := services.ParseApplication(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{Requester: entities.ExtractApiHeader(r), Application: application})

		utils.RespondList(w, r, output, http.StatusOK)
	}
}

func revoke(usecase contracts.IUseCase[useCases.RevokeInput, useCases.RevokeOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, invitation, e := services.ParseRevoke(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.RevokeInput{Requester: entities.ExtractApiHeader(r), Application: application, Invitation: invitation})

		utils.Respond(w, r, output, http.StatusNoContent)
	}
}

func accept(usecase contracts.IUseCase[useCases.AcceptInput, useCases.AcceptOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, e := services.ParseAccept(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.AcceptInput{Requester: entities.ExtractApiHeader(r), Token: token})

		utils.RespondJSON(w, r, output, http.StatusCreated)
	}
}
