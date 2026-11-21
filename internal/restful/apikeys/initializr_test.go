package apikeys_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/restful/apikeys"
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
	db := sqlitetest.New(t)
	deps := pkgmocks.NewDependencies(t, db)
	mux := http.NewServeMux()

	assert.NoError(t, apikeys.Init(db, deps, "https://vault.example.com", mux))

	_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, "/api/v1/api-keys/"+uuid.NewV7().String()+"?application_id="+uuid.NewV7().String(), nil))
	assert.NotEmpty(t, pattern, "a rota do service ficou registrada no mux")
}

func whenInitHasNoMux(t *testing.T) {
	db := sqlitetest.New(t)
	deps := pkgmocks.NewDependencies(t, db)

	assert.Error(t, apikeys.Init(db, deps, "https://vault.example.com", nil))
}

func whenInitGRPCRegistersService(t *testing.T) {
	db := sqlitetest.New(t)
	deps := pkgmocks.NewDependencies(t, db)
	registrar := grpc.NewServer()

	assert.NoError(t, apikeys.InitGRPC(db, deps, "https://vault.example.com", registrar))

	assert.Contains(t, registrar.GetServiceInfo(), "murkio.vault.v1.ApiKeyService")
}

func whenInitGRPCHasNoRegistrar(t *testing.T) {
	db := sqlitetest.New(t)
	deps := pkgmocks.NewDependencies(t, db)

	assert.Error(t, apikeys.InitGRPC(db, deps, "https://vault.example.com", nil))
}
