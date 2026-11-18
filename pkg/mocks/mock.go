// Package mocks tem o fake único do contracts.IUseCase, que os testes dos
// controllers de todos os services usam. Os fakes de repository são concretos e
// ficam em mocks/ de cada service.
package mocks

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"

	"github.com/murkio-dev/vault/internal/contracts"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/dependencies"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Mocks

// UseCase é o fake de contracts.IUseCase para os testes dos controllers: guarda
// cada Input recebido e devolve o Output de PerformFunc (ou o valor zero, que é
// um sucesso sem dados). É seguro para uso concorrente.
type UseCase[I, O any] struct {
	PerformFunc func(ctx context.Context, input I) O

	mu     sync.Mutex
	Inputs []I
}

func (m *UseCase[I, O]) Perform(ctx context.Context, input I) O {
	m.mu.Lock()
	m.Inputs = append(m.Inputs, input)
	m.mu.Unlock()
	if m.PerformFunc != nil {
		return m.PerformFunc(ctx, input)
	}
	var zero O
	return zero
}

var _ contracts.IUseCase[int, int] = (*UseCase[int, int])(nil)

// TokenAuthenticator é o fake de authn.TokenAuthenticator: devolve Actor e Err
// sem tocar banco e guarda cada token recebido. É seguro para uso concorrente.
type TokenAuthenticator struct {
	Actor entities.Actor
	Err   error

	mu     sync.Mutex
	Tokens []string
}

func (m *TokenAuthenticator) Authenticate(_ context.Context, raw string) (entities.Actor, error) {
	m.mu.Lock()
	m.Tokens = append(m.Tokens, raw)
	m.mu.Unlock()
	return m.Actor, m.Err
}

var _ authn.TokenAuthenticator = (*TokenAuthenticator)(nil)

// Helpers

// NewGRPCConn sobe um servidor gRPC de teste em loopback e devolve a conexão
// de cliente, para os testes dos grpc_controllers de todos os services não
// repetirem a montagem. Não usa o interceptor real: no lugar dele, um
// interceptor de teste põe no ctx o ator recebido, como a autenticação faria.
// Recebe o ator que os handlers devem enxergar (o zero simula ctx sem ator) e
// register, que registra os controllers do service no servidor.
// Falha o teste se register devolver erro ou se a porta não abrir. O servidor
// e a conexão são fechados no Cleanup do teste.
// Exemplo: conn := NewGRPCConn(t, actor, func(r grpc.ServiceRegistrar) error {
// return grpc_controllers.RegisterControllers(input(r)) }).
func NewGRPCConn(t testing.TB, actor entities.Actor, register func(grpc.ServiceRegistrar) error) *grpc.ClientConn {
	t.Helper()

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(entities.WithActor(ctx, actor), req)
	}))
	if err := register(server); err != nil {
		t.Fatalf("register controllers: %v", err)
	}

	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

const (
	alphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"
	middleChars  = "abcdefghijklmnopqrstuvwxyz0123456789-"
)

func GenRandomApplicationName() string {
	// 3 a 50 caracteres no total (o limite do nome de uma aplicação).
	middleLen := rand.Intn(47) + 1
	result := make([]byte, 1+middleLen+1)
	result[0] = alphanumeric[rand.Intn(len(alphanumeric))]

	for i := 1; i <= middleLen; i++ {
		result[i] = middleChars[rand.Intn(len(middleChars))]
	}

	result[1+middleLen] = alphanumeric[rand.Intn(len(alphanumeric))]

	return string(result)
}

func GenRandomIP(ipType string) string {
	var o1 int

	o2 := rand.Intn(256)
	o3 := rand.Intn(256)
	o4 := rand.Intn(256)

	switch ipType {
	case "a":
		o1 = rand.Intn(126) + 1
	case "b":
		o1 = rand.Intn(64) + 128
	case "c":
		o1 = rand.Intn(32) + 192
	case "d":
		o1 = rand.Intn(16) + 224
	case "e":
		o1 = rand.Intn(15) + 240
	default:
		o1 = rand.Intn(32) + 192
	}

	return fmt.Sprintf("%d.%d.%d.%d", o1, o2, o3, o4)
}

func GenRandomLetterAtoE() string {
	return string(byte('a' + rand.Intn(5)))
}

func GenRandomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:124.0) Gecko/20100101 Firefox/124.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:124.0) Gecko/20100101 Firefox/124.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36 Edg/123.0.0.0",
	}

	return userAgents[rand.Intn(len(userAgents))]
}

func GenRandomCountry() string {
	countries := []string{
		"Brazil",
		"United States",
		"Canada",
		"Germany",
		"Japan",
		"United Kingdom",
		"France",
		"Australia",
		"India",
		"Argentina",
		"Spain",
		"Italy",
	}

	return countries[rand.Intn(len(countries))]
}

func GenRandomOrigin() string {
	origins := []string{
		"https://application.murkio.dev",
		"https://dashboard.murkio.dev",
		"https://staging.murkio.dev",
		"http://localhost:3000",
		"http://localhost:8080",
	}

	return origins[rand.Intn(len(origins))]
}

func GenRandomCdnRayID() string {
	const hexChars = "0123456789abcdef"
	bytes := make([]byte, 16)
	for i := range bytes {
		bytes[i] = hexChars[rand.Intn(len(hexChars))]
	}

	pops := []string{"GRU", "GIG", "EZE", "MIA", "LHR"}
	pop := pops[rand.Intn(len(pops))]

	return fmt.Sprintf("%s-%s", string(bytes), pop)
}

const upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GenRandomSecretName() string {
	size := rand.Intn(20) + 2
	name := make([]byte, size)
	name[0] = upperChars[rand.Intn(len(upperChars))]
	for i := 1; i < size; i++ {
		name[i] = (upperChars + "0123456789_")[rand.Intn(len(upperChars)+11)]
	}
	return string(name)
}

func GenRandomSecretValue() string {
	size := rand.Intn(40) + 8
	value := make([]byte, size)
	for i := range value {
		value[i] = alphanumeric[rand.Intn(len(alphanumeric))]
	}
	return "v-" + string(value)
}

func GenRandomEmail() string {
	return GenRandomSecretValue()[:10] + "@example.com"
}

func GenRandomTokenName() string {
	return "token-" + GenRandomSecretValue()[:8]
}

// NewDependencies monta o Dependencies de verdade (dependencies.New) sobre o db
// de teste, com uma chave de cifra zerada que nunca sai do teste. Serve aos
// initializr_test de cada service, que só conferem se as rotas e os handlers
// gRPC ficaram registrados.
func NewDependencies(t testing.TB, db *sql.DB) contracts.Dependencies {
	t.Helper()

	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	deps, _ := dependencies.New(db, box)
	return deps
}
