package grpc_controllers_test

import (
	"context"
	"github.com/murkio-dev/vault/internal/restful/application/grpc_controllers"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/mocks"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
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
	t.Run("Quando cria uma aplicação e um lote.", func(t *testing.T) { whenCreateSucceeds(t) })
	t.Run("Quando o lote tem sucesso parcial.", func(t *testing.T) { whenBatchIsPartial(t) })
	t.Run("Quando todos os itens do lote falham.", func(t *testing.T) { whenBatchFailsEntirely(t) })
	t.Run("Quando o lote vem vazio.", func(t *testing.T) { whenBatchIsEmpty(t) })
	t.Run("Quando um id não é uuid.", func(t *testing.T) { whenIDIsInvalid(t) })
	t.Run("Quando busca uma aplicação.", func(t *testing.T) { whenGetSucceeds(t) })
	t.Run("Quando a busca falha.", func(t *testing.T) { whenGetFails(t) })
	t.Run("Quando lista com paginação.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando a listagem falha.", func(t *testing.T) { whenListFails(t) })
	t.Run("Quando atualiza com id repetido.", func(t *testing.T) { whenUpdateHasDuplicatedIDs(t) })
	t.Run("Quando apaga um lote.", func(t *testing.T) { whenDeleteSucceeds(t) })
	t.Run("Quando o erro interno vem num item do lote.", func(t *testing.T) { whenItemErrorIsInternal(t) })
	t.Run("Quando o ator e o request id chegam no Requester.", func(t *testing.T) { whenRequesterIsExtracted(t) })
	t.Run("Quando há chamadas concorrentes.", func(t *testing.T) { whenCallsAreConcurrent(t) })
}

// Functions

func whenRegisterIsSuccessful(t *testing.T) {
	f := newFakes()

	assert.NoError(t, grpc_controllers.RegisterControllers(f.input(grpc.NewServer())))
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	withoutRegistrar := newFakes().input(nil)
	withoutCreate := newFakes().input(grpc.NewServer())
	withoutCreate.CreateUseCase = nil
	withoutDelete := newFakes().input(grpc.NewServer())
	withoutDelete.DeleteUseCase = nil

	assert.Error(t, grpc_controllers.RegisterControllers(withoutRegistrar))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutCreate))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutDelete))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenCreateSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	owner := uuid.NewV7()
	f.create.PerformFunc = func(_ context.Context, input useCases.CreateInput) useCases.CreateOutput {
		data := make([]entities.Application, len(input.Application))
		for i, application := range input.Application {
			data[i] = entities.Application{ID: uuid.NewV7(), Name: application.Name, CreatedBy: owner, CreatedAt: time.Now().UTC()}
		}
		return useCases.CreateOutput{Data: data}
	}

	one, oneErr := client.CreateApplications(context.Background(), &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "app-unica"}}})
	many, manyErr := client.CreateApplications(context.Background(), &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "app-a"}, {Name: "app-b"}}})

	require.NoError(t, oneErr)
	if assert.Len(t, one.GetApplications(), 1) {
		created := one.GetApplications()[0]
		assert.Equal(t, "app-unica", created.GetName())
		assert.Equal(t, owner.String(), created.GetCreatedBy())
		assert.NotNil(t, created.GetCreatedAt())
		assert.Nil(t, created.GetUpdatedAt(), "aplicação nunca alterada não tem updated_at")
	}
	assert.Empty(t, one.GetErrors())
	require.NoError(t, manyErr)
	if assert.Len(t, many.GetApplications(), 2, "a ordem do lote é preservada") {
		assert.Equal(t, "app-a", many.GetApplications()[0].GetName())
		assert.Equal(t, "app-b", many.GetApplications()[1].GetName())
	}
	if assert.Len(t, f.create.Inputs, 2) {
		assert.Equal(t, uuid.UUID{}, f.create.Inputs[0].Application[0].CreatedBy, "o pedido não tem como definir o dono")
	}
}

func whenBatchIsPartial(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	ok := mocks.NewApp()
	f.update.PerformFunc = func(context.Context, useCases.UpdateInput) useCases.UpdateOutput {
		return useCases.UpdateOutput{Data: []entities.Application{ok}, Errors: []entities.Error{entities.NotFoundError("Resource")}}
	}

	response, err := client.UpdateApplications(context.Background(), &vaultv1.UpdateApplicationsRequest{Applications: []*vaultv1.ApplicationChange{
		{Id: ok.ID.String(), Name: "nova"}, {Id: uuid.NewV7().String(), Name: "outra"},
	}})

	require.NoError(t, err, "com ao menos um sucesso a chamada é OK, como o 207 do HTTP")
	if assert.Len(t, response.GetApplications(), 1) {
		assert.Equal(t, ok.ID.String(), response.GetApplications()[0].GetId())
	}
	if assert.Len(t, response.GetErrors(), 1) {
		assert.Equal(t, int32(http.StatusNotFound), response.GetErrors()[0].GetStatus())
		assert.Equal(t, entities.ErrTypeNotFound, response.GetErrors()[0].GetType())
	}
}

