package controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/services"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
	"github.com/murkio-dev/vault/pkg/utils"
)

type RegisterInput struct {
	Router        *http.ServeMux
	ListUseCase   contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	GetMeUseCase  contracts.IUseCase[useCases.GetMeInput, useCases.GetMeOutput]
	UpdateUseCase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	RemoveUseCase contracts.IUseCase[useCases.RemoveInput, useCases.RemoveOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := utils.AssertNotNilParams(map[string]any{
		"input.Router":        input.Router,
		"input.ListUseCase":   input.ListUseCase,
		"input.GetMeUseCase":  input.GetMeUseCase,
		"input.UpdateUseCase": input.UpdateUseCase,
		"input.RemoveUseCase": input.RemoveUseCase,
	})
	if err != nil {
		return err
	}

	utils.Handle(input.Router, "GET /api/v1/applications/{app}/members", list(input.ListUseCase))
	utils.Handle(input.Router, "GET /api/v1/applications/{app}/members/me", getMe(input.GetMeUseCase))
	utils.Handle(input.Router, "PATCH /api/v1/applications/{app}/members/{member}", update(input.UpdateUseCase))
	utils.Handle(input.Router, "DELETE /api/v1/applications/{app}/members/{member}", remove(input.RemoveUseCase))

	return nil
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, page, limit, e := services.ParseList(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{Requester: entities.ExtractApiHeader(r), Application: application, Page: page, Limit: limit})

		utils.RespondList(w, r, output, http.StatusOK)
	}
}

func getMe(usecase contracts.IUseCase[useCases.GetMeInput, useCases.GetMeOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, e := services.ParseApplication(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.GetMeInput{Requester: entities.ExtractApiHeader(r), Application: application})

		utils.Respond(w, r, output, http.StatusOK)
	}
}

func update(usecase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, role, status, e := services.ParseUpdate(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.UpdateInput{
			Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Role: role, Status: status,
		})

		utils.Respond(w, r, output, http.StatusOK)
	}
}

func remove(usecase contracts.IUseCase[useCases.RemoveInput, useCases.RemoveOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, e := services.ParseMember(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.RemoveInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member})

		utils.Respond(w, r, output, http.StatusNoContent)
	}
}
