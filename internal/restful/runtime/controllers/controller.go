package controllers

import (
	"net/http"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/runtime/services"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	"github.com/murkio-dev/vault/pkg/utils"
)

type RegisterInput struct {
	Router     *http.ServeMux
	EnvUseCase contracts.IUseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]
}

func RegisterControllers(input RegisterInput) error {
	err := utils.AssertNotNilParams(map[string]any{
		"input.Router":     input.Router,
		"input.EnvUseCase": input.EnvUseCase,
	})
	if err != nil {
		return err
	}

	utils.Handle(input.Router, "GET /api/v1/runtime/{uri_id}/env", env(input.EnvUseCase))

	return nil
}

func env(usecase contracts.IUseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uriID, names := services.ParseEnv(r)

		output := usecase.Perform(r.Context(), useCases.ReadEnvInput{Requester: entities.ExtractApiHeader(r), UriID: uriID, Names: names})

		// Os valores dos segredos: só JSON, nunca yaml/xml/tabela.
		utils.RespondJSON(w, r, output, http.StatusOK)
	}
}
