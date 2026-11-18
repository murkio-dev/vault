package grpc_controllers_test

import (
	"context"
	"encoding/json"
	"github.com/murkio-dev/vault/internal/restful/audit/grpc_controllers"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
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
	t.Run("Quando lista a trilha com filtros.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando é a última página.", func(t *testing.T) { whenListIsTheLastPage(t) })
	t.Run("Quando um filtro é inválido.", func(t *testing.T) { whenFilterIsInvalid(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
	t.Run("Quando o ator e o request id chegam no Requester.", func(t *testing.T) { whenRequesterIsExtracted(t) })
	t.Run("Quando há chamadas concorrentes.", func(t *testing.T) { whenCallsAreConcurrent(t) })
}

// Functions

func whenRegisterIsSuccessful(t *testing.T) {
	fake := &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{}

	assert.NoError(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{Registrar: grpc.NewServer(), ListUseCase: fake}))
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	fake := &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{}

	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{Registrar: grpc.NewServer()}))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{ListUseCase: fake}))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenListSucceeds(t *testing.T) {
	client, fake := newClient(t, entities.Actor{})
	app, actor, token := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	createdAt := time.Now().UTC().Truncate(time.Second)
	byPerson := entities.AuditLog{
		ID: uuid.NewV7(), ApplicationID: &app, ActorType: "user", ActorID: &actor, Action: entities.AuditSecretUpdated, Outcome: entities.AuditAllowed,
		TargetType: "secret", TargetID: "alvo", IP: "127.0.0.1", UserAgent: "grpc", RequestID: "req-1", Metadata: json.RawMessage(`{"version":2}`), CreatedAt: createdAt,
	}
	byToken := entities.AuditLog{ID: uuid.NewV7(), ActorType: "token", TokenID: &token, Action: "env.read", Outcome: entities.AuditDenied, CreatedAt: createdAt}
	next := byToken.ID.String()
	fake.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Data: []entities.AuditLog{byPerson, byToken}, Page: &useCases.CursorPage{NextCursor: &next}}
	}

	response, err := client.ListAuditLogs(context.Background(), &vaultv1.ListAuditLogsRequest{ApplicationId: app.String(), Limit: 2, ActorId: actor.String(), Outcome: "allowed", Action: "secret.updated"})

	require.NoError(t, err)
	assert.Equal(t, next, response.GetNextCursor())
	if assert.Len(t, response.GetLogs(), 2, "a ordem do use case é preservada") {
		first, second := response.GetLogs()[0], response.GetLogs()[1]
		assert.Equal(t, byPerson.ID.String(), first.GetId())
		assert.Equal(t, app.String(), first.GetApplicationId())
		assert.Equal(t, "user", first.GetActorType())
		assert.Equal(t, actor.String(), first.GetActorId())
		assert.Empty(t, first.GetTokenId())
		assert.Equal(t, entities.AuditSecretUpdated, first.GetAction())
		assert.Equal(t, "allowed", first.GetOutcome())
		assert.Equal(t, "secret", first.GetTargetType())
		assert.Equal(t, "alvo", first.GetTargetId())
		assert.Equal(t, "127.0.0.1", first.GetIp())
		assert.Equal(t, "req-1", first.GetRequestId())
		assert.JSONEq(t, `{"version":2}`, first.GetMetadata())
		assert.True(t, createdAt.Equal(first.GetCreatedAt().AsTime()))
		assert.Empty(t, second.GetApplicationId())
		assert.Empty(t, second.GetActorId())
		assert.Equal(t, token.String(), second.GetTokenId())
		assert.Equal(t, "denied", second.GetOutcome())
	}
	if assert.Len(t, fake.Inputs, 1) {
		input := fake.Inputs[0]
		assert.Equal(t, app, input.Application)
		assert.Equal(t, 2, input.Filter.Limit)
		assert.Equal(t, entities.AuditAllowed, input.Filter.Outcome)
		assert.Equal(t, "secret.updated", input.Filter.Action)
		if assert.NotNil(t, input.Filter.ActorID) {
			assert.Equal(t, actor, *input.Filter.ActorID)
		}
	}
}

