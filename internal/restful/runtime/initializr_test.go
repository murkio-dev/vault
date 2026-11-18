package runtime_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/restful/runtime"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// Main test

func TestInitializr(t *testing.T) {
	t.Run("Quando registra as rotas HTTP.", func(t *testing.T) { whenInitRegistersRoutes(t) })
	t.Run("Quando falta o mux no registro HTTP.", func(t *testing.T) { whenInitHasNoMux(t) })
	t.Run("Quando registra o serviço gRPC.", func(t *testing.T) { whenInitGRPCRegistersService(t) })
	t.Run("Quando falta o registrador gRPC.", func(t *testing.T) { whenInitGRPCHasNoRegistrar(t) })
}

// Functions

func whenInitRegistersRoutes(t *testing.T) {
	deps := pkgmocks.NewDependencies(t, sqlitetest.New(t))
	mux := http.NewServeMux()

	assert.NoError(t, runtime.Init(deps, mux))

	_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/runtime/%s/env", uuid.NewV7().String()), nil))
	assert.NotEmpty(t, pattern, "a rota do service ficou registrada no mux")
}

func whenInitHasNoMux(t *testing.T) {
	deps := pkgmocks.NewDependencies(t, sqlitetest.New(t))

	assert.Error(t, runtime.Init(deps, nil))
}

func whenInitGRPCRegistersService(t *testing.T) {
	deps := pkgmocks.NewDependencies(t, sqlitetest.New(t))
	registrar := grpc.NewServer()

	assert.NoError(t, runtime.InitGRPC(deps, registrar))

	assert.Contains(t, registrar.GetServiceInfo(), "murkio.vault.v1.VaultService")
}

func whenInitGRPCHasNoRegistrar(t *testing.T) {
	deps := pkgmocks.NewDependencies(t, sqlitetest.New(t))

	assert.Error(t, runtime.InitGRPC(deps, nil))
}
