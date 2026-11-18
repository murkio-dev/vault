package http_controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/http_controllers"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestControllers(t *testing.T) {

	mockUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			return useCases.CreateOutput{
				Data: input.Application,
			}
		},
	}

	// Small tests
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t, mockUseCase) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t, mockUseCase) })
	t.Run("Quando a requisição é válida e retorna 201.", func(t *testing.T) { whenRequestIsValid(t, mockUseCase) })
	t.Run("Quando o payload é um JSON inválido e retorna 400.", func(t *testing.T) { whenPayloadIsInvalidJson(t, mockUseCase) })
	t.Run("Quando o UseCase falha e retorna 500.", func(t *testing.T) { whenUseCaseReturnsError(t) })
	t.Run("Quando falta o UseCase no registro.", func(t *testing.T) { whenRegisterFailsDueToNilUseCase(t) })
	t.Run("Quando os headers HTTP são extraídos corretamente.", func(t *testing.T) { whenHeadersAreExtractedToRequester(t) })
	t.Run("Quando o output contém erros de negócio.", func(t *testing.T) { whenOutputContainsBusinessErrors(t) })
	t.Run("Quando há 2 requisições ao mesmo tempo de IPs diferentes.", func(t *testing.T) { whenConcurrentRequestsFromDifferentIPs(t) })
	t.Run("Quando há 2 requisições ao mesmo tempo do mesmo IP.", func(t *testing.T) { whenConcurrentRequestsFromSameIP(t) })
	t.Run("Quando busca pelo id e retorna 200.", func(t *testing.T) { whenGetByIDIsValid(t) })
	t.Run("Quando o id da busca é inválido e retorna 400.", func(t *testing.T) { whenGetByIDHasInvalidID(t) })
	t.Run("Quando lista com página e limite e retorna 200.", func(t *testing.T) { whenListIsValid(t) })
	t.Run("Quando a paginação é inválida e retorna 400.", func(t *testing.T) { whenListHasInvalidPagination(t) })
	t.Run("Quando substitui por id e por lote e retorna 200.", func(t *testing.T) { whenUpdateIsValid(t) })
	t.Run("Quando a substituição tem corpo ou id inválido e retorna 400.", func(t *testing.T) { whenUpdateIsInvalid(t) })
	t.Run("Quando atualiza parcialmente por id e por lote e retorna 200.", func(t *testing.T) { whenUpgradeIsValid(t) })
	t.Run("Quando a atualização parcial tem corpo ou id inválido e retorna 400.", func(t *testing.T) { whenUpgradeIsInvalid(t) })
	t.Run("Quando apaga por id e por lote e retorna 204.", func(t *testing.T) { whenDeleteIsValid(t) })
	t.Run("Quando o apagar tem id ou corpo inválido e retorna 400.", func(t *testing.T) { whenDeleteIsInvalid(t) })
}

// Functions

func whenRegisterIsSuccessful(t *testing.T, mockUseCase *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]) {
	input := http_controllers.RegisterInput{
		Router:         http.NewServeMux(),
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	}

	err := http_controllers.RegisterControllers(input)

	assert.NoError(t, err)
}

func whenRegisterFailsDueToNilParams(t *testing.T, mockUseCase *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]) {
	input := http_controllers.RegisterInput{
		Router:         nil,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	}

	err := http_controllers.RegisterControllers(input)

	assert.Error(t, err, "deveria retornar erro por causa do AssertNotNilParams")
}

func whenRequestIsValid(t *testing.T, _ *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]) {
	appName := pkgmocks.GenRandomApplicationName()

	mockUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			return useCases.CreateOutput{
				Data: input.Application,
			}
		},
	}

	mux := http.NewServeMux()
	http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	})

	bodyMap := []map[string]any{
		{
			"name": appName,
		},
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var output struct {
		Data entities.Application `json:"data"`
	}
	err := json.Unmarshal(recorder.Body.Bytes(), &output)

	assert.NoError(t, err)
	assert.Equal(t, appName, output.Data.Name, "item único deve vir como objeto em data")
}

func whenPayloadIsInvalidJson(t *testing.T, mockUseCase *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]) {
	mux := http.NewServeMux()
	http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	})

	body := `[{"name":"my-app"`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func whenUseCaseReturnsError(t *testing.T) {
	mockFailingUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			appName := "my-app"
			return useCases.CreateOutput{
				Errors: []entities.Error{
					entities.InternalServerError(errors.New("fake db error"), &appName),
				},
			}
		},
	}

	mux := http.NewServeMux()
	http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  mockFailingUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	})

	body := `[{"name":"my-app"}]`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func whenRegisterFailsDueToNilUseCase(t *testing.T) {
	input := http_controllers.RegisterInput{
		Router:        http.NewServeMux(),
		CreateUseCase: nil,
	}

	err := http_controllers.RegisterControllers(input)

	assert.Error(t, err)
}