func whenBatchFailsEntirely(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	conflict := entities.ConflictError("Application already exists", "The name is already in use.")
	forbidden := entities.ForbiddenError()
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Errors: []entities.Error{conflict, forbidden}}
	}
	f.update.PerformFunc = func(context.Context, useCases.UpdateInput) useCases.UpdateOutput {
		return useCases.UpdateOutput{Errors: []entities.Error{forbidden}}
	}
	f.delete.PerformFunc = func(context.Context, useCases.DeleteInput) useCases.DeleteOutput {
		return useCases.DeleteOutput{Errors: []entities.Error{entities.NotFoundError("Resource")}}
	}
	id := uuid.NewV7().String()

	created, createErr := client.CreateApplications(context.Background(), &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "a-app"}, {Name: "b-app"}}})
	_, updateErr := client.UpdateApplications(context.Background(), &vaultv1.UpdateApplicationsRequest{Applications: []*vaultv1.ApplicationChange{{Id: id, Name: "nova"}}})
	_, deleteErr := client.DeleteApplications(context.Background(), &vaultv1.DeleteApplicationsRequest{Ids: []string{id}})

	assert.Nil(t, created)
	assert.Equal(t, codes.AlreadyExists, status.Code(createErr), "sem nenhum sucesso, vale o status do primeiro erro")
	assert.Equal(t, codes.PermissionDenied, status.Code(updateErr))
	assert.Equal(t, codes.NotFound, status.Code(deleteErr))
}

func whenBatchIsEmpty(t *testing.T) {
	client, f := newClient(t, entities.Actor{})

	_, createErr := client.CreateApplications(context.Background(), &vaultv1.CreateApplicationsRequest{})
	_, updateErr := client.UpdateApplications(context.Background(), &vaultv1.UpdateApplicationsRequest{})
	_, deleteErr := client.DeleteApplications(context.Background(), &vaultv1.DeleteApplicationsRequest{})

	for _, err := range []error{createErr, updateErr, deleteErr} {
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	assert.Empty(t, f.create.Inputs, "lote vazio não chega ao use case")
	assert.Empty(t, f.update.Inputs)
	assert.Empty(t, f.delete.Inputs)
}

func whenIDIsInvalid(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	valid := uuid.NewV7().String()

	for _, raw := range []string{"", "nao-e-uuid", "' OR 1=1 --"} {
		_, getErr := client.GetApplication(context.Background(), &vaultv1.GetApplicationRequest{Id: raw})
		_, updateErr := client.UpdateApplications(context.Background(), &vaultv1.UpdateApplicationsRequest{Applications: []*vaultv1.ApplicationChange{{Id: valid, Name: "boa"}, {Id: raw, Name: "ruim"}}})
		_, deleteErr := client.DeleteApplications(context.Background(), &vaultv1.DeleteApplicationsRequest{Ids: []string{valid, raw}})

		for _, err := range []error{getErr, updateErr, deleteErr} {
			assert.Equal(t, codes.InvalidArgument, status.Code(err), raw)
		}
	}
	assert.Empty(t, f.get.Inputs, "id inválido não chega ao use case")
	assert.Empty(t, f.update.Inputs, "um id inválido recusa o lote inteiro, como o corpo inválido no HTTP")
	assert.Empty(t, f.delete.Inputs)
}

func whenGetSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app := mocks.NewApp()
	updatedAt := time.Now().UTC().Truncate(time.Second)
	app.UpdatedAt = &updatedAt
	f.get.PerformFunc = func(context.Context, useCases.GetByIDInput) useCases.GetByIDOutput {
		return useCases.GetByIDOutput{Data: app}
	}

	response, err := client.GetApplication(context.Background(), &vaultv1.GetApplicationRequest{Id: app.ID.String()})

	require.NoError(t, err)
	assert.Equal(t, app.ID.String(), response.GetId())
	assert.Equal(t, app.Name, response.GetName())
	assert.Equal(t, app.CreatedBy.String(), response.GetCreatedBy())
	assert.True(t, app.CreatedAt.Equal(response.GetCreatedAt().AsTime()))
	assert.True(t, updatedAt.Equal(response.GetUpdatedAt().AsTime()))
	if assert.Len(t, f.get.Inputs, 1) {
		assert.Equal(t, app.ID, f.get.Inputs[0].Application)
	}
}

