package grpc_controllers_test

import (
	"context"
	"github.com/murkio-dev/vault/internal/restful/member/grpc_controllers"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
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
	t.Run("Quando lista os membros.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando a paginação é inválida.", func(t *testing.T) { whenPaginationIsInvalid(t) })
	t.Run("Quando busca o próprio vínculo.", func(t *testing.T) { whenGetMeSucceeds(t) })
	t.Run("Quando muda o papel e o status.", func(t *testing.T) { whenUpdateSucceeds(t) })
	t.Run("Quando remove o membro.", func(t *testing.T) { whenRemoveSucceeds(t) })
	t.Run("Quando um id não é uuid.", func(t *testing.T) { whenIDIsInvalid(t) })
	t.Run("Quando o usecase devolve erro.", func(t *testing.T) { whenUseCaseReturnsErrors(t) })
	t.Run("Quando o ator e o request id chegam no Requester.", func(t *testing.T) { whenRequesterIsExtracted(t) })
	t.Run("Quando há chamadas concorrentes.", func(t *testing.T) { whenCallsAreConcurrent(t) })
}

// Functions

func whenRegisterIsSuccessful(t *testing.T) {
	assert.NoError(t, grpc_controllers.RegisterControllers(newFakes().input(grpc.NewServer())))
}

func whenRegisterFailsDueToNilParams(t *testing.T) {
	withoutUpdate := newFakes().input(grpc.NewServer())
	withoutUpdate.UpdateUseCase = nil

	assert.Error(t, grpc_controllers.RegisterControllers(newFakes().input(nil)))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutUpdate))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenListSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app := uuid.NewV7()
	inviter := uuid.NewV7()
	updatedAt := time.Now().UTC().Truncate(time.Second)
	admin, developer := mocks.NewMember(entities.RoleAdmin), mocks.NewMember(entities.RoleDeveloper)
	developer.InvitedBy, developer.UpdatedAt = &inviter, &updatedAt
	f.list.PerformFunc = func(_ context.Context, input useCases.ListInput) useCases.ListOutput {
		paginator := entities.NewPaginator(input.Page, input.Limit, 3)
		return useCases.ListOutput{Data: []entities.Member{admin, developer}, Page: &paginator}
	}

	response, err := client.ListMembers(context.Background(), &vaultv1.ListMembersRequest{ApplicationId: app.String(), Page: 1, Limit: 2})
	_, defaultsErr := client.ListMembers(context.Background(), &vaultv1.ListMembersRequest{ApplicationId: app.String()})

	require.NoError(t, err)
	if assert.Len(t, response.GetMembers(), 2, "a ordem do use case é preservada") {
		first, second := response.GetMembers()[0], response.GetMembers()[1]
		assert.Equal(t, admin.ID.String(), first.GetId())
		assert.Equal(t, admin.UserID.String(), first.GetUserId())
		assert.Equal(t, admin.UriID, first.GetUriId())
		assert.Equal(t, string(entities.RoleAdmin), first.GetRole())
		assert.Equal(t, string(admin.Status), first.GetStatus())
		assert.Empty(t, first.GetInvitedBy(), "quem criou a aplicação não tem invited_by")
		assert.Nil(t, first.GetUpdatedAt())
		assert.Equal(t, inviter.String(), second.GetInvitedBy())
		assert.True(t, updatedAt.Equal(second.GetUpdatedAt().AsTime()))
	}
	assert.Equal(t, int32(3), response.GetPage().GetTotal())
	assert.True(t, response.GetPage().GetHasNext())
	require.NoError(t, defaultsErr)
	if assert.Len(t, f.list.Inputs, 2) {
		assert.Equal(t, app, f.list.Inputs[0].Application)
		assert.Equal(t, 1, f.list.Inputs[0].Page)
		assert.Equal(t, 2, f.list.Inputs[0].Limit)
		assert.Equal(t, 1, f.list.Inputs[1].Page, "sem page, vale a primeira")
		assert.Equal(t, entities.MaxPageSize, f.list.Inputs[1].Limit, "sem limit, vale o máximo")
	}
}

func whenPaginationIsInvalid(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app := uuid.NewV7().String()

	_, negativePage := client.ListMembers(context.Background(), &vaultv1.ListMembersRequest{ApplicationId: app, Page: -1})
	_, negativeLimit := client.ListMembers(context.Background(), &vaultv1.ListMembersRequest{ApplicationId: app, Limit: -5})

	assert.Equal(t, codes.InvalidArgument, status.Code(negativePage))
	assert.Equal(t, codes.InvalidArgument, status.Code(negativeLimit))
	assert.Empty(t, f.list.Inputs)
}

func whenGetMeSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app := uuid.NewV7()
	me := mocks.NewMember(entities.RoleDeveloper)
	f.getMe.PerformFunc = func(context.Context, useCases.GetMeInput) useCases.GetMeOutput {
		return useCases.GetMeOutput{Data: me}
	}

	response, err := client.GetMe(context.Background(), &vaultv1.GetMeRequest{ApplicationId: app.String()})

	require.NoError(t, err)
	assert.Equal(t, me.ID.String(), response.GetId())
	assert.Equal(t, string(entities.RoleDeveloper), response.GetRole())
	if assert.Len(t, f.getMe.Inputs, 1) {
		assert.Equal(t, app, f.getMe.Inputs[0].Application)
	}
}

func whenUpdateSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member := uuid.NewV7(), uuid.NewV7()
	role, memberStatus := "admin", "suspended"
	f.update.PerformFunc = func(_ context.Context, input useCases.UpdateInput) useCases.UpdateOutput {
		updated := mocks.NewMember(entities.RoleDeveloper)
		updated.ID = input.Member
		return useCases.UpdateOutput{Data: updated}
	}

	both, bothErr := client.UpdateMember(context.Background(), &vaultv1.UpdateMemberRequest{ApplicationId: app.String(), MemberId: member.String(), Role: &role, Status: &memberStatus})
	_, onlyRoleErr := client.UpdateMember(context.Background(), &vaultv1.UpdateMemberRequest{ApplicationId: app.String(), MemberId: member.String(), Role: &role})
	_, noneErr := client.UpdateMember(context.Background(), &vaultv1.UpdateMemberRequest{ApplicationId: app.String(), MemberId: member.String()})

	require.NoError(t, bothErr)
	assert.Equal(t, member.String(), both.GetId())
	require.NoError(t, onlyRoleErr)
	require.NoError(t, noneErr)
	if assert.Len(t, f.update.Inputs, 3) {
		first, second, third := f.update.Inputs[0], f.update.Inputs[1], f.update.Inputs[2]
		assert.Equal(t, app, first.Application)
		assert.Equal(t, member, first.Member)
		if assert.NotNil(t, first.Role) && assert.NotNil(t, first.Status) {
			assert.Equal(t, entities.RoleAdmin, *first.Role)
			assert.Equal(t, entities.MemberStatus("suspended"), *first.Status)
		}
		assert.NotNil(t, second.Role)
		assert.Nil(t, second.Status, "campo ausente não muda")
		assert.Nil(t, third.Role)
		assert.Nil(t, third.Status)
	}
}

func whenRemoveSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member := uuid.NewV7(), uuid.NewV7()

	response, err := client.RemoveMember(context.Background(), &vaultv1.RemoveMemberRequest{ApplicationId: app.String(), MemberId: member.String()})

	require.NoError(t, err)
	assert.NotNil(t, response)
	if assert.Len(t, f.remove.Inputs, 1) {
		assert.Equal(t, app, f.remove.Inputs[0].Application)
		assert.Equal(t, member, f.remove.Inputs[0].Member)
	}
}

func whenIDIsInvalid(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	valid := uuid.NewV7().String()

	for _, raw := range []string{"", "nao-e-uuid", "me", uuid.UUID{}.String(), "' OR 1=1 --"} {
		_, listErr := client.ListMembers(context.Background(), &vaultv1.ListMembersRequest{ApplicationId: raw})
		_, getMeErr := client.GetMe(context.Background(), &vaultv1.GetMeRequest{ApplicationId: raw})
		_, updateAppErr := client.UpdateMember(context.Background(), &vaultv1.UpdateMemberRequest{ApplicationId: raw, MemberId: valid})
		_, updateMemberErr := client.UpdateMember(context.Background(), &vaultv1.UpdateMemberRequest{ApplicationId: valid, MemberId: raw})
		_, removeAppErr := client.RemoveMember(context.Background(), &vaultv1.RemoveMemberRequest{ApplicationId: raw, MemberId: valid})
		_, removeMemberErr := client.RemoveMember(context.Background(), &vaultv1.RemoveMemberRequest{ApplicationId: valid, MemberId: raw})

		for _, err := range []error{listErr, getMeErr, updateAppErr, updateMemberErr, removeAppErr, removeMemberErr} {
			assert.Equal(t, codes.InvalidArgument, status.Code(err), raw)
			assert.NotContains(t, status.Convert(err).Message(), "OR 1=1", "o erro não repete o valor recebido")
		}
	}
	assert.Empty(t, f.list.Inputs, "id inválido não chega ao use case")
	assert.Empty(t, f.getMe.Inputs)
	assert.Empty(t, f.update.Inputs, "mudar o papel de um membro não aceita \"me\"")
	assert.Empty(t, f.remove.Inputs)
}

