package grpc_controllers_test

import (
	"context"
	"github.com/murkio-dev/vault/internal/restful/token/grpc_controllers"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/token/mocks"
	"github.com/murkio-dev/vault/internal/restful/token/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando cria um token.", func(t *testing.T) { whenCreateSucceeds(t) })
	t.Run("Quando lista os tokens.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando revoga um token.", func(t *testing.T) { whenRevokeSucceeds(t) })
	t.Run("Quando o membro é me.", func(t *testing.T) { whenMemberIsMe(t) })
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
	withoutRevoke := newFakes().input(grpc.NewServer())
	withoutRevoke.RevokeUseCase = nil

	assert.Error(t, grpc_controllers.RegisterControllers(newFakes().input(nil)))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutRevoke))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenCreateSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member := uuid.NewV7(), uuid.NewV7()
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	token := mocks.NewToken()
	token.TokenHash = "hash-que-nao-pode-sair"
	token.ExpiresAt = &expiresAt
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: entities.AccessTokenCreated{AccessToken: token, Token: "sk_vt_token-novo", URI: "https://vault.example.com/api/v1/runtime/uri/env"}}
	}

	withExpiry, err := client.CreateToken(context.Background(), &vaultv1.CreateTokenRequest{ApplicationId: app.String(), MemberId: member.String(), Name: "ci", ExpiresAt: timestamppb.New(expiresAt)})
	_, withoutExpiryErr := client.CreateToken(context.Background(), &vaultv1.CreateTokenRequest{ApplicationId: app.String(), MemberId: member.String(), Name: "ci"})

	require.NoError(t, err)
	assert.Equal(t, "sk_vt_token-novo", withExpiry.GetToken(), "o token sai só nesta resposta")
	assert.Equal(t, "https://vault.example.com/api/v1/runtime/uri/env", withExpiry.GetUri())
	assert.Equal(t, token.ID.String(), withExpiry.GetAccessToken().GetId())
	assert.Equal(t, token.Name, withExpiry.GetAccessToken().GetName())
	assert.Equal(t, token.Prefix, withExpiry.GetAccessToken().GetPrefix())
	assert.True(t, expiresAt.Equal(withExpiry.GetAccessToken().GetExpiresAt().AsTime()))
	assert.Nil(t, withExpiry.GetAccessToken().GetRevokedAt())
	assert.NotContains(t, withExpiry.String(), "hash-que-nao-pode-sair", "o hash do token nunca sai")
	require.NoError(t, withoutExpiryErr)
	if assert.Len(t, f.create.Inputs, 2) {
		assert.Equal(t, app, f.create.Inputs[0].Application)
		assert.Equal(t, member, f.create.Inputs[0].Member)
		assert.Equal(t, "ci", f.create.Inputs[0].Name)
		if assert.NotNil(t, f.create.Inputs[0].ExpiresAt) {
			assert.True(t, expiresAt.Equal(*f.create.Inputs[0].ExpiresAt))
		}
		assert.Nil(t, f.create.Inputs[1].ExpiresAt, "sem expires_at, o token não vence")
	}
}

func whenListSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member := uuid.NewV7(), uuid.NewV7()
	active, revoked := mocks.NewToken(), mocks.NewToken()
	revokedAt := time.Now().UTC().Truncate(time.Second)
	revoked.RevokedAt = &revokedAt
	revoked.TokenHash = "hash-que-nao-pode-sair"
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Data: []entities.AccessToken{active, revoked}}
	}

	response, err := client.ListTokens(context.Background(), &vaultv1.ListTokensRequest{ApplicationId: app.String(), MemberId: member.String()})

	require.NoError(t, err)
	if assert.Len(t, response.GetAccessTokens(), 2, "a ordem do use case é preservada") {
		assert.Equal(t, active.ID.String(), response.GetAccessTokens()[0].GetId())
		assert.Nil(t, response.GetAccessTokens()[0].GetRevokedAt())
		assert.True(t, revokedAt.Equal(response.GetAccessTokens()[1].GetRevokedAt().AsTime()))
	}
	assert.NotContains(t, response.String(), "hash-que-nao-pode-sair")
	if assert.Len(t, f.list.Inputs, 1) {
		assert.Equal(t, app, f.list.Inputs[0].Application)
		assert.Equal(t, member, f.list.Inputs[0].Member)
	}
}

func whenRevokeSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member, token := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()

	response, err := client.RevokeToken(context.Background(), &vaultv1.RevokeTokenRequest{ApplicationId: app.String(), MemberId: member.String(), TokenId: token.String()})

	require.NoError(t, err)
	assert.NotNil(t, response)
	if assert.Len(t, f.revoke.Inputs, 1) {
		assert.Equal(t, app, f.revoke.Inputs[0].Application)
		assert.Equal(t, member, f.revoke.Inputs[0].Member)
		assert.Equal(t, token, f.revoke.Inputs[0].Token)
	}
}

func whenMemberIsMe(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, token := uuid.NewV7().String(), uuid.NewV7().String()

	_, createErr := client.CreateToken(context.Background(), &vaultv1.CreateTokenRequest{ApplicationId: app, MemberId: "me", Name: "ci"})
	_, listErr := client.ListTokens(context.Background(), &vaultv1.ListTokensRequest{ApplicationId: app, MemberId: "me"})
	_, revokeErr := client.RevokeToken(context.Background(), &vaultv1.RevokeTokenRequest{ApplicationId: app, MemberId: "me", TokenId: token})

	for _, err := range []error{createErr, listErr, revokeErr} {
		require.NoError(t, err)
	}
	assert.Equal(t, entities.SelfOwner, f.create.Inputs[0].Member, "\"me\" viaja como o marcador até a autorização")
	assert.Equal(t, entities.SelfOwner, f.list.Inputs[0].Member)
	assert.Equal(t, entities.SelfOwner, f.revoke.Inputs[0].Member)
}

func whenIDIsInvalid(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	valid := uuid.NewV7().String()

	for _, raw := range []string{"", "nao-e-uuid", "ME", uuid.UUID{}.String(), "' OR 1=1 --"} {
		_, createAppErr := client.CreateToken(context.Background(), &vaultv1.CreateTokenRequest{ApplicationId: raw, MemberId: valid, Name: "ci"})
		_, createMemberErr := client.CreateToken(context.Background(), &vaultv1.CreateTokenRequest{ApplicationId: valid, MemberId: raw, Name: "ci"})
		_, listAppErr := client.ListTokens(context.Background(), &vaultv1.ListTokensRequest{ApplicationId: raw, MemberId: valid})
		_, listMemberErr := client.ListTokens(context.Background(), &vaultv1.ListTokensRequest{ApplicationId: valid, MemberId: raw})
		_, revokeMemberErr := client.RevokeToken(context.Background(), &vaultv1.RevokeTokenRequest{ApplicationId: valid, MemberId: raw, TokenId: valid})
		_, revokeTokenErr := client.RevokeToken(context.Background(), &vaultv1.RevokeTokenRequest{ApplicationId: valid, MemberId: valid, TokenId: raw})

		for _, err := range []error{createAppErr, createMemberErr, listAppErr, listMemberErr, revokeMemberErr, revokeTokenErr} {
			assert.Equal(t, codes.InvalidArgument, status.Code(err), raw)
			assert.NotContains(t, status.Convert(err).Message(), "OR 1=1", "o erro não repete o valor recebido")
		}
	}
	_, revokeMe := client.RevokeToken(context.Background(), &vaultv1.RevokeTokenRequest{ApplicationId: valid, MemberId: valid, TokenId: "me"})
	assert.Equal(t, codes.InvalidArgument, status.Code(revokeMe), "\"me\" só vale no member_id")
	assert.Empty(t, f.create.Inputs, "id inválido não chega ao use case")
	assert.Empty(t, f.list.Inputs)
	assert.Empty(t, f.revoke.Inputs)
}

