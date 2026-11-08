package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/controllers"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/murkio-dev/vault/pkg/utils"
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
}

// Functions

func whenRegisterIsSuccessful(t *testing.T, mockUseCase *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]) {
	input := controllers.RegisterInput{
		Router:         http.NewServeMux(),
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	}

	err := controllers.RegisterControllers(input)

	assert.NoError(t, err)
}

func whenRegisterFailsDueToNilParams(t *testing.T, mockUseCase *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]) {
	input := controllers.RegisterInput{
		Router:         nil,
		CreateUseCase:  mockUseCase,
		GetByIDUseCase: &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		ListUseCase:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		UpdateUseCase:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		UpgradeUseCase: &pkgmocks.UseCase[useCases.UpgradeInput, useCases.UpgradeOutput]{},
		DeleteUseCase:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	}

	err := controllers.RegisterControllers(input)

	assert.Error(t, err, "deveria retornar erro por causa do AssertNotNilParams")
}

func whenRequestIsValid(t *testing.T, _ *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]) {
	appName := utils.GenRandomApplicationName()

	mockUseCase := &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{
		PerformFunc: func(ctx context.Context, input useCases.CreateInput) useCases.CreateOutput {
			return useCases.CreateOutput{
				Data: input.Application,
			}
		},
	}

	mux := http.NewServeMux()
	controllers.RegisterControllers(controllers.RegisterInput{
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
	controllers.RegisterControllers(controllers.RegisterInput{
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
	controllers.RegisterControllers(controllers.RegisterInput{
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
	input := controllers.RegisterInput{
		Router:        http.NewServeMux(),
		CreateUseCase: nil,
	}

	err := controllers.RegisterControllers(input)

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
	controllers.RegisterControllers(controllers.RegisterInput{
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
	controllers.RegisterControllers(controllers.RegisterInput{
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
	controllers.RegisterControllers(controllers.RegisterInput{
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
	controllers.RegisterControllers(controllers.RegisterInput{
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
