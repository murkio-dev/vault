package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/controllers"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando a lista é pedida com e sem barra final.", func(t *testing.T) { whenListAcceptsTrailingSlash(t) })
	t.Run("Quando o id do caminho é inválido.", func(t *testing.T) { whenPathIDIsInvalid(t) })
	t.Run("Quando o corpo da atualização é inválido.", func(t *testing.T) { whenUpdateBodyIsInvalid(t) })
	t.Run("Quando a atualização é válida.", func(t *testing.T) { whenUpdateIsValid(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
	t.Run("Quando a remoção dá certo.", func(t *testing.T) { whenRemoveSucceeds(t) })
	t.Run("Quando o ator chega pelo contexto.", func(t *testing.T) { whenActorComesFromContext(t) })
	t.Run("Quando há requisições ao mesmo tempo.", func(t *testing.T) { whenRequestsAreConcurrent(t) })
}

// Functions

type fakes struct {
	list   *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	getMe  *pkgmocks.UseCase[useCases.GetMeInput, useCases.GetMeOutput]
	update *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	remove *pkgmocks.UseCase[useCases.RemoveInput, useCases.RemoveOutput]
}

func newServer(t *testing.T) (*http.ServeMux, *fakes) {
	t.Helper()
	f := &fakes{
		list:   &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		getMe:  &pkgmocks.UseCase[useCases.GetMeInput, useCases.GetMeOutput]{},
		update: &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		remove: &pkgmocks.UseCase[useCases.RemoveInput, useCases.RemoveOutput]{},
	}
	mux := http.NewServeMux()
	require.NoError(t, controllers.RegisterControllers(controllers.RegisterInput{
		Router: mux, ListUseCase: f.list, GetMeUseCase: f.getMe, UpdateUseCase: f.update, RemoveUseCase: f.remove,
	}))
	return mux, f
}

func serve(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func whenRegisterIsSuccessful(t *testing.T) {
	mux, _ := newServer(t)

	assert.NotNil(t, mux)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	err := controllers.RegisterControllers(controllers.RegisterInput{Router: http.NewServeMux()})

	assert.Error(t, err)
}

func whenListAcceptsTrailingSlash(t *testing.T) {
	mux, f := newServer(t)
	app := uuid.NewV7().String()

	without := serve(mux, http.MethodGet, "/api/v1/applications/"+app+"/members", "")
	with := serve(mux, http.MethodGet, "/api/v1/applications/"+app+"/members/", "")

	assert.Equal(t, http.StatusOK, without.Code)
	assert.Equal(t, http.StatusOK, with.Code, "a barra final cai no mesmo handler")
	assert.Len(t, f.list.Inputs, 2)
	assert.Equal(t, 1, f.list.Inputs[0].Page)
	assert.Equal(t, entities.MaxPageSize, f.list.Inputs[0].Limit)

	paged := serve(mux, http.MethodGet, "/api/v1/applications/"+app+"/members?page=3&limit=20", "")
	assert.Equal(t, http.StatusOK, paged.Code)
	assert.Equal(t, 3, f.list.Inputs[2].Page)
	assert.Equal(t, 20, f.list.Inputs[2].Limit)

	for _, query := range []string{"?page=0", "?page=x", "?limit=-1", "?limit=abc"} {
		assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodGet, "/api/v1/applications/"+app+"/members"+query, "").Code, query)
	}
}

func whenPathIDIsInvalid(t *testing.T) {
	mux, f := newServer(t)
	valid := uuid.NewV7().String()

	for _, path := range []string{"/api/v1/applications/nao-e-uuid/members", "/api/v1/applications/00000000-0000-0000-0000-000000000000/members", "/api/v1/applications/" + valid + "/members/xyz"} {
		method := http.MethodGet
		if strings.Contains(path, "/members/xyz") {
			method = http.MethodDelete
		}
		rec := serve(mux, method, path, "")

		assert.Equal(t, http.StatusBadRequest, rec.Code, path)
		assert.NotContains(t, rec.Body.String(), `"value"`, "o erro aponta o campo, sem repetir o valor recebido")
	}
	assert.Empty(t, f.list.Inputs)
	assert.Empty(t, f.remove.Inputs)
}

func whenUpdateBodyIsInvalid(t *testing.T) {
	mux, f := newServer(t)
	path := "/api/v1/applications/" + uuid.NewV7().String() + "/members/" + uuid.NewV7().String()

	for name, body := range map[string]string{
		"json quebrado":      `{"role":`,
		"campo desconhecido": `{"role":"admin","user_id":"` + uuid.NewV7().String() + `"}`,
		"lista de dois":      `[{"role":"admin"},{"role":"developer"}]`,
		"yaml":               "role: admin\n",
		"vazio":              ``,
	} {
		rec := serve(mux, http.MethodPatch, path, body)

		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
	assert.Empty(t, f.update.Inputs, "corpo inválido nunca chega ao usecase")
}

func whenUpdateIsValid(t *testing.T) {
	mux, f := newServer(t)
	app, member := uuid.NewV7(), uuid.NewV7()

	rec := serve(mux, http.MethodPatch, "/api/v1/applications/"+app.String()+"/members/"+member.String(), `{"role":"admin","status":"suspended"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	if assert.Len(t, f.update.Inputs, 1) {
		input := f.update.Inputs[0]
		assert.Equal(t, app, input.Application)
		assert.Equal(t, member, input.Member)
		if assert.NotNil(t, input.Role) && assert.NotNil(t, input.Status) {
			assert.Equal(t, entities.RoleAdmin, *input.Role)
			assert.Equal(t, entities.MemberSuspended, *input.Status)
		}
	}
}

func whenUseCaseReturnsErrors(t *testing.T) {
	mux, f := newServer(t)
	forbidden, conflict := entities.ForbiddenError(), entities.ConflictError("Last admin", "x")
	f.update.PerformFunc = func(_ context.Context, _ useCases.UpdateInput) useCases.UpdateOutput {
		return useCases.UpdateOutput{Error: &conflict}
	}
	f.list.PerformFunc = func(_ context.Context, _ useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &forbidden}
	}
	app := uuid.NewV7().String()

	update := serve(mux, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+uuid.NewV7().String(), `{"role":"developer"}`)
	list := serve(mux, http.MethodGet, "/api/v1/applications/"+app+"/members", "")

	assert.Equal(t, http.StatusConflict, update.Code)
	assert.Equal(t, http.StatusForbidden, list.Code)
	assert.Contains(t, update.Body.String(), entities.ErrTypeConflict)
}

func whenRemoveSucceeds(t *testing.T) {
	mux, f := newServer(t)
	app, member := uuid.NewV7(), uuid.NewV7()

	rec := serve(mux, http.MethodDelete, "/api/v1/applications/"+app.String()+"/members/"+member.String()+"/", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
	if assert.Len(t, f.remove.Inputs, 1) {
		assert.Equal(t, member, f.remove.Inputs[0].Member)
	}
}

func whenActorComesFromContext(t *testing.T) {
	mux, f := newServer(t)
	actor := entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+uuid.NewV7().String()+"/members/me", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-User-ID", uuid.NewV7().String())
	req = req.WithContext(entities.WithActor(req.Context(), actor))

	mux.ServeHTTP(httptest.NewRecorder(), req)

	if assert.Len(t, f.getMe.Inputs, 1) {
		assert.Equal(t, actor, f.getMe.Inputs[0].Requester.Actor, "o ator vem do contexto posto pela autenticação")
		assert.Equal(t, "req-123", f.getMe.Inputs[0].Requester.RequestID)
	}
}

func whenRequestsAreConcurrent(t *testing.T) {
	mux, _ := newServer(t)
	app := uuid.NewV7().String()
	done := make(chan int, 20)

	for i := 0; i < 20; i++ {
		go func() { done <- serve(mux, http.MethodGet, "/api/v1/applications/"+app+"/members", "").Code }()
	}
	for i := 0; i < 20; i++ {
		assert.Equal(t, http.StatusOK, <-done)
	}
}