func whenHeadersAreExtractedToRequester(t *testing.T) {
	var capturedInput useCases.CreateInput

	mockUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			capturedInput = input
			return useCases.CreateOutput{
				Data: input.Application,
			}
		},
	}

	mux := http.NewServeMux()
	http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	})

	body := `[{"name":"app-test"}]`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewBufferString(body))
	request.Header.Set("CF-Connecting-IP", "200.100.50.1")
	request.Header.Set("User-Agent", "vault-test-agent")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "200.100.50.1", capturedInput.Requester.ClientIP)
	assert.Equal(t, "vault-test-agent", capturedInput.Requester.UserAgent)
}

func whenOutputContainsBusinessErrors(t *testing.T) {
	mockUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			fieldName := "app-dup"

			return useCases.CreateOutput{
				Data: input.Application,
				Errors: []entities.Error{
					entities.ApplicationAlreadyExistsError(fieldName),
				},
			}
		},
	}

	mux := http.NewServeMux()
	http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	})

	body := `[{"name":"app-dup"}]`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusMultiStatus, recorder.Code)

	var output useCases.CreateOutput
	err := json.Unmarshal(recorder.Body.Bytes(), &output)

	assert.NoError(t, err)
	assert.NotEmpty(t, output.Data)
	assert.Len(t, output.Errors, 1)
	assert.Equal(t, entities.ErrTypeConflict, output.Errors[0].Type)
	assert.Equal(t, http.StatusConflict, output.Errors[0].Status)
}

func whenConcurrentRequestsFromDifferentIPs(t *testing.T) {
	var mu sync.Mutex
	var capturedIPs []string

	mockUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			mu.Lock()
			capturedIPs = append(capturedIPs, input.Requester.ClientIP)
			mu.Unlock()
			return useCases.CreateOutput{Data: input.Application}
		},
	}

	mux := http.NewServeMux()
	http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	})

	ips := []string{"200.100.50.1", "198.51.100.2"}
	var wg sync.WaitGroup

	for _, ip := range ips {
		wg.Add(1)
		go func(clientIP string) {
			defer wg.Done()

			body := `[{"name":"app-diff-ip"}]`
			request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewBufferString(body))
			request.Header.Set("CF-Connecting-IP", clientIP)

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusCreated, recorder.Code)
		}(ip)
	}

	wg.Wait()

	assert.Len(t, capturedIPs, 2)
	assert.Contains(t, capturedIPs, "200.100.50.1")
	assert.Contains(t, capturedIPs, "198.51.100.2")
}

func whenConcurrentRequestsFromSameIP(t *testing.T) {
	var mu sync.Mutex
	var requestCount int

	mockUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			mu.Lock()
			requestCount++
			mu.Unlock()
			return useCases.CreateOutput{Data: input.Application}
		},
	}

	mux := http.NewServeMux()
	http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         mux,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	})

	clientIP := "10.0.0.55"
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			body := `[{"name":"app-same-ip"}]`
			request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewBufferString(body))
			request.Header.Set("CF-Connecting-IP", clientIP)

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusCreated, recorder.Code)
		}()
	}

	wg.Wait()

	assert.Equal(t, 2, requestCount, "Deveria ter processado 2 requisições concorrentes do mesmo IP")
}

type routesRig struct {
	mux     *http.ServeMux
	getByID *pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]
	list    *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	update  *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	upgrade *pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]
	delete  *pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]
}

func newRoutesRig(t *testing.T) *routesRig {
	t.Helper()

	rig := &routesRig{
		mux:     http.NewServeMux(),
		getByID: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		list:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		update:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		upgrade: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		delete:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	}

	err := http_controllers.RegisterControllers(http_controllers.RegisterInput{
		Router:         rig.mux,
		CreateUseCase:  &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		GetByIDUseCase: rig.getByID,
		ListUseCase:    rig.list,
		UpdateUseCase:  rig.update,
		UpgradeUseCase: rig.upgrade,
		DeleteUseCase:  rig.delete,
	})
	assert.NoError(t, err)

	return rig
}

func (r *routesRig) call(method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	r.mux.ServeHTTP(recorder, request)

	return recorder
}

func whenGetByIDIsValid(t *testing.T) {
	rig := newRoutesRig(t)
	id := uuid.NewV7()

	recorder := rig.call(http.MethodGet, "/api/v1/applications/"+id.String(), "")

	assert.Equal(t, http.StatusOK, recorder.Code)
	if assert.Len(t, rig.getByID.Inputs, 1) {
		assert.Equal(t, id, rig.getByID.Inputs[0].Application)
	}
}

func whenGetByIDHasInvalidID(t *testing.T) {
	rig := newRoutesRig(t)

	recorder := rig.call(http.MethodGet, "/api/v1/applications/nao-e-uuid", "")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Empty(t, rig.getByID.Inputs, "id inválido não chega ao use case")
}

