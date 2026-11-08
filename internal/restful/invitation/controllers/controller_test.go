package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/invitation/controllers"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando cria um convite.", func(t *testing.T) { whenCreateIsValid(t) })
	t.Run("Quando o corpo da criação é inválido.", func(t *testing.T) { whenCreateBodyIsInvalid(t) })
	t.Run("Quando a resposta leva o token.", func(t *testing.T) { whenResponseCarriesTheToken(t) })
	t.Run("Quando lista com e sem barra final.", func(t *testing.T) { whenListAcceptsTrailingSlash(t) })
	t.Run("Quando revoga.", func(t *testing.T) { whenRevokeIsValid(t) })
	t.Run("Quando aceita o convite.", func(t *testing.T) { whenAcceptIsValid(t) })
	t.Run("Quando o corpo do aceite é inválido.", func(t *testing.T) { whenAcceptBodyIsInvalid(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
}

// Functions

type fakes struct {
	create *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list   *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	revoke *pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]
	accept *pkgmocks.UseCase[useCases.AcceptInput, useCases.AcceptOutput]
}

func newServer(t *testing.T) (*http.ServeMux, *fakes) {
	t.Helper()
	f := &fakes{
		create: &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:   &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		revoke: &pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]{},
		accept: &pkgmocks.UseCase[useCases.AcceptInput, useCases.AcceptOutput]{},
	}
	mux := http.NewServeMux()
	require.NoError(t, controllers.RegisterControllers(controllers.RegisterInput{
		Router: mux, CreateUseCase: f.create, ListUseCase: f.list, RevokeUseCase: f.revoke, AcceptUseCase: f.accept,
	}))
	return mux, f
}

func serve(mux *http.ServeMux, method, path, body, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func whenRegisterIsSuccessful(t *testing.T) {
	mux, _ := newServer(t)

	assert.NotNil(t, mux)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	assert.Error(t, controllers.RegisterControllers(controllers.RegisterInput{Router: http.NewServeMux()}))
}

func whenCreateIsValid(t *testing.T) {
	mux, f := newServer(t)
	app := uuid.NewV7()

	rec := serve(mux, http.MethodPost, "/api/v1/applications/"+app.String()+"/invitations", `{"email":"joao@example.com","role":"developer"}`, "")

	assert.Equal(t, http.StatusCreated, rec.Code)
	if assert.Len(t, f.create.Inputs, 1) {
		assert.Equal(t, app, f.create.Inputs[0].Application)
		assert.Equal(t, "joao@example.com", f.create.Inputs[0].Email)
		assert.Equal(t, entities.RoleDeveloper, f.create.Inputs[0].Role)
	}
}

func whenCreateBodyIsInvalid(t *testing.T) {
	mux, f := newServer(t)
	path := "/api/v1/applications/" + uuid.NewV7().String() + "/invitations"

	for name, body := range map[string]string{
		"json quebrado":      `{"email":`,
		"campo desconhecido": `{"email":"a@example.com","role":"developer","created_by":"x"}`,
		"lista de dois":      `[{"email":"a@example.com","role":"developer"},{"email":"b@example.com","role":"developer"}]`,
		"yaml":               "email: a@example.com\n",
		"vazio":              ``,
	} {
		assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPost, path, body, "").Code, name)
	}
	assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPost, "/api/v1/applications/xyz/invitations", `{}`, "").Code)
	assert.Empty(t, f.create.Inputs)
}

func whenResponseCarriesTheToken(t *testing.T) {
	mux, f := newServer(t)
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: entities.InvitationCreated{Token: "inv_segredo-do-convite"}}
	}
	f.accept.PerformFunc = func(context.Context, useCases.AcceptInput) useCases.AcceptOutput { return useCases.AcceptOutput{} }

	for _, accept := range []string{"application/x-yaml", "application/xml", "text/plain"} {
		rec := serve(mux, http.MethodPost, "/api/v1/applications/"+uuid.NewV7().String()+"/invitations", `{"email":"a@example.com","role":"developer"}`, accept)

		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", accept)
		assert.Contains(t, rec.Body.String(), "inv_segredo-do-convite", accept)
	}
	accepted := serve(mux, http.MethodPost, "/api/v1/invitations/accept", `{"token":"inv_x"}`, "text/plain")
	assert.Contains(t, accepted.Header().Get("Content-Type"), "application/json")
}

func whenListAcceptsTrailingSlash(t *testing.T) {
	mux, f := newServer(t)
	path := "/api/v1/applications/" + uuid.NewV7().String() + "/invitations"

	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, path, "", "").Code)
	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, path+"/", "", "").Code)
	assert.Len(t, f.list.Inputs, 2)
}

func whenRevokeIsValid(t *testing.T) {
	mux, f := newServer(t)
	app, id := uuid.NewV7(), uuid.NewV7()

	rec := serve(mux, http.MethodDelete, "/api/v1/applications/"+app.String()+"/invitations/"+id.String(), "", "")
	bad := serve(mux, http.MethodDelete, "/api/v1/applications/"+app.String()+"/invitations/xyz", "", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	if assert.Len(t, f.revoke.Inputs, 1) {
		assert.Equal(t, id, f.revoke.Inputs[0].Invitation)
	}
}

func whenAcceptIsValid(t *testing.T) {
	mux, f := newServer(t)

	rec := serve(mux, http.MethodPost, "/api/v1/invitations/accept", `{"token":"inv_abc"}`, "")
	slash := serve(mux, http.MethodPost, "/api/v1/invitations/accept/", `{"token":"inv_abc"}`, "")

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, http.StatusCreated, slash.Code)
	if assert.Len(t, f.accept.Inputs, 2) {
		assert.Equal(t, "inv_abc", f.accept.Inputs[0].Token)
	}
}

func whenAcceptBodyIsInvalid(t *testing.T) {
	mux, f := newServer(t)

	for name, body := range map[string]string{"quebrado": `{"token":`, "extra": `{"token":"inv_a","role":"admin"}`, "vazio": ``, "lista de dois": `[{"token":"inv_a"},{"token":"inv_b"}]`} {
		assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPost, "/api/v1/invitations/accept", body, "").Code, name)
	}
	assert.Empty(t, f.accept.Inputs)
}

func whenUseCaseReturnsErrors(t *testing.T) {
	mux, f := newServer(t)
	notFound, forbidden := entities.NotFoundError("Resource"), entities.ForbiddenError()
	f.accept.PerformFunc = func(context.Context, useCases.AcceptInput) useCases.AcceptOutput {
		return useCases.AcceptOutput{Error: &notFound}
	}
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Error: &forbidden}
	}

	accept := serve(mux, http.MethodPost, "/api/v1/invitations/accept", `{"token":"inv_x"}`, "")
	create := serve(mux, http.MethodPost, "/api/v1/applications/"+uuid.NewV7().String()+"/invitations", `{"email":"a@example.com","role":"developer"}`, "")

	assert.Equal(t, http.StatusNotFound, accept.Code)
	assert.Equal(t, http.StatusForbidden, create.Code)
}
