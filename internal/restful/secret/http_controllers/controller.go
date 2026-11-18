package http_controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/services"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	"github.com/murkio-dev/vault/pkg/httpx"
)

type RegisterInput struct {
	Router          *http.ServeMux
	CreateUseCase   contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]
	ListUseCase     contracts.IUseCase[useCases.ListInput, useCases.ListOutput]
	GetUseCase      contracts.IUseCase[useCases.GetInput, useCases.GetOutput]
	UpdateUseCase   contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]
	RenameUseCase   contracts.IUseCase[useCases.RenameInput, useCases.RenameOutput]
	DeleteUseCase   contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]
	VersionsUseCase contracts.IUseCase[useCases.VersionsInput, useCases.VersionsOutput]
	RevealUseCase   contracts.IUseCase[useCases.RevealInput, useCases.RevealOutput]
}

const base = "/api/v1/applications/{app}/members/{member}/secrets"

func RegisterControllers(input RegisterInput) error {
	err := httpx.AssertNotNilParams(map[string]any{
		"input.Router":          input.Router,
		"input.CreateUseCase":   input.CreateUseCase,
		"input.ListUseCase":     input.ListUseCase,
		"input.GetUseCase":      input.GetUseCase,
		"input.UpdateUseCase":   input.UpdateUseCase,
		"input.RenameUseCase":   input.RenameUseCase,
		"input.DeleteUseCase":   input.DeleteUseCase,
		"input.VersionsUseCase": input.VersionsUseCase,
		"input.RevealUseCase":   input.RevealUseCase,
	})
	if err != nil {
		return err
	}

	httpx.Handle(input.Router, "POST "+base, create(input.CreateUseCase))
	httpx.Handle(input.Router, "GET "+base, list(input.ListUseCase))
	httpx.Handle(input.Router, "GET "+base+"/{id}", get(input.GetUseCase))
	httpx.Handle(input.Router, "PUT "+base+"/{id}", update(input.UpdateUseCase))
	httpx.Handle(input.Router, "PATCH "+base+"/{id}", rename(input.RenameUseCase))
	httpx.Handle(input.Router, "DELETE "+base+"/{id}", remove(input.DeleteUseCase))
	httpx.Handle(input.Router, "GET "+base+"/{id}/versions", versions(input.VersionsUseCase))
	httpx.Handle(input.Router, "POST "+base+"/{id}/reveal", reveal(input.RevealUseCase))

	return nil
}

func create(usecase contracts.IUseCase[useCases.CreateInput, useCases.CreateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, items, e := services.ParseCreate(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.CreateInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Items: items})

		httpx.Respond(w, r, output, http.StatusCreated)
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

func get(usecase contracts.IUseCase[useCases.GetInput, useCases.GetOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, secret, e := services.ParseSecretPath(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.GetInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Secret: secret})

		httpx.Respond(w, r, output, http.StatusOK)
	}
}

func update(usecase contracts.IUseCase[useCases.UpdateInput, useCases.UpdateOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, secret, value, e := services.ParseUpdate(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.UpdateInput{
			Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Secret: secret, Value: value,
		})

		httpx.Respond(w, r, output, http.StatusOK)
	}
}

func rename(usecase contracts.IUseCase[useCases.RenameInput, useCases.RenameOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, secret, name, e := services.ParseRename(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.RenameInput{
			Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Secret: secret, Name: name,
		})

		httpx.Respond(w, r, output, http.StatusOK)
	}
}

func remove(usecase contracts.IUseCase[useCases.DeleteInput, useCases.DeleteOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, secret, e := services.ParseSecretPath(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.DeleteInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Secret: secret})

		httpx.Respond(w, r, output, http.StatusNoContent)
	}
}

func versions(usecase contracts.IUseCase[useCases.VersionsInput, useCases.VersionsOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, secret, e := services.ParseSecretPath(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.VersionsInput{Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Secret: secret})

		httpx.RespondList(w, r, output, http.StatusOK)
	}
}

func reveal(usecase contracts.IUseCase[useCases.RevealInput, useCases.RevealOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		application, member, secret, version, e := services.ParseReveal(r)
		if e != nil {
			httpx.RespondError(w, r, *e)
			return
		}

		output := usecase.Perform(r.Context(), useCases.RevealInput{
			Requester: entities.ExtractApiHeader(r), Application: application, Member: member, Secret: secret, Version: version,
		})

		// O valor do segredo: só JSON, nunca yaml/xml/tabela.
		httpx.RespondJSON(w, r, output, http.StatusOK)
	}
}