func whenGetFails(t *testing.T) {
	notFound := entities.NotFoundError("Resource")
	internal := entities.Error{Status: http.StatusInternalServerError, Title: "Internal Server Error", Detail: "segredo-interno"}
	for code, failure := range map[codes.Code]entities.Error{codes.NotFound: notFound, codes.Internal: internal} {
		client, f := newClient(t, entities.Actor{})
		f.get.PerformFunc = func(context.Context, useCases.GetByIDInput) useCases.GetByIDOutput {
			return useCases.GetByIDOutput{Error: &failure, Data: mocks.NewApp()}
		}

		response, err := client.GetApplication(context.Background(), &vaultv1.GetApplicationRequest{Id: uuid.NewV7().String()})

		assert.Nil(t, response, "com erro, nenhuma aplicação sai")
		assert.Equal(t, code, status.Code(err))
		assert.NotContains(t, status.Convert(err).Message(), "segredo-interno")
	}
}

func whenListSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	apps := []entities.Application{mocks.NewApp(), mocks.NewApp()}
	f.list.PerformFunc = func(_ context.Context, input useCases.ListInput) useCases.ListOutput {
		paginator := entities.NewPaginator(input.Page, input.Limit, 5)
		return useCases.ListOutput{Data: apps, Page: &paginator}
	}

	response, err := client.ListApplications(context.Background(), &vaultv1.ListApplicationsRequest{Page: 2, Limit: 2})
	_, defaultsErr := client.ListApplications(context.Background(), &vaultv1.ListApplicationsRequest{})

	require.NoError(t, err)
	if assert.Len(t, response.GetApplications(), 2) {
		assert.Equal(t, apps[0].ID.String(), response.GetApplications()[0].GetId())
		assert.Equal(t, apps[1].ID.String(), response.GetApplications()[1].GetId())
	}
	assert.Equal(t, int32(2), response.GetPage().GetPage())
	assert.Equal(t, int32(2), response.GetPage().GetLimit())
	assert.Equal(t, int32(5), response.GetPage().GetTotal())
	assert.True(t, response.GetPage().GetHasNext())
	assert.True(t, response.GetPage().GetHasPrev())
	require.NoError(t, defaultsErr)
	if assert.Len(t, f.list.Inputs, 2) {
		assert.Equal(t, 2, f.list.Inputs[0].Page)
		assert.Equal(t, 2, f.list.Inputs[0].Limit)
		assert.Zero(t, f.list.Inputs[1].Page, "sem page e limit, o use case aplica os padrões")
		assert.Zero(t, f.list.Inputs[1].Limit)
	}
}

func whenListFails(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	unauthorized := entities.UnauthorizedError()
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &unauthorized, Data: []entities.Application{mocks.NewApp()}}
	}

	response, err := client.ListApplications(context.Background(), &vaultv1.ListApplicationsRequest{})

	assert.Nil(t, response)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func whenUpdateHasDuplicatedIDs(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	repeated, single := uuid.NewV7(), uuid.NewV7()

	_, err := client.UpdateApplications(context.Background(), &vaultv1.UpdateApplicationsRequest{Applications: []*vaultv1.ApplicationChange{
		{Id: repeated.String(), Name: "primeira"}, {Id: single.String(), Name: "unica"}, {Id: repeated.String(), Name: "segunda"},
	}})

	require.NoError(t, err)
	if assert.Len(t, f.update.Inputs, 1) {
		input := f.update.Inputs[0]
		assert.Len(t, input.Applications, 3)
		assert.Equal(t, "primeira", input.Applications[0].Name)
		assert.Equal(t, map[uuid.UUID]bool{repeated: true}, input.DuplicatedIDs)
	}
}

func whenDeleteSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	first, second, repeated := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.delete.PerformFunc = func(context.Context, useCases.DeleteInput) useCases.DeleteOutput {
		return useCases.DeleteOutput{Data: []uuid.UUID{first, second}, Errors: []entities.Error{entities.InvalidApplicationIDError(repeated)}}
	}

	response, err := client.DeleteApplications(context.Background(), &vaultv1.DeleteApplicationsRequest{Ids: []string{first.String(), second.String(), repeated.String(), repeated.String()}})

	require.NoError(t, err)
	assert.Equal(t, []string{first.String(), second.String()}, response.GetIds())
	assert.Len(t, response.GetErrors(), 1)
	if assert.Len(t, f.delete.Inputs, 1) {
		assert.Equal(t, []uuid.UUID{first, second, repeated, repeated}, f.delete.Inputs[0].IDs)
		assert.Equal(t, map[uuid.UUID]bool{repeated: true}, f.delete.Inputs[0].DuplicatedIDs)
	}
}

