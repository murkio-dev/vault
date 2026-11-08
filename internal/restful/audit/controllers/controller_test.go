package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/controllers"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando lista com e sem barra final.", func(t *testing.T) { whenListAcceptsTrailingSlash(t) })
	t.Run("Quando os filtros são válidos.", func(t *testing.T) { whenFiltersAreValid(t) })
	t.Run("Quando os filtros são inválidos.", func(t *testing.T) { whenFiltersAreInvalid(t) })
	t.Run("Quando a trilha não aceita escrita.", func(t *testing.T) { whenTrailHasNoWriteRoute(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
}

// Functions

func newServer(t *testing.T) (*http.ServeMux, *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]) {
	t.Helper()
	fake := &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{}
	mux := http.NewServeMux()
	require.NoError(t, controllers.RegisterControllers(controllers.RegisterInput{Router: mux, ListUseCase: fake}))
	return mux, fake
}

func request(mux *http.ServeMux, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func base() string { return "/api/v1/applications/" + uuid.NewV7().String() + "/audit-logs" }

func whenRegisterIsSuccessful(t *testing.T) {
	mux, _ := newServer(t)

	assert.NotNil(t, mux)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	assert.Error(t, controllers.RegisterControllers(controllers.RegisterInput{Router: http.NewServeMux()}))
}

func whenListAcceptsTrailingSlash(t *testing.T) {
	mux, fake := newServer(t)
	path := base()

	assert.Equal(t, http.StatusOK, request(mux, http.MethodGet, path).Code)
	assert.Equal(t, http.StatusOK, request(mux, http.MethodGet, path+"/").Code)
	assert.Len(t, fake.Inputs, 2)
}

func whenFiltersAreValid(t *testing.T) {
	mux, fake := newServer(t)
	actor, cursor := uuid.NewV7(), uuid.NewV7()

	rec := request(mux, http.MethodGet, base()+"?limit=20&cursor="+cursor.String()+"&actor_id="+actor.String()+
		"&action=secret.revealed&outcome=denied&target_id=abc&from=2026-01-01T00:00:00Z&to=2026-12-31T23:59:59Z")

	require.Equal(t, http.StatusOK, rec.Code)
	if assert.Len(t, fake.Inputs, 1) {
		filter := fake.Inputs[0].Filter
		assert.Equal(t, 20, filter.Limit)
		assert.Equal(t, cursor, filter.Cursor)
		if assert.NotNil(t, filter.ActorID) {
			assert.Equal(t, actor, *filter.ActorID)
		}
		assert.Equal(t, "secret.revealed", filter.Action)
		assert.Equal(t, entities.AuditDenied, filter.Outcome)
		assert.Equal(t, "abc", filter.TargetID)
		if assert.NotNil(t, filter.From) {
			assert.True(t, filter.From.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
		}
	}
}

func whenFiltersAreInvalid(t *testing.T) {
	mux, fake := newServer(t)

	for _, query := range []string{"?limit=0", "?limit=x", "?cursor=nao-e-uuid", "?actor_id=x", "?outcome=talvez", "?from=ontem", "?to=2020", "?action=" + strings.Repeat("a", 65), "?target_id=" + strings.Repeat("a", 65)} {
		assert.Equal(t, http.StatusBadRequest, request(mux, http.MethodGet, base()+query).Code, query)
	}
	assert.Equal(t, http.StatusBadRequest, request(mux, http.MethodGet, "/api/v1/applications/xyz/audit-logs").Code)
	assert.Empty(t, fake.Inputs)
}

func whenTrailHasNoWriteRoute(t *testing.T) {
	mux, fake := newServer(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		assert.Equal(t, http.StatusMethodNotAllowed, request(mux, method, base()).Code, method)
	}
	assert.Empty(t, fake.Inputs)
}

func whenUseCaseReturnsErrors(t *testing.T) {
	mux, fake := newServer(t)
	notFound := entities.NotFoundError("Resource")
	fake.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &notFound}
	}

	assert.Equal(t, http.StatusNotFound, request(mux, http.MethodGet, base()).Code)
}
