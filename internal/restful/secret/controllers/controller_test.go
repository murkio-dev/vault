package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/controllers"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando cria um segredo ou um lote.", func(t *testing.T) { whenCreateAcceptsOneOrMany(t) })
	t.Run("Quando o corpo da criação é inválido.", func(t *testing.T) { whenCreateBodyIsInvalid(t) })
	t.Run("Quando o lote da criação é misto.", func(t *testing.T) { whenCreateBatchIsMixed(t) })
	t.Run("Quando o membro do caminho é me ou um id.", func(t *testing.T) { whenMemberPathIsMeOrAnID(t) })
	t.Run("Quando lista com e sem barra final.", func(t *testing.T) { whenListAcceptsTrailingSlash(t) })
	t.Run("Quando troca o valor, renomeia e apaga.", func(t *testing.T) { whenWriteRoutesAreValid(t) })
	t.Run("Quando os corpos de escrita são inválidos.", func(t *testing.T) { whenWriteBodiesAreInvalid(t) })
	t.Run("Quando revela o segredo.", func(t *testing.T) { whenRevealIsValid(t) })
	t.Run("Quando o reveal ignora o Accept.", func(t *testing.T) { whenRevealIgnoresAccept(t) })
	t.Run("Quando o reveal tem corpo inválido.", func(t *testing.T) { whenRevealBodyIsInvalid(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
}

// Functions

type fakes struct {
	create   *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list     *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	get      *pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]
	update   *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	rename   *pkgmocks.UseCase[useCases.RenameInput, useCases.RenameOutput]
	remove   *pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]
	versions *pkgmocks.UseCase[useCases.VersionsInput, useCases.VersionsOutput]
	reveal   *pkgmocks.UseCase[useCases.RevealInput, useCases.RevealOutput]
}

