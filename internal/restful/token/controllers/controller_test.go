package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/token/controllers"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando o membro do caminho é me ou um id.", func(t *testing.T) { whenMemberPathIsMeOrAnID(t) })
	t.Run("Quando o corpo da criação é inválido.", func(t *testing.T) { whenCreateBodyIsInvalid(t) })
	t.Run("Quando a resposta leva o token.", func(t *testing.T) { whenResponseCarriesTheToken(t) })
	t.Run("Quando lista com e sem barra final.", func(t *testing.T) { whenListAcceptsTrailingSlash(t) })
	t.Run("Quando revoga.", func(t *testing.T) { whenRevokeIsValid(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
}

// Functions

type fakes struct {
	create *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list   *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	revoke *pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]
}

func newServer(t *testing.T) (*http.ServeMux, *fakes) {
	t.Helper()
	f := &fakes{
		create: &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:   &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		revoke: &pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]{},
	}
	mux := http.NewServeMux()
	require.NoError(t, controllers.RegisterControllers(controllers.RegisterInput{Router: mux, CreateUseCase: f.create, ListUseCase: f.list, RevokeUseCase: f.revoke}))
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

func base(member string) string {
	return "/api/v1/applications/" + uuid.NewV7().String() + "/members/" + member + "/tokens"
}

func whenRegisterIsSuccessful(t *testing.T) {
	mux, _ := newServer(t)

	assert.NotNil(t, mux)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	assert.Error(t, controllers.RegisterControllers(controllers.RegisterInput{Router: http.NewServeMux()}))
}

func whenMemberPathIsMeOrAnID(t *testing.T) {
	mux, f := newServer(t)
	member := uuid.NewV7()

	me := serve(mux, http.MethodPost, base("me"), `{"name":"ci"}`, "")
	byID := serve(mux, http.MethodPost, base(member.String()), `{"name":"ci"}`, "")
	bad := serve(mux, http.MethodPost, base("xyz"), `{"name":"ci"}`, "")

	assert.Equal(t, http.StatusCreated, me.Code)
	assert.Equal(t, http.StatusCreated, byID.Code)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	if assert.Len(t, f.create.Inputs, 2) {
		assert.Equal(t, entities.SelfOwner, f.create.Inputs[0].Member, "me vira o marcador que o porteiro resolve")
		assert.Equal(t, member, f.create.Inputs[1].Member)
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
		assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPost, base("me"), body, "").Code, name)
	}
	assert.Empty(t, f.create.Inputs)
}

func whenResponseCarriesTheToken(t *testing.T) {
	mux, f := newServer(t)
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: entities.AccessTokenCreated{Token: "vt_segredo-do-token"}}
	}

	for _, accept := range []string{"application/x-yaml", "application/xml", "text/plain"} {
		rec := serve(mux, http.MethodPost, base("me"), `{"name":"ci"}`, accept)

		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", accept)
		assert.Contains(t, rec.Body.String(), "vt_segredo-do-token", accept)
	}
}

func whenListAcceptsTrailingSlash(t *testing.T) {
	mux, f := newServer(t)
	path := base("me")

	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, path, "", "").Code)
	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, path+"/", "", "").Code)
	assert.Len(t, f.list.Inputs, 2)
}

func whenRevokeIsValid(t *testing.T) {
	mux, f := newServer(t)
	token := uuid.NewV7()

	rec := serve(mux, http.MethodPost, base("me")+"/"+token.String()+"/revoke", "", "")
	bad := serve(mux, http.MethodPost, base("me")+"/xyz/revoke", "", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	if assert.Len(t, f.revoke.Inputs, 1) {
		assert.Equal(t, token, f.revoke.Inputs[0].Token)
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

	assert.Equal(t, http.StatusConflict, serve(mux, http.MethodPost, base("me"), `{"name":"ci"}`, "").Code)
	assert.Equal(t, http.StatusForbidden, serve(mux, http.MethodGet, base("me"), "", "").Code)
}
