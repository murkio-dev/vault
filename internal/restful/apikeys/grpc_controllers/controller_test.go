package grpc_controllers_test

import (
	"context"
	"github.com/murkio-dev/vault/internal/restful/apikeys/grpc_controllers"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/apikeys/mocks"
	"github.com/murkio-dev/vault/internal/restful/apikeys/useCases"
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
	t.Run("Quando cria uma chave.", func(t *testing.T) { whenCreateSucceeds(t) })
	t.Run("Quando lista as chaves.", func(t *testing.T) { whenListSucceeds(t) })
	t.Run("Quando revoga uma chave.", func(t *testing.T) { whenRevokeSucceeds(t) })
	t.Run("Quando lê, renomeia e rotaciona.", func(t *testing.T) { whenByIDCallsSucceed(t) })
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
	withoutRevoke.DeleteUseCase = nil

	assert.Error(t, grpc_controllers.RegisterControllers(newFakes().input(nil)))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutRevoke))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenCreateSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member := uuid.NewV7(), uuid.NewV7()
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	token := mocks.NewApiKey()
	token.Hash = "hash-que-nao-pode-sair"
	token.ExpiresAt = &expiresAt
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Data: entities.ApiKeyCreated{ApiKey: token, Key: "sk_vt_token-novo", URI: "https://vault.example.com/api/v1/runtime/uri/env"}}
	}

	withExpiry, err := client.CreateApiKey(context.Background(), &vaultv1.CreateApiKeyRequest{ApplicationId: app.String(), MemberId: member.String(), Name: "ci", Type: "secret", ExpiresAt: timestamppb.New(expiresAt)})
	_, withoutExpiryErr := client.CreateApiKey(context.Background(), &vaultv1.CreateApiKeyRequest{ApplicationId: app.String(), MemberId: member.String(), Name: "ci"})

	require.NoError(t, err)
	assert.Equal(t, "sk_vt_token-novo", withExpiry.GetKey(), "o token sai só nesta resposta")
	assert.Equal(t, "https://vault.example.com/api/v1/runtime/uri/env", withExpiry.GetUri())
	assert.Equal(t, token.ID.String(), withExpiry.GetApiKey().GetId())
	assert.Equal(t, token.Name, withExpiry.GetApiKey().GetName())
	assert.Equal(t, token.Prefix, withExpiry.GetApiKey().GetPrefix())
	assert.True(t, expiresAt.Equal(withExpiry.GetApiKey().GetExpiresAt().AsTime()))
	assert.Nil(t, withExpiry.GetApiKey().GetDeletedAt())
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
	active, revoked := mocks.NewApiKey(), mocks.NewApiKey()
	revokedAt := time.Now().UTC().Truncate(time.Second)
	revoked.DeletedAt = &revokedAt
	revoked.Hash = "hash-que-nao-pode-sair"
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Data: []entities.ApiKey{active, revoked}}
	}

	response, err := client.ListApiKeys(context.Background(), &vaultv1.ListApiKeysRequest{ApplicationId: app.String(), MemberId: member.String()})

	require.NoError(t, err)
	if assert.Len(t, response.GetApiKeys(), 2, "a ordem do use case é preservada") {
		assert.Equal(t, active.ID.String(), response.GetApiKeys()[0].GetId())
		assert.Nil(t, response.GetApiKeys()[0].GetDeletedAt())
		assert.True(t, revokedAt.Equal(response.GetApiKeys()[1].GetDeletedAt().AsTime()))
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

	response, err := client.DeleteApiKey(context.Background(), &vaultv1.ApiKeyRequest{ApplicationId: app.String(), MemberId: member.String(), Id: token.String()})

	require.NoError(t, err)
	assert.NotNil(t, response)
	if assert.Len(t, f.revoke.Inputs, 1) {
		assert.Equal(t, app, f.revoke.Inputs[0].Application)
		assert.Equal(t, member, f.revoke.Inputs[0].Member)
		assert.Equal(t, token, f.revoke.Inputs[0].ID)
	}
}

func whenByIDCallsSucceed(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, id := uuid.NewV7().String(), uuid.NewV7()
	key := mocks.NewApiKey()
	key.Hash = "hash-que-nao-pode-sair"
	f.get.PerformFunc = func(context.Context, useCases.GetInput) useCases.GetOutput { return useCases.GetOutput{Data: key} }
	f.rotate.PerformFunc = func(context.Context, useCases.RotateInput) useCases.RotateOutput {
		return useCases.RotateOutput{Data: entities.ApiKeyCreated{ApiKey: key, Key: "sk_vt_rotacionada"}}
	}
	byID := &vaultv1.ApiKeyRequest{ApplicationId: app, MemberId: "me", Id: id.String()}
	change := &vaultv1.ChangeApiKeyRequest{ApplicationId: app, MemberId: "me", Id: id.String(), Name: "deploy"}

	got, getErr := client.GetApiKey(context.Background(), byID)
	_, updateErr := client.UpdateApiKey(context.Background(), change)
	_, upgradeErr := client.UpgradeApiKey(context.Background(), change)
	rotated, rotateErr := client.RotateApiKey(context.Background(), byID)
	_, badID := client.GetApiKey(context.Background(), &vaultv1.ApiKeyRequest{ApplicationId: app, MemberId: "me", Id: "x"})

	for _, err := range []error{getErr, updateErr, upgradeErr, rotateErr} {
		require.NoError(t, err)
	}
	assert.Equal(t, key.ID.String(), got.GetId())
	assert.Equal(t, string(key.Type), got.GetType())
	assert.NotContains(t, got.String(), "hash-que-nao-pode-sair")
	assert.Equal(t, "sk_vt_rotacionada", rotated.GetKey())
	assert.Equal(t, codes.InvalidArgument, status.Code(badID))
	assert.Equal(t, id, f.get.Inputs[0].ID)
	assert.Equal(t, "deploy", f.update.Inputs[0].Name)
	assert.Equal(t, "deploy", f.upgrade.Inputs[0].Name, "UpgradeApiKey vai para o use case de upgrade")
	assert.Equal(t, entities.SelfOwner, f.rotate.Inputs[0].Member)
	assert.Len(t, f.get.Inputs, 1, "id inválido não chega ao use case")
}

