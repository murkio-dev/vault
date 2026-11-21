package http_controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/services"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
	"github.com/murkio-dev/vault/pkg/httpx"
)

type RegisterInput struct {
	Router         *http.ServeMux
	CreateUseCase  contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase    contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	GetUseCase     contracts.IUseCase[useCases.GetInput, useCases.GetOutput]
	UpdateUseCase  contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	UpgradeUseCase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	DeleteUseCase  contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
	RotateUseCase  contracts.IUseCase[useCases.RotateInput, useCases.RotateOutput]
}

// base é a mesma rota de chaves do Identity e do Messaging. De quem são as
// chaves vem na query (application_id e member_id), ver services.ParseOwner.
const base = "/api/v1/api-keys"

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Router":         input.Router,
		"input.CreateUseCase":  input.CreateUseCase,
		"input.ListUseCase":    input.ListUseCase,
		"input.GetUseCase":     input.GetUseCase,
		"input.UpdateUseCase":  input.UpdateUseCase,
		"input.UpgradeUseCase": input.UpgradeUseCase,
		"input.DeleteUseCase":  input.DeleteUseCase,
		"input.RotateUseCase":  input.RotateUseCase,
	})
	if err != nil {
		return err
	}

	httpx.Handle(input.Router, "POST "+base, create(input.CreateUseCase))
	httpx.Handle(input.Router, "GET "+base, list(input.ListUseCase))
	httpx.Handle(input.Router, "GET "+base+"/{id}", getByID(input.GetUseCase))
	httpx.Handle(input.Router, "PATCH "+base+"/{id}", updateByID(input.UpdateUseCase))
	httpx.Handle(input.Router, "PUT "+base+"/{id}", updateByID(input.UpgradeUseCase))
	httpx.Handle(input.Router, "DELETE "+base+"/{id}", deleteByID(input.DeleteUseCase))
	httpx.Handle(input.Router, "POST "+base+"/{id}/rotate", rotateByID(input.RotateUseCase))

	return nil
}

func create(usecase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, e := services.ParseCreate(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.CreateInput{
			Requester: entities.ExtractApiHeader(r), Application: data.Application, Member: data.Member, Name: data.Name, Type: data.Type, ExpiresAt: data.ExpiresAt,
		})

		// A resposta leva a chave: só JSON.
		httpx.RespondJSON(w, r, output, http.StatusCreated)
	}
}

func list(usecase contracts.IUseCase[useCases.ListInput, useCases.ListOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, e := services.ParseOwner(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.ListInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member})

		httpx.RespondList(w, r, output, http.StatusOK)
	}
}

func getByID(usecase contracts.IUseCase[useCases.GetInput, useCases.GetOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, id, e := services.ParseByID(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.GetInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, ID: id})

		httpx.Respond(w, r, output, http.StatusOK)
	}
}

func updateByID(usecase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, id, name, e := services.ParseUpdate(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.UpdateInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, ID: id, Name: name})

		httpx.Respond(w, r, output, http.StatusOK)
	}
}

func deleteByID(usecase contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, id, e := services.ParseByID(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.DeleteInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, ID: id})

		httpx.Respond(w, r, output, http.StatusNoContent)
	}
}

func rotateByID(usecase contracts.IUseCase[useCases.RotateInput, useCases.RotateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, id, e := services.ParseByID(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.RotateInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, ID: id})

		// A resposta leva a chave nova: só JSON.
		httpx.RespondJSON(w, r, output, http.StatusCreated)
	}
}