func whenListIsTheLastPage(t *testing.T) {
	client, fake := newClient(t, entities.Actor{})
	fake.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Data: []entities.AuditLog{}, Page: &useCases.CursorPage{}}
	}

	response, err := client.ListAuditLogs(context.Background(), &vaultv1.ListAuditLogsRequest{ApplicationId: uuid.NewV7().String()})

	require.NoError(t, err)
	assert.Empty(t, response.GetLogs())
	assert.Empty(t, response.GetNextCursor(), "sem próxima página, o cursor fica vazio")
}

func whenFilterIsInvalid(t *testing.T) {
	client, fake := newClient(t, entities.Actor{})
	app := uuid.NewV7().String()

	requests := map[string]*vaultv1.ListAuditLogsRequest{
		"aplicação": {ApplicationId: "nao-e-uuid"},
		"sem app":   {},
		"limit":     {ApplicationId: app, Limit: -1},
		"cursor":    {ApplicationId: app, Cursor: "' OR 1=1 --"},
		"actor_id":  {ApplicationId: app, ActorId: "nao-e-uuid"},
		"outcome":   {ApplicationId: app, Outcome: "talvez"},
		"action":    {ApplicationId: app, Action: string(make([]byte, 65))},
		"target_id": {ApplicationId: app, TargetId: string(make([]byte, 65))},
	}
	for name, request := range requests {
		_, err := client.ListAuditLogs(context.Background(), request)

		assert.Equal(t, codes.InvalidArgument, status.Code(err), name)
		assert.NotContains(t, status.Convert(err).Message(), "OR 1=1", "o erro não repete o valor recebido")
	}
	assert.Empty(t, fake.Inputs, "filtro inválido não chega ao use case")
}

func whenUseCaseReturnsErrors(t *testing.T) {
	forbidden := entities.ForbiddenError()
	internal := entities.Error{Status: http.StatusInternalServerError, Title: "Internal Server Error", Detail: "segredo-interno"}
	for code, failure := range map[codes.Code]entities.Error{codes.PermissionDenied: forbidden, codes.Internal: internal} {
		client, fake := newClient(t, entities.Actor{})
		fake.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
			return useCases.ListOutput{Error: &failure, Data: []entities.AuditLog{{Action: "nao-vazar"}}}
		}

		response, err := client.ListAuditLogs(context.Background(), &vaultv1.ListAuditLogsRequest{ApplicationId: uuid.NewV7().String()})

		assert.Nil(t, response, "com erro, nenhuma linha da trilha sai")
		assert.Equal(t, code, status.Code(err))
		assert.NotContains(t, status.Convert(err).Message(), "segredo-interno")
	}
}

func whenRequesterIsExtracted(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7(), PlatformAdmin: true}
	client, fake := newClient(t, actor)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")

	_, err := client.ListAuditLogs(ctx, &vaultv1.ListAuditLogsRequest{ApplicationId: uuid.NewV7().String()})

	require.NoError(t, err)
	if assert.Len(t, fake.Inputs, 1) {
		assert.Equal(t, actor, fake.Inputs[0].Requester.Actor, "o ator vem do ctx posto pelo interceptor, nunca do pedido")
		assert.Equal(t, "req-123", fake.Inputs[0].Requester.RequestID)
		assert.Equal(t, "grpc", fake.Inputs[0].Requester.UserAgent)
	}
}

func whenCallsAreConcurrent(t *testing.T) {
	client, fake := newClient(t, entities.Actor{})
	fake.PerformFunc = func(_ context.Context, input useCases.ListInput) useCases.ListOutput {
		next := input.Application.String()
		return useCases.ListOutput{Page: &useCases.CursorPage{NextCursor: &next}}
	}
	const calls = 20
	asked, answered := make([]string, calls), make([]string, calls)

	var wg sync.WaitGroup
	for i := range calls {
		asked[i] = uuid.NewV7().String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.ListAuditLogs(context.Background(), &vaultv1.ListAuditLogsRequest{ApplicationId: asked[i]})
			if assert.NoError(t, err) {
				answered[i] = response.GetNextCursor()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, asked, answered, "cada chamada recebe a própria resposta")
	assert.Len(t, fake.Inputs, calls)
}

// Helpers

func newClient(t *testing.T, actor entities.Actor) (vaultv1.AuditServiceClient, *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]) {
	t.Helper()

	fake := &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{}
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{Registrar: registrar, ListUseCase: fake})
	})

	return vaultv1.NewAuditServiceClient(conn), fake
}