func whenMemberIsMe(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, token := uuid.NewV7().String(), uuid.NewV7().String()

	_, createErr := client.CreateApiKey(context.Background(), &vaultv1.CreateApiKeyRequest{ApplicationId: app, MemberId: "me", Name: "ci"})
	_, listErr := client.ListApiKeys(context.Background(), &vaultv1.ListApiKeysRequest{ApplicationId: app, MemberId: "me"})
	_, revokeErr := client.DeleteApiKey(context.Background(), &vaultv1.ApiKeyRequest{ApplicationId: app, MemberId: "me", Id: token})

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
		_, createAppErr := client.CreateApiKey(context.Background(), &vaultv1.CreateApiKeyRequest{ApplicationId: raw, MemberId: valid, Name: "ci"})
		_, createMemberErr := client.CreateApiKey(context.Background(), &vaultv1.CreateApiKeyRequest{ApplicationId: valid, MemberId: raw, Name: "ci"})
		_, listAppErr := client.ListApiKeys(context.Background(), &vaultv1.ListApiKeysRequest{ApplicationId: raw, MemberId: valid})
		_, listMemberErr := client.ListApiKeys(context.Background(), &vaultv1.ListApiKeysRequest{ApplicationId: valid, MemberId: raw})
		_, revokeMemberErr := client.DeleteApiKey(context.Background(), &vaultv1.ApiKeyRequest{ApplicationId: valid, MemberId: raw, Id: valid})
		_, revokeTokenErr := client.DeleteApiKey(context.Background(), &vaultv1.ApiKeyRequest{ApplicationId: valid, MemberId: valid, Id: raw})

		for _, err := range []error{createAppErr, createMemberErr, listAppErr, listMemberErr, revokeMemberErr, revokeTokenErr} {
			assert.Equal(t, codes.InvalidArgument, status.Code(err), raw)
			assert.NotContains(t, status.Convert(err).Message(), "OR 1=1", "o erro não repete o valor recebido")
		}
	}
	_, revokeMe := client.DeleteApiKey(context.Background(), &vaultv1.ApiKeyRequest{ApplicationId: valid, MemberId: valid, Id: "me"})
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
		return useCases.CreateOutput{Error: &conflict, Data: entities.ApiKeyCreated{Key: "sk_vt_nao-vazar"}}
	}
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &forbidden, Data: []entities.ApiKey{mocks.NewApiKey()}}
	}
	f.revoke.PerformFunc = func(context.Context, useCases.DeleteInput) useCases.DeleteOutput {
		return useCases.DeleteOutput{Error: &internal}
	}
	id := uuid.NewV7().String()

	created, createErr := client.CreateApiKey(context.Background(), &vaultv1.CreateApiKeyRequest{ApplicationId: id, MemberId: id})
	listed, listErr := client.ListApiKeys(context.Background(), &vaultv1.ListApiKeysRequest{ApplicationId: id, MemberId: id})
	_, revokeErr := client.DeleteApiKey(context.Background(), &vaultv1.ApiKeyRequest{ApplicationId: id, MemberId: id, Id: id})

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

	_, createErr := client.CreateApiKey(ctx, &vaultv1.CreateApiKeyRequest{ApplicationId: id, MemberId: id})
	_, listErr := client.ListApiKeys(ctx, &vaultv1.ListApiKeysRequest{ApplicationId: id, MemberId: id})
	_, revokeErr := client.DeleteApiKey(ctx, &vaultv1.ApiKeyRequest{ApplicationId: id, MemberId: id, Id: id})

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
		return useCases.CreateOutput{Data: entities.ApiKeyCreated{Key: input.Name}}
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
			response, err := client.CreateApiKey(context.Background(), &vaultv1.CreateApiKeyRequest{ApplicationId: id, MemberId: id, Name: asked[i]})
			if assert.NoError(t, err) {
				answered[i] = response.GetKey()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, asked, answered, "cada chamada recebe a própria resposta")
	assert.Len(t, f.create.Inputs, calls)
}

// Helpers

type fakes struct {
	create  *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list    *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	get     *pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]
	update  *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	upgrade *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	revoke  *pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]
	rotate  *pkgmocks.UseCase[useCases.RotateInput, useCases.RotateOutput]
}

func newFakes() fakes {
	return fakes{
		create:  &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:    &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		get:     &pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]{},
		update:  &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		upgrade: &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		revoke:  &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
		rotate:  &pkgmocks.UseCase[useCases.RotateInput, useCases.RotateOutput]{},
	}
}

func (f fakes) input(registrar grpc.ServiceRegistrar) grpc_controllers.RegisterInput {
	return grpc_controllers.RegisterInput{Registrar: registrar, CreateUseCase: f.create, ListUseCase: f.list, GetUseCase: f.get, UpdateUseCase: f.update,
		UpgradeUseCase: f.upgrade, DeleteUseCase: f.revoke, RotateUseCase: f.rotate}
}

func newClient(t *testing.T, actor entities.Actor) (vaultv1.ApiKeyServiceClient, fakes) {
	t.Helper()

	f := newFakes()
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(f.input(registrar))
	})

	return vaultv1.NewApiKeyServiceClient(conn), f
}