func whenListIsValid(t *testing.T) {
	rig := newRoutesRig(t)

	assert.Equal(t, http.StatusOK, rig.call(http.MethodGet, "/api/v1/applications?page=2&limit=5", "").Code)
	assert.Equal(t, http.StatusOK, rig.call(http.MethodGet, "/api/v1/applications/", "").Code)

	if assert.Len(t, rig.list.Inputs, 2) {
		assert.Equal(t, 2, rig.list.Inputs[0].Page)
		assert.Equal(t, 5, rig.list.Inputs[0].Limit)
		assert.Equal(t, 1, rig.list.Inputs[1].Page, "sem parâmetros vale a página 1")
		assert.Equal(t, entities.MaxPageSize, rig.list.Inputs[1].Limit)
	}
}

func whenListHasInvalidPagination(t *testing.T) {
	rig := newRoutesRig(t)

	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodGet, "/api/v1/applications?page=0", "").Code)
	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodGet, "/api/v1/applications?limit=abc", "").Code)
	assert.Empty(t, rig.list.Inputs)
}

func whenUpdateIsValid(t *testing.T) {
	rig := newRoutesRig(t)
	id := uuid.NewV7()
	name := pkgmocks.GenRandomApplicationName()

	single := rig.call(http.MethodPut, "/api/v1/applications/"+id.String(), `{"name":"`+name+`"}`)
	batch := rig.call(http.MethodPut, "/api/v1/applications", `[{"id":"`+id.String()+`","name":"`+name+`"}]`)

	assert.Equal(t, http.StatusOK, single.Code)
	assert.Equal(t, http.StatusOK, batch.Code)
	if assert.Len(t, rig.update.Inputs, 2) {
		assert.Equal(t, id, rig.update.Inputs[0].Applications[0].ID, "o id do path vence")
		assert.Equal(t, name, rig.update.Inputs[0].Applications[0].Name)
		assert.Equal(t, id, rig.update.Inputs[1].Applications[0].ID)
	}
}

func whenUpdateIsInvalid(t *testing.T) {
	rig := newRoutesRig(t)
	id := uuid.NewV7().String()

	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodPut, "/api/v1/applications", "{").Code)
	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodPut, "/api/v1/applications/nao-e-uuid", `{"name":"x"}`).Code)
	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodPut, "/api/v1/applications/"+id, `[{"name":"a"},{"name":"b"}]`).Code, "id no path pede exatamente um item")
	assert.Empty(t, rig.update.Inputs)
}

func whenUpgradeIsValid(t *testing.T) {
	rig := newRoutesRig(t)
	id := uuid.NewV7()
	name := pkgmocks.GenRandomApplicationName()

	single := rig.call(http.MethodPatch, "/api/v1/applications/"+id.String(), `{"name":"`+name+`"}`)
	batch := rig.call(http.MethodPatch, "/api/v1/applications/", `[{"id":"`+id.String()+`","name":"`+name+`"}]`)

	assert.Equal(t, http.StatusOK, single.Code)
	assert.Equal(t, http.StatusOK, batch.Code)
	if assert.Len(t, rig.upgrade.Inputs, 2) {
		assert.Equal(t, id, rig.upgrade.Inputs[0].Applications[0].ID)
		assert.Equal(t, id, rig.upgrade.Inputs[1].Applications[0].ID)
	}
}

func whenUpgradeIsInvalid(t *testing.T) {
	rig := newRoutesRig(t)

	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodPatch, "/api/v1/applications", "").Code)
	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodPatch, "/api/v1/applications/nao-e-uuid", `{"name":"x"}`).Code)
	assert.Empty(t, rig.upgrade.Inputs)
}

func whenDeleteIsValid(t *testing.T) {
	rig := newRoutesRig(t)
	first, second := uuid.NewV7(), uuid.NewV7()

	single := rig.call(http.MethodDelete, "/api/v1/applications/"+first.String(), "")
	batch := rig.call(http.MethodDelete, "/api/v1/applications", `["`+first.String()+`","`+second.String()+`","`+first.String()+`"]`)

	assert.Equal(t, http.StatusNoContent, single.Code)
	assert.Equal(t, http.StatusNoContent, batch.Code)
	if assert.Len(t, rig.delete.Inputs, 2) {
		assert.Equal(t, []uuid.UUID{first}, rig.delete.Inputs[0].IDs)
		assert.Equal(t, []uuid.UUID{first, second, first}, rig.delete.Inputs[1].IDs)
		assert.True(t, rig.delete.Inputs[1].DuplicatedIDs[first], "o id repetido chega marcado ao use case")
		assert.False(t, rig.delete.Inputs[1].DuplicatedIDs[second])
	}
}

func whenDeleteIsInvalid(t *testing.T) {
	rig := newRoutesRig(t)

	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodDelete, "/api/v1/applications/nao-e-uuid", "").Code)
	assert.Equal(t, http.StatusBadRequest, rig.call(http.MethodDelete, "/api/v1/applications", "").Code)
	assert.Empty(t, rig.delete.Inputs)
}
