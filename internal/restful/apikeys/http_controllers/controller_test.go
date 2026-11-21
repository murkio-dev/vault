package http_controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando o membro da query é me, ausente ou um id.", func(t *testing.T) { whenMemberQueryIsMeOrAnID(t) })
	t.Run("Quando o corpo da criação é inválido.", func(t *testing.T) { whenCreateBodyIsInvalid(t) })
	t.Run("Quando a resposta leva a chave.", func(t *testing.T) { whenResponseCarriesTheKey(t) })
	t.Run("Quando lista com e sem barra final.", func(t *testing.T) { whenListAcceptsTrailingSlash(t) })
	t.Run("Quando usa as rotas por id.", func(t *testing.T) { whenRoutesByIDAreUsed(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
	t.Run("Quando a aplicação falta na query.", func(t *testing.T) { whenApplicationIsMissing(t) })
}

// Functions

type fakes struct {
	create  *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list    *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	get     *pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]
	update  *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	upgrade *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	delete  *pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]
	rotate  *pkgmocks.UseCase[useCases.RotateInput, useCases.RotateOutput]
}

func newServer(t *testing.T) (*http.ServeMux, *fakes) {
	t.Helper()
	f := &fakes{
		create:  &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		get:     &pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]{},
		update:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		upgrade: &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		delete:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
		rotate:  &pkgmocks.UseCase[useCases.RotateInput, useCases.RotateOutput]{},
	}
	mux := http.NewServeMux()
	require.NoError(t, http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router: mux, CreateUseCase: f.create, ListUseCase: f.list, GetUseCase: f.get, UpdateUseCase: f.update,
		UpgradeUseCase: f.upgrade, DeleteUseCase: f.delete, RotateUseCase: f.rotate,
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

// route monta a rota com o dono na query; sub é o que vem depois de
// /api/v1/api-keys (ex.: "/<id>/rotate").
func route(sub, member string) string {
	query := "?application_id=" + uuid.NewV7().String()
	if member != "" {
		query += "&member_id=" + member
	}
	return "/api/v1/api-keys" + sub + query
}

func whenRegisterIsSuccessful(t *testing.T) {
	mux, _ := newServer(t)

	assert.NotNil(t, mux)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	assert.Error(t, http_controllers.RegisterControllers(http_controllers.RegisterInput{Router: http.NewServeMux()}))
}

func whenMemberQueryIsMeOrAnID(t *testing.T) {
	mux, f := newServer(t)
	member := uuid.NewV7()
	body := `{"name":"ci","type":"secret"}`

	me := serve(mux, http.MethodPost, route("", "me"), body, "")
	absent := serve(mux, http.MethodPost, route("", ""), body, "")
	byID := serve(mux, http.MethodPost, route("", member.String()), body, "")
	bad := serve(mux, http.MethodPost, route("", "xyz"), body, "")

	assert.Equal(t, http.StatusCreated, me.Code)
	assert.Equal(t, http.StatusCreated, absent.Code)
	assert.Equal(t, http.StatusCreated, byID.Code)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	if assert.Len(t, f.create.Inputs, 3) {
		assert.Equal(t, entities.SelfOwner, f.create.Inputs[0].Member, "me vira o marcador que o porteiro resolve")
		assert.Equal(t, entities.SelfOwner, f.create.Inputs[1].Member, "sem member_id, vale o membro de quem chama")
		assert.Equal(t, member, f.create.Inputs[2].Member)
		assert.Equal(t, entities.KeySecret, f.create.Inputs[0].Type)
	}
}

func whenCreateBodyIsInvalid(t *testing.T) {
	mux, f := newServer(t)

	for name, body := range map[string]string{
		"json quebrado":      `{"name":`,
		"campo desconhecido": `{"name":"ci","member_id":"` + uuid.NewV7().String() + `"}`,
		"data inválida":      `{"name":"ci","expires_at":"amanhã"}`,
		"lista de dois":      `[{"name":"ci"},{"name":"outro"}]`,
		"yaml":               "name: ci\n",
		"vazio":              ``,
	} {
		assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPost, route("", "me"), body, "").Code, name)
	}
	assert.Empty(t, f.create.Inputs)
}

