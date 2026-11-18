package grpc_controllers_test

import (
	"context"
	"github.com/murkio-dev/vault/internal/restful/invitation/grpc_controllers"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/invitation/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
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
	t.Run("Quando cria um convite.", func(t *testing.T) { whenCreateSucceeds(t) })
	t.Run("Quando lista os convites.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando revoga um convite.", func(t *testing.T) { whenRevokeSucceeds(t) })
	t.Run("Quando aceita um convite.", func(t *testing.T) { whenAcceptSucceeds(t) })
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
	withoutAccept := newFakes().input(grpc.NewServer())
	withoutAccept.AcceptUseCase = nil

	assert.Error(t, grpc_controllers.RegisterControllers(newFakes().input(nil)))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutAccept))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenCreateSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app := uuid.NewV7()
	invitation := mocks.NewInvitation()
	invitation.TokenHash = "hash-que-nao-pode-sair"
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: entities.InvitationCreated{Invitation: invitation, Token: "vi_token-do-convite"}}
	}

	response, err := client.CreateInvitation(context.Background(), &vaultv1.CreateInvitationRequest{ApplicationId: app.String(), Email: "dev@example.com", Role: "developer"})

	require.NoError(t, err)
	assert.Equal(t, "vi_token-do-convite", response.GetToken(), "o token do convite sai só nesta resposta")
	assert.Equal(t, invitation.ID.String(), response.GetInvitation().GetId())
	assert.Equal(t, invitation.Email, response.GetInvitation().GetEmail())
	assert.Equal(t, string(invitation.Role), response.GetInvitation().GetRole())
	assert.True(t, invitation.ExpiresAt.Equal(response.GetInvitation().GetExpiresAt().AsTime()))
	assert.Empty(t, response.GetInvitation().GetAcceptedBy())
	assert.Nil(t, response.GetInvitation().GetAcceptedAt())
	assert.NotContains(t, response.String(), "hash-que-nao-pode-sair", "o hash do token nunca sai")
	if assert.Len(t, f.create.Inputs, 1) {
		assert.Equal(t, app, f.create.Inputs[0].Application)
		assert.Equal(t, "dev@example.com", f.create.Inputs[0].Email)
		assert.Equal(t, entities.RoleDeveloper, f.create.Inputs[0].Role)
	}
}

func whenListSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app := uuid.NewV7()
	pending, accepted := mocks.NewInvitation(), mocks.NewInvitation()
	acceptedBy, acceptedAt := uuid.NewV7(), time.Now().UTC().Truncate(time.Second)
	accepted.AcceptedBy, accepted.AcceptedAt = &acceptedBy, &acceptedAt
	accepted.TokenHash = "hash-que-nao-pode-sair"
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Data: []entities.InvitationView{
			{Invitation: pending, Status: entities.InvitationPending},
			{Invitation: accepted, Status: entities.InvitationAccepted},
		}}
	}

	response, err := client.ListInvitations(context.Background(), &vaultv1.ListInvitationsRequest{ApplicationId: app.String()})

	require.NoError(t, err)
	if assert.Len(t, response.GetInvitations(), 2, "a ordem do use case é preservada") {
		assert.Equal(t, pending.ID.String(), response.GetInvitations()[0].GetId())
		assert.Equal(t, "pending", response.GetInvitations()[0].GetStatus())
		assert.Equal(t, "accepted", response.GetInvitations()[1].GetStatus())
		assert.Equal(t, acceptedBy.String(), response.GetInvitations()[1].GetAcceptedBy())
		assert.True(t, acceptedAt.Equal(response.GetInvitations()[1].GetAcceptedAt().AsTime()))
	}
	assert.NotContains(t, response.String(), "hash-que-nao-pode-sair")
	if assert.Len(t, f.list.Inputs, 1) {
		assert.Equal(t, app, f.list.Inputs[0].Application)
	}
}

func whenRevokeSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, invitation := uuid.NewV7(), uuid.NewV7()

	response, err := client.RevokeInvitation(context.Background(), &vaultv1.RevokeInvitationRequest{ApplicationId: app.String(), InvitationId: invitation.String()})

	require.NoError(t, err)
	assert.NotNil(t, response)
	if assert.Len(t, f.revoke.Inputs, 1) {
		assert.Equal(t, app, f.revoke.Inputs[0].Application)
		assert.Equal(t, invitation, f.revoke.Inputs[0].Invitation)
	}
}

func whenAcceptSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	member := member_mocks.NewMember(entities.RoleDeveloper)
	f.accept.PerformFunc = func(context.Context, useCases.AcceptInput) useCases.AcceptOutput {
		return useCases.AcceptOutput{Data: member}
	}

	response, err := client.AcceptInvitation(context.Background(), &vaultv1.AcceptInvitationRequest{Token: "vi_token-do-convite"})

	require.NoError(t, err)
	assert.Equal(t, member.ID.String(), response.GetId())
	assert.Equal(t, member.ApplicationID.String(), response.GetApplicationId())
	assert.Equal(t, string(entities.RoleDeveloper), response.GetRole())
	if assert.Len(t, f.accept.Inputs, 1) {
		assert.Equal(t, "vi_token-do-convite", f.accept.Inputs[0].Token)
	}
}

func whenIDIsInvalid(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	valid := uuid.NewV7().String()

	for _, raw := range []string{"", "nao-e-uuid", uuid.UUID{}.String(), "' OR 1=1 --"} {
		_, createErr := client.CreateInvitation(context.Background(), &vaultv1.CreateInvitationRequest{ApplicationId: raw, Email: "dev@example.com", Role: "developer"})
		_, listErr := client.ListInvitations(context.Background(), &vaultv1.ListInvitationsRequest{ApplicationId: raw})
		_, revokeAppErr := client.RevokeInvitation(context.Background(), &vaultv1.RevokeInvitationRequest{ApplicationId: raw, InvitationId: valid})
		_, revokeIDErr := client.RevokeInvitation(context.Background(), &vaultv1.RevokeInvitationRequest{ApplicationId: valid, InvitationId: raw})

		for _, err := range []error{createErr, listErr, revokeAppErr, revokeIDErr} {
			assert.Equal(t, codes.InvalidArgument, status.Code(err), raw)
			assert.NotContains(t, status.Convert(err).Message(), "OR 1=1", "o erro não repete o valor recebido")
		}
	}
	assert.Empty(t, f.create.Inputs, "id inválido não chega ao use case")
	assert.Empty(t, f.list.Inputs)
	assert.Empty(t, f.revoke.Inputs)
}

func whenUseCaseReturnsErrors(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	forbidden := entities.ForbiddenError()
	notFound := entities.NotFoundError("Resource")
	invalid := entities.ValidationError("The field email is invalid.")
	internal := entities.Error{Status: http.StatusInternalServerError, Title: "Internal Server Error", Detail: "segredo-interno"}
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Error: &invalid, Data: entities.InvitationCreated{Token: "vi_nao-vazar"}}
	}
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &forbidden}
	}
	f.revoke.PerformFunc = func(context.Context, useCases.RevokeInput) useCases.RevokeOutput {
		return useCases.RevokeOutput{Error: &internal}
	}
	f.accept.PerformFunc = func(context.Context, useCases.AcceptInput) useCases.AcceptOutput {
		return useCases.AcceptOutput{Error: &notFound}
	}
	app, id := uuid.NewV7().String(), uuid.NewV7().String()

	created, createErr := client.CreateInvitation(context.Background(), &vaultv1.CreateInvitationRequest{ApplicationId: app})
	_, listErr := client.ListInvitations(context.Background(), &vaultv1.ListInvitationsRequest{ApplicationId: app})
	_, revokeErr := client.RevokeInvitation(context.Background(), &vaultv1.RevokeInvitationRequest{ApplicationId: app, InvitationId: id})
	_, acceptErr := client.AcceptInvitation(context.Background(), &vaultv1.AcceptInvitationRequest{Token: "vi_errado"})

	assert.Nil(t, created, "com erro, nenhum token sai")
	assert.Equal(t, codes.InvalidArgument, status.Code(createErr))
	assert.Equal(t, codes.PermissionDenied, status.Code(listErr))
	assert.Equal(t, codes.Internal, status.Code(revokeErr))
	assert.NotContains(t, status.Convert(revokeErr).Message(), "segredo-interno")
	assert.Equal(t, codes.NotFound, status.Code(acceptErr))
	assert.NotContains(t, status.Convert(acceptErr).Message(), "vi_errado", "o erro não repete o token recebido")
}