func whenUseCaseReturnsErrors(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	conflict := entities.ConflictError("Member is suspended", "A suspended member cannot receive a token.")
	forbidden := entities.ForbiddenError()
	internal := entities.Error{Status: http.StatusInternalServerError, Title: "Internal Server Error", Detail: "segredo-interno"}
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Error: &conflict, Data: entities.AccessTokenCreated{Token: "sk_vt_nao-vazar"}}
	}
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &forbidden, Data: []entities.AccessToken{mocks.NewToken()}}
	}
	f.revoke.PerformFunc = func(context.Context, useCases.RevokeInput) useCases.RevokeOutput {
		return useCases.RevokeOutput{Error: &internal}
	}
	id := uuid.NewV7().String()

	created, createErr := client.CreateToken(context.Background(), &vaultv1.CreateTokenRequest{ApplicationId: id, MemberId: id})
	listed, listErr := client.ListTokens(context.Background(), &vaultv1.ListTokensRequest{ApplicationId: id, MemberId: id})
	_, revokeErr := client.RevokeToken(context.Background(), &vaultv1.RevokeTokenRequest{ApplicationId: id, MemberId: id, TokenId: id})

	assert.Nil(t, created, "com erro, nenhum token sai")
	assert.Equal(t, codes.AlreadyExists, status.Code(createErr))
	assert.NotContains(t, status.Convert(createErr).Message(), "sk_vt_nao-vazar")
	assert.Nil(t, listed)
	assert.Equal(t, codes.PermissionDenied, status.Code(listErr))
	assert.Equal(t, codes.Internal, status.Code(revokeErr))
	assert.NotContains(t, status.Convert(revokeErr).Message(), "segredo-interno")
}

func whenRequesterIsExtracted(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7(), SessionID: "sessao"}
	client, f := newClient(t, actor)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")
	id := uuid.NewV7().String()

	_, createErr := client.CreateToken(ctx, &vaultv1.CreateTokenRequest{ApplicationId: id, MemberId: id})
	_, listErr := client.ListTokens(ctx, &vaultv1.ListTokensRequest{ApplicationId: id, MemberId: id})
	_, revokeErr := client.RevokeToken(ctx, &vaultv1.RevokeTokenRequest{ApplicationId: id, MemberId: id, TokenId: id})

	for _, err := range []error{createErr, listErr, revokeErr} {
		require.NoError(t, err)
	}
	for _, requester := range []entities.ApiHeader{f.create.Inputs[0].Requester, f.list.Inputs[0].Requester, f.revoke.Inputs[0].Requester} {
		assert.Equal(t, actor, requester.Actor, "o ator vem do ctx posto pelo interceptor, nunca do pedido")
		assert.Equal(t, "req-123", requester.RequestID)
		assert.Equal(t, "grpc", requester.UserAgent)
	}
}

func whenCallsAreConcurrent(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.create.PerformFunc = func(_ context.Context, input useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: entities.AccessTokenCreated{Token: input.Name}}
	}
	const calls = 20
	id := uuid.NewV7().String()
	asked, answered := make([]string, calls), make([]string, calls)

	var wg sync.WaitGroup
	for i := range calls {
		asked[i] = uuid.NewV7().String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.CreateToken(context.Background(), &vaultv1.CreateTokenRequest{ApplicationId: id, MemberId: id, Name: asked[i]})
			if assert.NoError(t, err) {
				answered[i] = response.GetToken()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, asked, answered, "cada chamada recebe a própria resposta")
	assert.Len(t, f.create.Inputs, calls)
}

// Helpers

type fakes struct {
	create *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list   *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	revoke *pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]
}

func newFakes() fakes {
	return fakes{
		create: &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:   &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		revoke: &pkgmocks.UseCase[useCases.RevokeInput, useCases.RevokeOutput]{},
	}
}

func (f fakes) input(registrar grpc.ServiceRegistrar) grpc_controllers.RegisterInput {
	return grpc_controllers.RegisterInput{Registrar: registrar, CreateUseCase: f.create, ListUseCase: f.list, RevokeUseCase: f.revoke}
}

func newClient(t *testing.T, actor entities.Actor) (vaultv1.TokenServiceClient, fakes) {
	t.Helper()

	f := newFakes()
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(f.input(registrar))
	})

	return vaultv1.NewTokenServiceClient(conn), f
}