func whenItemErrorIsInternal(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{
			Data:   []entities.Application{mocks.NewApp()},
			Errors: []entities.Error{{Status: http.StatusInternalServerError, Type: entities.ErrTypeInternal, Title: "Internal Server Error", Detail: "segredo-interno"}},
		}
	}

	response, err := client.CreateApplications(context.Background(), &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "a-app"}, {Name: "b-app"}}})

	require.NoError(t, err)
	if assert.Len(t, response.GetErrors(), 1) {
		assert.Equal(t, int32(http.StatusInternalServerError), response.GetErrors()[0].GetStatus())
		assert.Equal(t, entities.ErrTypeInternal, response.GetErrors()[0].GetType())
		assert.Empty(t, response.GetErrors()[0].GetDetail(), "o detalhe do erro interno não sai nem dentro do lote")
	}
}

func whenRequesterIsExtracted(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7(), SessionID: "sessao", PlatformAdmin: true}
	client, f := newClient(t, actor)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")
	id := uuid.NewV7().String()

	_, createErr := client.CreateApplications(ctx, &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "a-app"}}})
	_, getErr := client.GetApplication(ctx, &vaultv1.GetApplicationRequest{Id: id})
	_, listErr := client.ListApplications(ctx, &vaultv1.ListApplicationsRequest{})
	_, updateErr := client.UpdateApplications(ctx, &vaultv1.UpdateApplicationsRequest{Applications: []*vaultv1.ApplicationChange{{Id: id, Name: "nova"}}})
	_, deleteErr := client.DeleteApplications(ctx, &vaultv1.DeleteApplicationsRequest{Ids: []string{id}})

	for _, err := range []error{createErr, getErr, listErr, updateErr, deleteErr} {
		require.NoError(t, err)
	}
	requesters := []entities.ApiHeader{f.create.Inputs[0].Requester, f.get.Inputs[0].Requester, f.list.Inputs[0].Requester, f.update.Inputs[0].Requester, f.delete.Inputs[0].Requester}
	for _, requester := range requesters {
		assert.Equal(t, actor, requester.Actor, "o ator vem do ctx posto pelo interceptor, nunca do pedido")
		assert.Equal(t, "req-123", requester.RequestID)
		assert.Equal(t, "grpc", requester.UserAgent)
		assert.Equal(t, "127.0.0.1", requester.ClientIP)
	}
}

func whenCallsAreConcurrent(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.get.PerformFunc = func(_ context.Context, input useCases.GetByIDInput) useCases.GetByIDOutput {
		return useCases.GetByIDOutput{Data: entities.Application{ID: input.Application}}
	}
	const calls = 20
	asked, answered := make([]string, calls), make([]string, calls)

	var wg sync.WaitGroup
	for i := range calls {
		asked[i] = uuid.NewV7().String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.GetApplication(context.Background(), &vaultv1.GetApplicationRequest{Id: asked[i]})
			if assert.NoError(t, err) {
				answered[i] = response.GetId()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, asked, answered, "cada chamada recebe a própria resposta")
	assert.Len(t, f.get.Inputs, calls)
}

// Helpers

type fakes struct {
	create *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	get    *pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]
	list   *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	update *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	delete *pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]
}

func newFakes() fakes {
	return fakes{
		create: &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		get:    &pkgmocks.UseCase[useCases.GetByIDInput, useCases.GetByIDOutput]{},
		list:   &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		update: &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		delete: &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
	}
}

func (f fakes) input(registrar grpc.ServiceRegistrar) grpc_controllers.RegisterInput {
	return grpc_controllers.RegisterInput{
		Registrar:      registrar,
		CreateUseCase:  f.create,
		GetByIDUseCase: f.get,
		ListUseCase:    f.list,
		UpdateUseCase:  f.update,
		DeleteUseCase:  f.delete,
	}
}

func newClient(t *testing.T, actor entities.Actor) (vaultv1.ApplicationServiceClient, fakes) {
	t.Helper()

	f := newFakes()
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(f.input(registrar))
	})

	return vaultv1.NewApplicationServiceClient(conn), f
}
