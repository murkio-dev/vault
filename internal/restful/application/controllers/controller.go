package controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/services"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	"github.com/murkio-dev/vault/pkg/utils"
)

type RegisterInput struct {
	Router         *http.ServeMux
	CreateUseCase  contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	GetByIDUseCase contracts.IUseCase[useCases.GetByIDInput, useCases.GetByIDOutput]
	ListUseCase    contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	UpdateUseCase  contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	UpgradeUseCase contracts.IUseCase[useCases.UpgradeInput, useCases.UpgradeOutput]
	DeleteUseCase  contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := utils.AssertNotNilParams(map[string]any{
		"input.Router":         input.Router,
		"input.CreateUseCase":  input.CreateUseCase,
		"input.GetByID":        input.GetByIDUseCase,
		"input.List":           input.ListUseCase,
		"input.UpdateUseCase":  input.UpdateUseCase,
		"input.UpgradeUseCase": input.UpgradeUseCase,
		"input.DeleteUseCase":  input.DeleteUseCase,
	})
	if err != nil {
		return err
	}

	input.Router.HandleFunc("POST /api/v1/applications", create(input.CreateUseCase))
	input.Router.HandleFunc("POST /api/v1/applications/{$}", create(input.CreateUseCase))
	input.Router.HandleFunc("GET /api/v1/applications", list(input.ListUseCase))
	input.Router.HandleFunc("GET /api/v1/applications/{$}", list(input.ListUseCase))
	input.Router.HandleFunc("GET /api/v1/applications/{id}", getByID(input.GetByIDUseCase))
	input.Router.HandleFunc("PUT /api/v1/applications", update(input.UpdateUseCase))
	input.Router.HandleFunc("PUT /api/v1/applications/{$}", update(input.UpdateUseCase))
	input.Router.HandleFunc("PUT /api/v1/applications/{id}", update(input.UpdateUseCase))
	input.Router.HandleFunc("PATCH /api/v1/applications", upgrade(input.UpgradeUseCase))
	input.Router.HandleFunc("PATCH /api/v1/applications/{$}", upgrade(input.UpgradeUseCase))
	input.Router.HandleFunc("PATCH /api/v1/applications/{id}", upgrade(input.UpgradeUseCase))
	input.Router.HandleFunc("DELETE /api/v1/applications", deleteApp(input.DeleteUseCase))
	input.Router.HandleFunc("DELETE /api/v1/applications/{$}", deleteApp(input.DeleteUseCase))
	input.Router.HandleFunc("DELETE /api/v1/applications/{id}", deleteApp(input.DeleteUseCase))

	return nil
}

func create(useCase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		applications, e := services.ParseApplications(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := useCase.Perform(r.Context(), useCases.CreateInput{
			Requester:   entities.ExtractApiHeader(r),
			Application: applications,
		})

		utils.Respond(w, r, output, http.StatusCreated)
	}
}

func getByID(usecase contracts.IUseCase[useCases.GetByIDInput, useCases.GetByIDOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, e := services.ParsePathID(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.GetByIDInput{
			Requester:   entities.ExtractApiHeader(r),
			Application: id,
		})

		utils.Respond(w, r, output, http.StatusOK)
	}
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, limit, e := services.ParsePagination(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{
			Requester: entities.ExtractApiHeader(r),
			Page:      page,
			Limit:     limit,
		})

		utils.RespondList(w, r, output, http.StatusOK)
	}
}

func update(usecase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		applications, e := services.ParseApplicationsForChange(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.UpdateInput{
			Requester:     entities.ExtractApiHeader(r),
			Applications:  applications,
			DuplicatedIDs: services.DuplicatedIDs(services.ApplicationIDs(applications)),
		})

		utils.Respond(w, r, output, http.StatusOK)
	}
}

func upgrade(usecase contracts.IUseCase[useCases.UpgradeInput, useCases.UpgradeOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		applications, e := services.ParseApplicationsForChange(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.UpgradeInput{
			Requester:     entities.ExtractApiHeader(r),
			Applications:  applications,
			DuplicatedIDs: services.DuplicatedIDs(services.ApplicationIDs(applications)),
		})

		utils.Respond(w, r, output, http.StatusOK)
	}
}

func deleteApp(usecase contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, e := services.ParseIDs(r)
		if e != nil {
			utils.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.DeleteInput{
			Requester:     entities.ExtractApiHeader(r),
			IDs:           ids,
			DuplicatedIDs: services.DuplicatedIDs(ids),
		})

		utils.Respond(w, r, output, http.StatusNoContent)
	}
}
