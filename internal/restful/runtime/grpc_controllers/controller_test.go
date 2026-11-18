package grpc_controllers_test

import (
	"context"
	"github.com/murkio-dev/vault/internal/restful/runtime/grpc_controllers"
	"sync"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando lê o ambiente.", func(t *testing.T) { whenEnvIsRead(t) })
	t.Run("Quando filtra por nomes.", func(t *testing.T) { whenEnvIsFiltered(t) })
	t.Run("Quando o ator e o request id chegam no Requester.", func(t *testing.T) { whenRequesterIsExtracted(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
	t.Run("Quando há chamadas concorrentes.", func(t *testing.T) { whenCallsAreConcurrent(t) })
}

// Functions

func whenRegisterIsSuccessful(t *testing.T) {
	fake := &pkgmocks.UseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]{}

	err := grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{Registrar: grpc.NewServer(), EnvUseCase: fake})

	assert.NoError(t, err)
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	fake := &pkgmocks.UseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]{}

	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{Registrar: grpc.NewServer()}))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{EnvUseCase: fake}))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenEnvIsRead(t *testing.T) {
	client, fake := newClient(t, entities.Actor{})
	fake.PerformFunc = func(_ context.Context, input useCases.ReadEnvInput) useCases.ReadEnvOutput {
		return useCases.ReadEnvOutput{Data: useCases.Environment{UriID: input.UriID, Variables: []entities.RuntimeVariable{
			{Name: "DATABASE_URL", Type: entities.SecretURL, Version: 3, Value: "postgres://localhost/app"},
			{Name: "API_KEY", Type: entities.SecretAPIKey, Version: 1, Value: "sk-123"},
		}}}
	}

	response, err := client.ReadEnv(context.Background(), &vaultv1.ReadEnvRequest{UriId: "uri-do-membro"})

	require.NoError(t, err)
	assert.Equal(t, "uri-do-membro", response.GetUriId())
	if assert.Len(t, response.GetVariables(), 2, "a ordem do use case é preservada") {
		first := response.GetVariables()[0]
		assert.Equal(t, "DATABASE_URL", first.GetName())
		assert.Equal(t, string(entities.SecretURL), first.GetType())
		assert.Equal(t, int32(3), first.GetVersion())
		assert.Equal(t, "postgres://localhost/app", first.GetValue())
		assert.Equal(t, "API_KEY", response.GetVariables()[1].GetName())
	}
	if assert.Len(t, fake.Inputs, 1) {
		assert.Equal(t, "uri-do-membro", fake.Inputs[0].UriID)
		assert.Empty(t, fake.Inputs[0].Names, "sem filtro, o use case recebe a lista vazia")
	}
}

func whenEnvIsFiltered(t *testing.T) {
	client, fake := newClient(t, entities.Actor{})

	response, err := client.ReadEnv(context.Background(), &vaultv1.ReadEnvRequest{UriId: "uri", Names: []string{"API_KEY", "DATABASE_URL"}})

	require.NoError(t, err)
	assert.Empty(t, response.GetVariables())
	if assert.Len(t, fake.Inputs, 1) {
		assert.Equal(t, []string{"API_KEY", "DATABASE_URL"}, fake.Inputs[0].Names)
	}
}

func whenRequesterIsExtracted(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7(), TokenID: uuid.NewV7(), MemberID: uuid.NewV7(), ApplicationID: uuid.NewV7()}
	client, fake := newClient(t, actor)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")

	_, err := client.ReadEnv(ctx, &vaultv1.ReadEnvRequest{UriId: "uri"})

	require.NoError(t, err)
	if assert.Len(t, fake.Inputs, 1) {
		requester := fake.Inputs[0].Requester
		assert.Equal(t, actor, requester.Actor, "o ator vem do ctx posto pelo interceptor, nunca do pedido")
		assert.Equal(t, "req-123", requester.RequestID)
		assert.Equal(t, "grpc", requester.UserAgent)
		assert.Equal(t, "127.0.0.1", requester.ClientIP)
	}
}

func whenUseCaseReturnsErrors(t *testing.T) {
	internal := entities.Error{Status: 500, Title: "Internal Server Error", Detail: "segredo-interno"}
	cases := map[codes.Code]entities.Error{
		codes.Unauthenticated:  entities.UnauthorizedError(),
		codes.PermissionDenied: entities.ForbiddenError(),
		codes.NotFound:         entities.NotFoundError("Resource"),
		codes.InvalidArgument:  entities.ValidationError("Too many names."),
		codes.Internal:         internal,
	}
	for code, failure := range cases {
		client, fake := newClient(t, entities.Actor{})
		fake.PerformFunc = func(context.Context, useCases.ReadEnvInput) useCases.ReadEnvOutput {
			return useCases.ReadEnvOutput{Error: &failure, Data: useCases.Environment{Variables: []entities.RuntimeVariable{{Name: "API_KEY", Value: "nao-vazar"}}}}
		}

		response, err := client.ReadEnv(context.Background(), &vaultv1.ReadEnvRequest{UriId: "uri"})

		assert.Equal(t, code, status.Code(err), code.String())
		assert.Nil(t, response, "com erro, nenhuma variável sai")
		assert.NotContains(t, status.Convert(err).Message(), "segredo-interno", "o erro interno não vaza pelo canal")
		assert.NotContains(t, status.Convert(err).Message(), "nao-vazar")
	}
}

func whenCallsAreConcurrent(t *testing.T) {
	client, fake := newClient(t, entities.Actor{})
	fake.PerformFunc = func(_ context.Context, input useCases.ReadEnvInput) useCases.ReadEnvOutput {
		return useCases.ReadEnvOutput{Data: useCases.Environment{UriID: input.UriID}}
	}
	const calls = 20
	uris := make([]string, calls)

	var wg sync.WaitGroup
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.ReadEnv(context.Background(), &vaultv1.ReadEnvRequest{UriId: uuid.NewV7().String()})
			if assert.NoError(t, err) {
				uris[i] = response.GetUriId()
			}
		}()
	}
	wg.Wait()

	assert.Len(t, fake.Inputs, calls)
	seen := map[string]bool{}
	for _, uri := range uris {
		seen[uri] = true
	}
	assert.Len(t, seen, calls, "cada chamada recebe a própria resposta")
}

// Helpers

func newClient(t *testing.T, actor entities.Actor) (vaultv1.VaultServiceClient, *pkgmocks.UseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]) {
	t.Helper()

	fake := &pkgmocks.UseCase[useCases.ReadEnvInput, useCases.ReadEnvOutput]{}
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{Registrar: registrar, EnvUseCase: fake})
	})

	return vaultv1.NewVaultServiceClient(conn), fake
}