func whenRequesterIsExtracted(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7(), SessionID: "sessao"}
	client, f := newClient(t, actor)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")
	app, id := uuid.NewV7().String(), uuid.NewV7().String()

	_, createErr := client.CreateInvitation(ctx, &vaultv1.CreateInvitationRequest{ApplicationId: app})
	_, listErr := client.ListInvitations(ctx, &vaultv1.ListInvitationsRequest{ApplicationId: app})
	_, revokeErr := client.RevokeInvitation(ctx, &vaultv1.RevokeInvitationRequest{ApplicationId: app, InvitationId: id})
	_, acceptErr := client.AcceptInvitation(ctx, &vaultv1.AcceptInvitationRequest{Token: "vi_token"})

	for _, err := range []error{createErr, listErr, revokeErr, acceptErr} {
		require.NoError(t, err)
	}
	for _, requester := range []entities.ApiHeader{f.create.Inputs[0].Requester, f.list.Inputs[0].Requester, f.revoke.Inputs[0].Requester, f.accept.Inputs[0].Requester} {
		assert.Equal(t, actor, requester.Actor, "o ator vem do ctx posto pelo interceptor, nunca do pedido")
		assert.Equal(t, "req-123", requester.RequestID)
		assert.Equal(t, "grpc", requester.UserAgent)
	}
}

func whenCallsAreConcurrent(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.accept.PerformFunc = func(_ context.Context, input useCases.AcceptInput) useCases.AcceptOutput {
		return useCases.AcceptOutput{Data: entities.Member{UriID: input.Token}}
	}
	const calls = 20
	asked, answered := make([]string, calls), make([]string, calls)

	var wg sync.WaitGroup
	for i := range calls {
		asked[i] = "vi_" + uuid.NewV7().String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.AcceptInvitation(context.Background(), &vaultv1.AcceptInvitationRequest{Token: asked[i]})
			if assert.NoError(t, err) {
				answered[i] = response.GetUriId()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, asked, answered, "cada chamada recebe a própria resposta")
	assert.Len(t, f.accept.Inputs, calls)
}

// Helpers

type fakes struct {
	create *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list   *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	revoke *pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]
	accept *pkgmocks.UseCase[useCases.AcceptInput, useCases.AcceptOutput]
}

func newFakes() fakes {
	return fakes{
		create: &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:   &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		revoke: &pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]{},
		accept: &pkgmocks.UseCase[useCases.AcceptInput, useCases.AcceptOutput]{},
	}
}

func (f fakes) input(registrar grpc.ServiceRegistrar) grpc_controllers.RegisterInput {
	return grpc_controllers.RegisterInput{Registrar: registrar, CreateUseCase: f.create, ListUseCase: f.list, RevokeUseCase: f.revoke, AcceptUseCase: f.accept}
}

func newClient(t *testing.T, actor entities.Actor) (vaultv1.InvitationServiceClient, fakes) {
	t.Helper()

	f := newFakes()
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(f.input(registrar))
	})

	return vaultv1.NewInvitationServiceClient(conn), f
}