func whenUseCaseReturnsErrors(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	forbidden := entities.ForbiddenError()
	notFound := entities.NotFoundError("Resource")
	conflict := entities.ConflictError("Last admin", "The application needs an active admin.")
	internal := entities.Error{Status: http.StatusInternalServerError, Title: "Internal Server Error", Detail: "segredo-interno"}
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &forbidden, Data: []entities.Member{mocks.NewMember(entities.RoleAdmin)}}
	}
	f.getMe.PerformFunc = func(context.Context, useCases.GetMeInput) useCases.GetMeOutput {
		return useCases.GetMeOutput{Error: &notFound}
	}
	f.update.PerformFunc = func(context.Context, useCases.UpdateInput) useCases.UpdateOutput {
		return useCases.UpdateOutput{Error: &conflict}
	}
	f.remove.PerformFunc = func(context.Context, useCases.RemoveInput) useCases.RemoveOutput {
		return useCases.RemoveOutput{Error: &internal}
	}
	app, member := uuid.NewV7().String(), uuid.NewV7().String()

	listed, listErr := client.ListMembers(context.Background(), &vaultv1.ListMembersRequest{ApplicationId: app})
	_, getMeErr := client.GetMe(context.Background(), &vaultv1.GetMeRequest{ApplicationId: app})
	_, updateErr := client.UpdateMember(context.Background(), &vaultv1.UpdateMemberRequest{ApplicationId: app, MemberId: member})
	_, removeErr := client.RemoveMember(context.Background(), &vaultv1.RemoveMemberRequest{ApplicationId: app, MemberId: member})

	assert.Nil(t, listed, "com erro, nenhum membro sai")
	assert.Equal(t, codes.PermissionDenied, status.Code(listErr))
	assert.Equal(t, codes.NotFound, status.Code(getMeErr))
	assert.Equal(t, codes.AlreadyExists, status.Code(updateErr))
	assert.Equal(t, codes.Internal, status.Code(removeErr))
	assert.NotContains(t, status.Convert(removeErr).Message(), "segredo-interno")
}

func whenRequesterIsExtracted(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7(), SessionID: "sessao"}
	client, f := newClient(t, actor)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")
	app, member := uuid.NewV7().String(), uuid.NewV7().String()

	_, listErr := client.ListMembers(ctx, &vaultv1.ListMembersRequest{ApplicationId: app})
	_, getMeErr := client.GetMe(ctx, &vaultv1.GetMeRequest{ApplicationId: app})
	_, updateErr := client.UpdateMember(ctx, &vaultv1.UpdateMemberRequest{ApplicationId: app, MemberId: member})
	_, removeErr := client.RemoveMember(ctx, &vaultv1.RemoveMemberRequest{ApplicationId: app, MemberId: member})

	for _, err := range []error{listErr, getMeErr, updateErr, removeErr} {
		require.NoError(t, err)
	}
	for _, requester := range []entities.ApiHeader{f.list.Inputs[0].Requester, f.getMe.Inputs[0].Requester, f.update.Inputs[0].Requester, f.remove.Inputs[0].Requester} {
		assert.Equal(t, actor, requester.Actor, "o ator vem do ctx posto pelo interceptor, nunca do pedido")
		assert.Equal(t, "req-123", requester.RequestID)
		assert.Equal(t, "grpc", requester.UserAgent)
	}
}

func whenCallsAreConcurrent(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.update.PerformFunc = func(_ context.Context, input useCases.UpdateInput) useCases.UpdateOutput {
		return useCases.UpdateOutput{Data: entities.Member{ID: input.Member}}
	}
	const calls = 20
	app := uuid.NewV7().String()
	asked, answered := make([]string, calls), make([]string, calls)

	var wg sync.WaitGroup
	for i := range calls {
		asked[i] = uuid.NewV7().String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.UpdateMember(context.Background(), &vaultv1.UpdateMemberRequest{ApplicationId: app, MemberId: asked[i]})
			if assert.NoError(t, err) {
				answered[i] = response.GetId()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, asked, answered, "cada chamada recebe a própria resposta")
	assert.Len(t, f.update.Inputs, calls)
}

// Helpers

type fakes struct {
	list   *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	getMe  *pkgmocks.UseCase[useCases.GetMeInput, useCases.GetMeOutput]
	update *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	remove *pkgmocks.UseCase[useCases.RemoveInput, useCases.RemoveOutput]
}

func newFakes() fakes {
	return fakes{
		list:   &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		getMe:  &pkgmocks.UseCase[useCases.GetMeInput, useCases.GetMeOutput]{},
		update: &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		remove: &pkgmocks.UseCase[useCases.RemoveInput, useCases.RemoveOutput]{},
	}
}

func (f fakes) input(registrar grpc.ServiceRegistrar) grpc_controllers.RegisterInput {
	return grpc_controllers.RegisterInput{Registrar: registrar, ListUseCase: f.list, GetMeUseCase: f.getMe, UpdateUseCase: f.update, RemoveUseCase: f.remove}
}

func newClient(t *testing.T, actor entities.Actor) (vaultv1.MemberServiceClient, fakes) {
	t.Helper()

	f := newFakes()
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(f.input(registrar))
	})

	return vaultv1.NewMemberServiceClient(conn), f
}