func newServer(t *testing.T) (*http.ServeMux, *fakes) {
	t.Helper()
	f := &fakes{
		create:   &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:     &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		get:      &pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]{},
		update:   &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		rename:   &pkgmocks.UseCase[useCases.RenameInput, useCases.RenameOutput]{},
		remove:   &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
		versions: &pkgmocks.UseCase[useCases.VersionsInput, useCases.VersionsOutput]{},
		reveal:   &pkgmocks.UseCase[useCases.RevealInput, useCases.RevealOutput]{},
	}
	mux := http.NewServeMux()
	require.NoError(t, controllers.RegisterControllers(controllers.RegisterInput{
		Router: mux, CreateUseCase: f.create, ListUseCase: f.list, GetUseCase: f.get, UpdateUseCase: f.update,
		RenameUseCase: f.rename, DeleteUseCase: f.remove, VersionsUseCase: f.versions, RevealUseCase: f.reveal,
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

func base(member string) string {
	return "/api/v1/applications/" + uuid.NewV7().String() + "/members/" + member + "/secrets"
}

func whenRegisterIsSuccessful(t *testing.T) {
	mux, _ := newServer(t)

	assert.NotNil(t, mux)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	assert.Error(t, controllers.RegisterControllers(controllers.RegisterInput{Router: http.NewServeMux()}))
}

func whenCreateAcceptsOneOrMany(t *testing.T) {
	mux, f := newServer(t)

	one := serve(mux, http.MethodPost, base("me"), `{"name":"A","type":"string","value":"v"}`, "")
	many := serve(mux, http.MethodPost, base("me"), `[{"name":"A","type":"string","value":"v"},{"name":"B","type":"url","value":"https://x.io"}]`, "")

	assert.Equal(t, http.StatusCreated, one.Code)
	assert.Equal(t, http.StatusCreated, many.Code)
	if assert.Len(t, f.create.Inputs, 2) {
		assert.Len(t, f.create.Inputs[0].Items, 1)
		assert.Len(t, f.create.Inputs[1].Items, 2)
		assert.Equal(t, entities.SecretURL, f.create.Inputs[1].Items[1].Type)
	}
}

func whenCreateBodyIsInvalid(t *testing.T) {
	mux, f := newServer(t)

	for name, body := range map[string]string{
		"json quebrado":      `{"name":`,
		"campo desconhecido": `{"name":"A","type":"string","value":"v","member_id":"x"}`,
		"dono forjado":       `{"name":"A","type":"string","value":"v","created_by":"x"}`,
		"yaml":               "name: A\ntype: string\nvalue: v\n",
		"xml":                `<secret><name>A</name></secret>`,
		"lista vazia":        `[]`,
		"vazio":              ``,
	} {
		rec := serve(mux, http.MethodPost, base("me"), body, "")

		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
		assert.NotContains(t, rec.Body.String(), `"value":"v"`, name)
	}
	assert.Empty(t, f.create.Inputs, "corpo inválido nunca chega ao usecase")
}

func whenCreateBatchIsMixed(t *testing.T) {
	mux, f := newServer(t)
	invalid := entities.ValidationError("x")
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: []entities.Secret{mocks.NewSecret()}, Errors: []entities.Error{invalid}}
	}

	rec := serve(mux, http.MethodPost, base("me"), `[{"name":"A","type":"string","value":"v"},{"name":"b","type":"string","value":"v"}]`, "")

	assert.Equal(t, http.StatusMultiStatus, rec.Code)
	assert.Contains(t, rec.Body.String(), `"data"`)
	assert.Contains(t, rec.Body.String(), `"errors"`)
}

func whenMemberPathIsMeOrAnID(t *testing.T) {
	mux, f := newServer(t)
	member := uuid.NewV7()

	serve(mux, http.MethodGet, base("me"), "", "")
	serve(mux, http.MethodGet, base(member.String()), "", "")
	bad := serve(mux, http.MethodGet, base("xyz"), "", "")

	assert.Equal(t, http.StatusBadRequest, bad.Code)
	if assert.Len(t, f.list.Inputs, 2) {
		assert.Equal(t, entities.SelfOwner, f.list.Inputs[0].Member)
		assert.Equal(t, member, f.list.Inputs[1].Member)
	}
}

func whenListAcceptsTrailingSlash(t *testing.T) {
	mux, f := newServer(t)
	path := base("me")

	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, path, "", "").Code)
	assert.Equal(t, http.StatusOK, serve(mux, http.MethodGet, path+"/", "", "").Code)
	assert.Len(t, f.list.Inputs, 2)
}

func whenWriteRoutesAreValid(t *testing.T) {
	mux, f := newServer(t)
	id := uuid.NewV7().String()

	update := serve(mux, http.MethodPut, base("me")+"/"+id, `{"value":"novo"}`, "")
	rename := serve(mux, http.MethodPatch, base("me")+"/"+id, `{"name":"NOVO"}`, "")
	remove := serve(mux, http.MethodDelete, base("me")+"/"+id, "", "")
	versions := serve(mux, http.MethodGet, base("me")+"/"+id+"/versions", "", "")
	get := serve(mux, http.MethodGet, base("me")+"/"+id, "", "")

	assert.Equal(t, http.StatusOK, update.Code)
	assert.Equal(t, http.StatusOK, rename.Code)
	assert.Equal(t, http.StatusNoContent, remove.Code)
	assert.Equal(t, http.StatusOK, versions.Code)
	assert.Equal(t, http.StatusOK, get.Code)
	if assert.Len(t, f.update.Inputs, 1) && assert.Len(t, f.rename.Inputs, 1) {
		assert.Equal(t, "novo", f.update.Inputs[0].Value)
		assert.Equal(t, "NOVO", f.rename.Inputs[0].Name)
	}
}