func whenResponseCarriesTheKey(t *testing.T) {
	mux, f := newServer(t)
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: entities.ApiKeyCreated{Key: "sk_vt_segredo-da-chave"}}
	}
	f.rotate.PerformFunc = func(context.Context, useCases.RotateInput) useCases.RotateOutput {
		return useCases.RotateOutput{Data: entities.ApiKeyCreated{Key: "sk_vt_segredo-da-chave"}}
	}

	for _, accept := range []string{"application/x-yaml", "application/xml", "text/plain"} {
		created := serve(mux, http.MethodPost, route("", "me"), `{"name":"ci","type":"secret"}`, accept)
		rotated := serve(mux, http.MethodPost, route("/"+uuid.NewV7().String()+"/rotate", "me"), "", accept)

		for name, rec := range map[string]*httptest.ResponseRecorder{"create": created, "rotate": rotated} {
			assert.Equal(t, http.StatusCreated, rec.Code, name)
			assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", name+" "+accept)
			assert.Contains(t, rec.Body.String(), "sk_vt_segredo-da-chave", name+" "+accept)
		}
	}
}

func whenListAcceptsTrailingSlash(t *testing.T) {
	mux, f := newServer(t)
	query := "?application_id=" + uuid.NewV7().String()

	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, "/api/v1/api-keys"+query, "", "").Code)
	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, "/api/v1/api-keys/"+query, "", "").Code)
	assert.Len(t, f.list.Inputs, 2)
}

func whenRoutesByIDAreUsed(t *testing.T) {
	mux, f := newServer(t)
	id := uuid.NewV7()
	sub := "/" + id.String()

	get := serve(mux, http.MethodGet, route(sub, "me"), "", "")
	patch := serve(mux, http.MethodPatch, route(sub, "me"), `{"name":"deploy"}`, "")
	put := serve(mux, http.MethodPut, route(sub, "me"), `{"name":"deploy"}`, "")
	del := serve(mux, http.MethodDelete, route(sub, "me"), "", "")
	bad := serve(mux, http.MethodDelete, route("/xyz", "me"), "", "")

	assert.Equal(t, http.StatusOK, get.Code)
	assert.Equal(t, http.StatusOK, patch.Code)
	assert.Equal(t, http.StatusOK, put.Code)
	assert.Equal(t, http.StatusNoContent, del.Code)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	if assert.Len(t, f.get.Inputs, 1) && assert.Len(t, f.update.Inputs, 1) && assert.Len(t, f.upgrade.Inputs, 1) && assert.Len(t, f.delete.Inputs, 1) {
		assert.Equal(t, id, f.get.Inputs[0].ID)
		assert.Equal(t, "deploy", f.update.Inputs[0].Name)
		assert.Equal(t, "deploy", f.upgrade.Inputs[0].Name, "PUT vai para o use case de upgrade")
		assert.Equal(t, id, f.delete.Inputs[0].ID)
	}
}

func whenUseCaseReturnsErrors(t *testing.T) {
	mux, f := newServer(t)
	forbidden, conflict := entities.ForbiddenError(), entities.ConflictError("Member suspended", "x")
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Error: &conflict}
	}
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &forbidden}
	}

	assert.Equal(t, http.StatusConflict, serve(mux, http.MethodPost, route("", "me"), `{"name":"ci","type":"secret"}`, "").Code)
	assert.Equal(t, http.StatusForbidden, serve(mux, http.MethodGet, route("", "me"), "", "").Code)
}

func whenApplicationIsMissing(t *testing.T) {
	mux, f := newServer(t)

	assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodGet, "/api/v1/api-keys?application_id=nao-e-uuid", "", "").Code)
	assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodGet, "/api/v1/api-keys", "", "").Code)
	assert.Empty(t, f.list.Inputs, "entrada inválida não chega ao use case")
}