func whenWriteBodiesAreInvalid(t *testing.T) {
	mux, f := newServer(t)
	id := uuid.NewV7().String()

	for name, body := range map[string]string{"quebrado": `{"value":`, "extra": `{"value":"v","version":3}`, "lista de dois": `[{"value":"a"},{"value":"b"}]`, "yaml": "value: v\n", "vazio": ``} {
		assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPut, base("me")+"/"+id, body, "").Code, "PUT "+name)
	}
	assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPatch, base("me")+"/"+id, `{"name":"A","extra":1}`, "").Code)
	assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPut, base("me")+"/nao-uuid", `{"value":"v"}`, "").Code)
	assert.Empty(t, f.update.Inputs)
	assert.Empty(t, f.rename.Inputs)
}

func whenRevealIsValid(t *testing.T) {
	mux, f := newServer(t)
	id := uuid.NewV7()

	withoutBody := serve(mux, http.MethodPost, base("me")+"/"+id.String()+"/reveal", "", "")
	withVersion := serve(mux, http.MethodPost, base("me")+"/"+id.String()+"/reveal", `{"version":2}`, "")

	assert.Equal(t, http.StatusOK, withoutBody.Code)
	assert.Equal(t, http.StatusOK, withVersion.Code)
	if assert.Len(t, f.reveal.Inputs, 2) {
		assert.Zero(t, f.reveal.Inputs[0].Version, "sem corpo, a versão é a atual")
		assert.Equal(t, 2, f.reveal.Inputs[1].Version)
		assert.Equal(t, id, f.reveal.Inputs[1].Secret)
	}
}

func whenRevealIgnoresAccept(t *testing.T) {
	mux, f := newServer(t)
	f.reveal.PerformFunc = func(context.Context, useCases.RevealInput) useCases.RevealOutput {
		return useCases.RevealOutput{Data: entities.RevealedSecret{Name: "A", Version: 1, Value: "valor-secreto"}}
	}

	for _, accept := range []string{"application/x-yaml", "application/xml", "text/plain"} {
		rec := serve(mux, http.MethodPost, base("me")+"/"+uuid.NewV7().String()+"/reveal", "", accept)

		assert.Equal(t, http.StatusOK, rec.Code, accept)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", accept)
		assert.Contains(t, rec.Body.String(), `"value":"valor-secreto"`, accept)
	}
}

func whenRevealBodyIsInvalid(t *testing.T) {
	mux, f := newServer(t)
	path := base("me") + "/" + uuid.NewV7().String() + "/reveal"

	for name, body := range map[string]string{"quebrado": `{"version":`, "extra": `{"version":1,"x":1}`, "texto": `{"version":"um"}`, "yaml": "version: 1\n"} {
		assert.Equal(t, http.StatusBadRequest, serve(mux, http.MethodPost, path, body, "").Code, name)
	}
	assert.Empty(t, f.reveal.Inputs)
}

func whenUseCaseReturnsErrors(t *testing.T) {
	mux, f := newServer(t)
	forbidden, notFound, conflict := entities.ForbiddenError(), entities.NotFoundError("Resource"), entities.ConflictError("Secret is referenced", "x")
	f.get.PerformFunc = func(context.Context, useCases.GetInput) useCases.GetOutput {
		return useCases.GetOutput{Error: &forbidden}
	}
	f.reveal.PerformFunc = func(context.Context, useCases.RevealInput) useCases.RevealOutput {
		return useCases.RevealOutput{Error: &notFound}
	}
	f.remove.PerformFunc = func(context.Context, useCases.DeleteInput) useCases.DeleteOutput {
		return useCases.DeleteOutput{Error: &conflict}
	}
	id := uuid.NewV7().String()

	assert.Equal(t, http.StatusForbidden, serve(mux, http.MethodGet, base("me")+"/"+id, "", "").Code)
	assert.Equal(t, http.StatusNotFound, serve(mux, http.MethodPost, base("me")+"/"+id+"/reveal", "", "").Code)
	assert.Equal(t, http.StatusConflict, serve(mux, http.MethodDelete, base("me")+"/"+id, "", "").Code)
}
