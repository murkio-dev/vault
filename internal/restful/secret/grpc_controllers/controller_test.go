package grpc_controllers_test

import (
	"context"
	"github.com/murkio-dev/vault/internal/restful/secret/grpc_controllers"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/murkio-dev/vault/internal/restful/secret/useCases"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const plain = "valor-que-nao-pode-vazar"

// Main test

func TestControllers(t *testing.T) {
	t.Run("Quando todos os parâmetros de registro são válidos.", func(t *testing.T) { whenRegisterIsSuccessful(t) })
	t.Run("Quando falta um parâmetro de registro obrigatório.", func(t *testing.T) { whenRegisterFailsDueToNilParams(t) })
	t.Run("Quando cria um segredo e um lote.", func(t *testing.T) { whenCreateSucceeds(t) })
	t.Run("Quando o lote tem sucesso parcial.", func(t *testing.T) { whenCreateIsPartial(t) })
	t.Run("Quando o lote falha inteiro ou vem vazio.", func(t *testing.T) { whenCreateFails(t) })
	t.Run("Quando lista, busca e lista as versões.", func(t *testing.T) { whenReadsSucceed(t) })
	t.Run("Quando troca o valor, renomeia e apaga.", func(t *testing.T) { whenWritesSucceed(t) })
	t.Run("Quando revela o valor.", func(t *testing.T) { whenRevealSucceeds(t) })
	t.Run("Quando só o reveal devolve valor.", func(t *testing.T) { whenValueNeverLeaksOutsideReveal(t) })
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
	withoutReveal := newFakes().input(grpc.NewServer())
	withoutReveal.RevealUseCase = nil
	withoutCreate := newFakes().input(grpc.NewServer())
	withoutCreate.CreateUseCase = nil

	assert.Error(t, grpc_controllers.RegisterControllers(newFakes().input(nil)))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutReveal))
	assert.Error(t, grpc_controllers.RegisterControllers(withoutCreate))
	assert.Error(t, grpc_controllers.RegisterControllers(grpc_controllers.RegisterInput{}))
}

func whenCreateSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member, target := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.create.PerformFunc = func(_ context.Context, input useCases.CreateInput) useCases.CreateOutput {
		data := make([]entities.Secret, len(input.Items))
		for i, item := range input.Items {
			data[i] = entities.Secret{ID: uuid.NewV7(), ApplicationID: input.Application, MemberID: input.Member, Name: item.Name, Type: item.Type, Source: item.Source, ReferenceSecretID: item.ReferenceSecretID, Version: 1, CreatedAt: time.Now().UTC()}
		}
		return useCases.CreateOutput{Data: data}
	}
	reference := target.String()

	response, err := client.CreateSecrets(context.Background(), &vaultv1.CreateSecretsRequest{ApplicationId: app.String(), MemberId: member.String(), Items: []*vaultv1.SecretDraft{
		{Name: "DATABASE_URL", Type: "url", Source: "static", Value: plain},
		{Name: "ALIAS", Type: "url", Source: "reference", ReferenceSecretId: &reference},
	}})

	require.NoError(t, err)
	if assert.Len(t, response.GetSecrets(), 2, "a ordem do lote é preservada") {
		static, alias := response.GetSecrets()[0], response.GetSecrets()[1]
		assert.Equal(t, "DATABASE_URL", static.GetName())
		assert.Equal(t, "url", static.GetType())
		assert.Equal(t, "static", static.GetSource())
		assert.Equal(t, int32(1), static.GetVersion())
		assert.Equal(t, member.String(), static.GetMemberId())
		assert.Empty(t, static.GetReferenceSecretId())
		assert.Nil(t, static.GetUpdatedAt())
		assert.Equal(t, target.String(), alias.GetReferenceSecretId())
	}
	assert.Empty(t, response.GetErrors())
	assert.NotContains(t, response.String(), plain, "a resposta da criação não repete o valor")
	if assert.Len(t, f.create.Inputs, 1) {
		input := f.create.Inputs[0]
		assert.Equal(t, app, input.Application)
		assert.Equal(t, member, input.Member)
		if assert.Len(t, input.Items, 2) {
			assert.Equal(t, plain, input.Items[0].Value)
			assert.Equal(t, entities.SecretType("url"), input.Items[0].Type)
			assert.Nil(t, input.Items[0].ReferenceSecretID)
			if assert.NotNil(t, input.Items[1].ReferenceSecretID) {
				assert.Equal(t, target, *input.Items[1].ReferenceSecretID)
			}
		}
	}
}

func whenCreateIsPartial(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{
			Data: []entities.Secret{mocks.NewSecret()},
			Errors: []entities.Error{
				entities.ValidationError("The field value is invalid."),
				{Status: http.StatusInternalServerError, Type: entities.ErrTypeInternal, Title: "Internal Server Error", Detail: "segredo-interno"},
			},
		}
	}

	response, err := client.CreateSecrets(context.Background(), &vaultv1.CreateSecretsRequest{ApplicationId: uuid.NewV7().String(), MemberId: "me", Items: []*vaultv1.SecretDraft{{Name: "A", Value: plain}, {Name: "B", Value: plain}, {Name: "C", Value: plain}}})

	require.NoError(t, err, "com ao menos um sucesso a chamada é OK, como o 207 do HTTP")
	assert.Len(t, response.GetSecrets(), 1)
	if assert.Len(t, response.GetErrors(), 2) {
		assert.Equal(t, int32(http.StatusBadRequest), response.GetErrors()[0].GetStatus())
		assert.Equal(t, int32(http.StatusInternalServerError), response.GetErrors()[1].GetStatus())
		assert.Empty(t, response.GetErrors()[1].GetDetail(), "o detalhe do erro interno não sai nem dentro do lote")
	}
	assert.NotContains(t, response.String(), plain)
}

func whenCreateFails(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.create.PerformFunc = func(context.Context, useCases.CreateInput) useCases.CreateOutput {
		return useCases.CreateOutput{Errors: []entities.Error{entities.ConflictError("Secret already exists", "This member already has a secret with that name."), entities.ForbiddenError()}}
	}
	app := uuid.NewV7().String()
	badReference := "nao-e-uuid"

	failed, allFail := client.CreateSecrets(context.Background(), &vaultv1.CreateSecretsRequest{ApplicationId: app, MemberId: "me", Items: []*vaultv1.SecretDraft{{Name: "A", Value: plain}, {Name: "B", Value: plain}}})
	_, empty := client.CreateSecrets(context.Background(), &vaultv1.CreateSecretsRequest{ApplicationId: app, MemberId: "me"})
	_, reference := client.CreateSecrets(context.Background(), &vaultv1.CreateSecretsRequest{ApplicationId: app, MemberId: "me", Items: []*vaultv1.SecretDraft{{Name: "A", Value: plain}, {Name: "ALIAS", Source: "reference", ReferenceSecretId: &badReference}}})

	assert.Nil(t, failed)
	assert.Equal(t, codes.AlreadyExists, status.Code(allFail), "sem nenhum sucesso, vale o status do primeiro erro")
	assert.Equal(t, codes.InvalidArgument, status.Code(empty))
	assert.Equal(t, codes.InvalidArgument, status.Code(reference))
	for _, err := range []error{allFail, empty, reference} {
		assert.NotContains(t, status.Convert(err).Message(), plain, "o erro nunca repete o valor recebido")
	}
	assert.Len(t, f.create.Inputs, 1, "lote vazio e referência inválida não chegam ao use case")
}

func whenReadsSucceed(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member := uuid.NewV7(), uuid.NewV7()
	first, second := mocks.NewSecret(), mocks.NewSecret()
	updatedAt := time.Now().UTC().Truncate(time.Second)
	second.UpdatedAt = &updatedAt
	author := uuid.NewV7()
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Data: []entities.Secret{first, second}}
	}
	f.get.PerformFunc = func(context.Context, useCases.GetInput) useCases.GetOutput {
		return useCases.GetOutput{Data: second}
	}
	f.versions.PerformFunc = func(context.Context, useCases.VersionsInput) useCases.VersionsOutput {
		return useCases.VersionsOutput{Data: []entities.SecretVersion{
			{ID: uuid.NewV7(), Version: 2, CreatedBy: author, CreatedAt: updatedAt, Ciphertext: []byte("cifra-que-nao-pode-sair")},
			{ID: uuid.NewV7(), Version: 1, CreatedBy: author, CreatedAt: updatedAt, Ciphertext: []byte("cifra-que-nao-pode-sair")},
		}}
	}
	request := &vaultv1.SecretRequest{ApplicationId: app.String(), MemberId: member.String(), SecretId: second.ID.String()}

	listed, listErr := client.ListSecrets(context.Background(), &vaultv1.ListSecretsRequest{ApplicationId: app.String(), MemberId: member.String()})
	got, getErr := client.GetSecret(context.Background(), request)
	versions, versionsErr := client.ListSecretVersions(context.Background(), request)

	require.NoError(t, listErr)
	if assert.Len(t, listed.GetSecrets(), 2) {
		assert.Equal(t, first.ID.String(), listed.GetSecrets()[0].GetId())
		assert.Equal(t, second.ID.String(), listed.GetSecrets()[1].GetId())
	}
	require.NoError(t, getErr)
	assert.Equal(t, second.ID.String(), got.GetId())
	assert.Equal(t, second.Name, got.GetName())
	assert.True(t, updatedAt.Equal(got.GetUpdatedAt().AsTime()))
	require.NoError(t, versionsErr)
	if assert.Len(t, versions.GetVersions(), 2, "da mais nova para a mais antiga, como o use case devolve") {
		assert.Equal(t, int32(2), versions.GetVersions()[0].GetVersion())
		assert.Equal(t, author.String(), versions.GetVersions()[0].GetCreatedBy())
		assert.Equal(t, int32(1), versions.GetVersions()[1].GetVersion())
	}
	assert.NotContains(t, versions.String(), "cifra-que-nao-pode-sair", "o ciphertext nunca sai")
	assert.Equal(t, member, f.list.Inputs[0].Member)
	assert.Equal(t, second.ID, f.get.Inputs[0].Secret)
	assert.Equal(t, second.ID, f.versions.Inputs[0].Secret)
}

func whenWritesSucceed(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member, secret := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.update.PerformFunc = func(_ context.Context, input useCases.UpdateInput) useCases.UpdateOutput {
		return useCases.UpdateOutput{Data: entities.Secret{ID: input.Secret, Version: 2}}
	}
	f.rename.PerformFunc = func(_ context.Context, input useCases.RenameInput) useCases.RenameOutput {
		return useCases.RenameOutput{Data: entities.Secret{ID: input.Secret, Name: input.Name}}
	}

	updated, updateErr := client.UpdateSecret(context.Background(), &vaultv1.UpdateSecretRequest{ApplicationId: app.String(), MemberId: member.String(), SecretId: secret.String(), Value: plain})
	renamed, renameErr := client.RenameSecret(context.Background(), &vaultv1.RenameSecretRequest{ApplicationId: app.String(), MemberId: member.String(), SecretId: secret.String(), Name: "NOVO_NOME"})
	deleted, deleteErr := client.DeleteSecret(context.Background(), &vaultv1.SecretRequest{ApplicationId: app.String(), MemberId: member.String(), SecretId: secret.String()})

	require.NoError(t, updateErr)
	assert.Equal(t, int32(2), updated.GetVersion())
	assert.NotContains(t, updated.String(), plain)
	require.NoError(t, renameErr)
	assert.Equal(t, "NOVO_NOME", renamed.GetName())
	require.NoError(t, deleteErr)
	assert.NotNil(t, deleted)
	if assert.Len(t, f.update.Inputs, 1) {
		assert.Equal(t, app, f.update.Inputs[0].Application)
		assert.Equal(t, member, f.update.Inputs[0].Member)
		assert.Equal(t, secret, f.update.Inputs[0].Secret)
		assert.Equal(t, plain, f.update.Inputs[0].Value)
	}
	assert.Equal(t, "NOVO_NOME", f.rename.Inputs[0].Name)
	assert.Equal(t, secret, f.delete.Inputs[0].Secret)
}

func whenRevealSucceeds(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, member, secret := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.reveal.PerformFunc = func(_ context.Context, input useCases.RevealInput) useCases.RevealOutput {
		return useCases.RevealOutput{Data: entities.RevealedSecret{ID: input.Secret, Name: "DATABASE_URL", Version: 3, Value: plain}}
	}

	current, currentErr := client.RevealSecret(context.Background(), &vaultv1.RevealSecretRequest{ApplicationId: app.String(), MemberId: member.String(), SecretId: secret.String()})
	_, oldErr := client.RevealSecret(context.Background(), &vaultv1.RevealSecretRequest{ApplicationId: app.String(), MemberId: member.String(), SecretId: secret.String(), Version: 2})

	require.NoError(t, currentErr)
	assert.Equal(t, secret.String(), current.GetId())
	assert.Equal(t, "DATABASE_URL", current.GetName())
	assert.Equal(t, int32(3), current.GetVersion())
	assert.Equal(t, plain, current.GetValue())
	require.NoError(t, oldErr)
	if assert.Len(t, f.reveal.Inputs, 2) {
		assert.Zero(t, f.reveal.Inputs[0].Version, "versão zero é a atual")
		assert.Equal(t, 2, f.reveal.Inputs[1].Version)
	}
}

func whenValueNeverLeaksOutsideReveal(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	failure := entities.ForbiddenError()
	f.reveal.PerformFunc = func(context.Context, useCases.RevealInput) useCases.RevealOutput {
		return useCases.RevealOutput{Error: &failure, Data: entities.RevealedSecret{Value: plain}}
	}
	id := uuid.NewV7().String()

	revealed, err := client.RevealSecret(context.Background(), &vaultv1.RevealSecretRequest{ApplicationId: id, MemberId: id, SecretId: id})

	assert.Nil(t, revealed, "com erro, o valor não sai mesmo que o use case o tenha preenchido")
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.NotContains(t, status.Convert(err).Message(), plain)
}

func whenMemberIsMe(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	app, secret := uuid.NewV7().String(), uuid.NewV7().String()
	request := &vaultv1.SecretRequest{ApplicationId: app, MemberId: "me", SecretId: secret}

	_, createErr := client.CreateSecrets(context.Background(), &vaultv1.CreateSecretsRequest{ApplicationId: app, MemberId: "me", Items: []*vaultv1.SecretDraft{{Name: "A", Value: plain}}})
	_, listErr := client.ListSecrets(context.Background(), &vaultv1.ListSecretsRequest{ApplicationId: app, MemberId: "me"})
	_, getErr := client.GetSecret(context.Background(), request)
	_, updateErr := client.UpdateSecret(context.Background(), &vaultv1.UpdateSecretRequest{ApplicationId: app, MemberId: "me", SecretId: secret, Value: plain})
	_, renameErr := client.RenameSecret(context.Background(), &vaultv1.RenameSecretRequest{ApplicationId: app, MemberId: "me", SecretId: secret, Name: "B"})
	_, deleteErr := client.DeleteSecret(context.Background(), request)
	_, versionsErr := client.ListSecretVersions(context.Background(), request)
	_, revealErr := client.RevealSecret(context.Background(), &vaultv1.RevealSecretRequest{ApplicationId: app, MemberId: "me", SecretId: secret})

	for _, err := range []error{createErr, listErr, getErr, updateErr, renameErr, deleteErr, versionsErr, revealErr} {
		require.NoError(t, err)
	}
	members := []uuid.UUID{f.create.Inputs[0].Member, f.list.Inputs[0].Member, f.get.Inputs[0].Member, f.update.Inputs[0].Member, f.rename.Inputs[0].Member, f.delete.Inputs[0].Member, f.versions.Inputs[0].Member, f.reveal.Inputs[0].Member}
	for _, member := range members {
		assert.Equal(t, entities.SelfOwner, member, "\"me\" viaja como o marcador até a autorização")
	}
}

func whenIDIsInvalid(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	valid := uuid.NewV7().String()

	for _, raw := range []string{"", "nao-e-uuid", "ME", uuid.UUID{}.String(), "' OR 1=1 --"} {
		for name, ids := range map[string][3]string{"aplicação": {raw, valid, valid}, "membro": {valid, raw, valid}, "segredo": {valid, valid, raw}} {
			request := &vaultv1.SecretRequest{ApplicationId: ids[0], MemberId: ids[1], SecretId: ids[2]}
			_, getErr := client.GetSecret(context.Background(), request)
			_, updateErr := client.UpdateSecret(context.Background(), &vaultv1.UpdateSecretRequest{ApplicationId: ids[0], MemberId: ids[1], SecretId: ids[2], Value: plain})
			_, renameErr := client.RenameSecret(context.Background(), &vaultv1.RenameSecretRequest{ApplicationId: ids[0], MemberId: ids[1], SecretId: ids[2], Name: "B"})
			_, deleteErr := client.DeleteSecret(context.Background(), request)
			_, versionsErr := client.ListSecretVersions(context.Background(), request)
			_, revealErr := client.RevealSecret(context.Background(), &vaultv1.RevealSecretRequest{ApplicationId: ids[0], MemberId: ids[1], SecretId: ids[2]})

			for _, err := range []error{getErr, updateErr, renameErr, deleteErr, versionsErr, revealErr} {
				assert.Equal(t, codes.InvalidArgument, status.Code(err), name+" "+raw)
				assert.NotContains(t, status.Convert(err).Message(), "OR 1=1", "o erro não repete o valor recebido")
				assert.NotContains(t, status.Convert(err).Message(), plain)
			}
		}
		_, createErr := client.CreateSecrets(context.Background(), &vaultv1.CreateSecretsRequest{ApplicationId: raw, MemberId: valid, Items: []*vaultv1.SecretDraft{{Name: "A", Value: plain}}})
		_, listErr := client.ListSecrets(context.Background(), &vaultv1.ListSecretsRequest{ApplicationId: valid, MemberId: raw})
		assert.Equal(t, codes.InvalidArgument, status.Code(createErr), raw)
		assert.Equal(t, codes.InvalidArgument, status.Code(listErr), raw)
	}
	assert.Empty(t, f.create.Inputs, "id inválido não chega ao use case")
	assert.Empty(t, f.list.Inputs)
	assert.Empty(t, f.get.Inputs)
	assert.Empty(t, f.update.Inputs)
	assert.Empty(t, f.rename.Inputs)
	assert.Empty(t, f.delete.Inputs)
	assert.Empty(t, f.versions.Inputs)
	assert.Empty(t, f.reveal.Inputs)
}

func whenUseCaseReturnsErrors(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	forbidden := entities.ForbiddenError()
	notFound := entities.NotFoundError("Resource")
	conflict := entities.ConflictError("Secret is referenced", "Other secrets point to this one.")
	invalid := entities.ValidationError("The field value is invalid.")
	internal := entities.Error{Status: http.StatusInternalServerError, Title: "Internal Server Error", Detail: "segredo-interno"}
	f.list.PerformFunc = func(context.Context, useCases.ListInput) useCases.ListOutput {
		return useCases.ListOutput{Error: &forbidden, Data: []entities.Secret{mocks.NewSecret()}}
	}
	f.get.PerformFunc = func(context.Context, useCases.GetInput) useCases.GetOutput {
		return useCases.GetOutput{Error: &notFound}
	}
	f.update.PerformFunc = func(context.Context, useCases.UpdateInput) useCases.UpdateOutput {
		return useCases.UpdateOutput{Error: &invalid}
	}
	f.rename.PerformFunc = func(context.Context, useCases.RenameInput) useCases.RenameOutput {
		return useCases.RenameOutput{Error: &conflict}
	}
	f.delete.PerformFunc = func(context.Context, useCases.DeleteInput) useCases.DeleteOutput {
		return useCases.DeleteOutput{Error: &conflict}
	}
	f.versions.PerformFunc = func(context.Context, useCases.VersionsInput) useCases.VersionsOutput {
		return useCases.VersionsOutput{Error: &internal}
	}
	id := uuid.NewV7().String()
	request := &vaultv1.SecretRequest{ApplicationId: id, MemberId: id, SecretId: id}

	listed, listErr := client.ListSecrets(context.Background(), &vaultv1.ListSecretsRequest{ApplicationId: id, MemberId: id})
	_, getErr := client.GetSecret(context.Background(), request)
	_, updateErr := client.UpdateSecret(context.Background(), &vaultv1.UpdateSecretRequest{ApplicationId: id, MemberId: id, SecretId: id, Value: plain})
	_, renameErr := client.RenameSecret(context.Background(), &vaultv1.RenameSecretRequest{ApplicationId: id, MemberId: id, SecretId: id, Name: "B"})
	_, deleteErr := client.DeleteSecret(context.Background(), request)
	_, versionsErr := client.ListSecretVersions(context.Background(), request)

	assert.Nil(t, listed, "com erro, nenhum segredo sai")
	assert.Equal(t, codes.PermissionDenied, status.Code(listErr))
	assert.Equal(t, codes.NotFound, status.Code(getErr))
	assert.Equal(t, codes.InvalidArgument, status.Code(updateErr))
	assert.NotContains(t, status.Convert(updateErr).Message(), plain, "o erro não repete o valor recebido")
	assert.Equal(t, codes.AlreadyExists, status.Code(renameErr))
	assert.Equal(t, codes.AlreadyExists, status.Code(deleteErr))
	assert.Equal(t, codes.Internal, status.Code(versionsErr))
	assert.NotContains(t, status.Convert(versionsErr).Message(), "segredo-interno")
}

func whenRequesterIsExtracted(t *testing.T) {
	actor := entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7(), SessionID: "sessao"}
	client, f := newClient(t, actor)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "req-123")
	id := uuid.NewV7().String()
	request := &vaultv1.SecretRequest{ApplicationId: id, MemberId: id, SecretId: id}

	_, createErr := client.CreateSecrets(ctx, &vaultv1.CreateSecretsRequest{ApplicationId: id, MemberId: id, Items: []*vaultv1.SecretDraft{{Name: "A", Value: plain}}})
	_, listErr := client.ListSecrets(ctx, &vaultv1.ListSecretsRequest{ApplicationId: id, MemberId: id})
	_, getErr := client.GetSecret(ctx, request)
	_, updateErr := client.UpdateSecret(ctx, &vaultv1.UpdateSecretRequest{ApplicationId: id, MemberId: id, SecretId: id, Value: plain})
	_, renameErr := client.RenameSecret(ctx, &vaultv1.RenameSecretRequest{ApplicationId: id, MemberId: id, SecretId: id, Name: "B"})
	_, deleteErr := client.DeleteSecret(ctx, request)
	_, versionsErr := client.ListSecretVersions(ctx, request)
	_, revealErr := client.RevealSecret(ctx, &vaultv1.RevealSecretRequest{ApplicationId: id, MemberId: id, SecretId: id})

	for _, err := range []error{createErr, listErr, getErr, updateErr, renameErr, deleteErr, versionsErr, revealErr} {
		require.NoError(t, err)
	}
	requesters := []entities.ApiHeader{
		f.create.Inputs[0].Requester, f.list.Inputs[0].Requester, f.get.Inputs[0].Requester, f.update.Inputs[0].Requester,
		f.rename.Inputs[0].Requester, f.delete.Inputs[0].Requester, f.versions.Inputs[0].Requester, f.reveal.Inputs[0].Requester,
	}
	for _, requester := range requesters {
		assert.Equal(t, actor, requester.Actor, "o ator vem do ctx posto pelo interceptor, nunca do pedido")
		assert.Equal(t, "req-123", requester.RequestID)
		assert.Equal(t, "grpc", requester.UserAgent)
	}
}

func whenCallsAreConcurrent(t *testing.T) {
	client, f := newClient(t, entities.Actor{})
	f.reveal.PerformFunc = func(_ context.Context, input useCases.RevealInput) useCases.RevealOutput {
		return useCases.RevealOutput{Data: entities.RevealedSecret{ID: input.Secret, Value: input.Secret.String()}}
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
			response, err := client.RevealSecret(context.Background(), &vaultv1.RevealSecretRequest{ApplicationId: id, MemberId: id, SecretId: asked[i]})
			if assert.NoError(t, err) {
				answered[i] = response.GetValue()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, asked, answered, "cada chamada recebe o valor do próprio segredo")
	assert.Len(t, f.reveal.Inputs, calls)
}

// Helpers

type fakes struct {
	create   *pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]
	list     *pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]
	get      *pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]
	update   *pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]
	rename   *pkgmocks.UseCase[useCases.RenameInput, useCases.RenameOutput]
	delete   *pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]
	versions *pkgmocks.UseCase[useCases.VersionsInput, useCases.VersionsOutput]
	reveal   *pkgmocks.UseCase[useCases.RevealInput, useCases.RevealOutput]
}

func newFakes() fakes {
	return fakes{
		create:   &pkgmocks.UseCase[useCases.CreateInput, useCases.CreateOutput]{},
		list:     &pkgmocks.UseCase[useCases.ListInput, useCases.ListOutput]{},
		get:      &pkgmocks.UseCase[useCases.GetInput, useCases.GetOutput]{},
		update:   &pkgmocks.UseCase[useCases.UpdateInput, useCases.UpdateOutput]{},
		rename:   &pkgmocks.UseCase[useCases.RenameInput, useCases.RenameOutput]{},
		delete:   &pkgmocks.UseCase[useCases.DeleteInput, useCases.DeleteOutput]{},
		versions: &pkgmocks.UseCase[useCases.VersionsInput, useCases.VersionsOutput]{},
		reveal:   &pkgmocks.UseCase[useCases.RevealInput, useCases.RevealOutput]{},
	}
}

func (f fakes) input(registrar grpc.ServiceRegistrar) grpc_controllers.RegisterInput {
	return grpc_controllers.RegisterInput{
		Registrar: registrar, CreateUseCase: f.create, ListUseCase: f.list, GetUseCase: f.get, UpdateUseCase: f.update,
		RenameUseCase: f.rename, DeleteUseCase: f.delete, VersionsUseCase: f.versions, RevealUseCase: f.reveal,
	}
}

func newClient(t *testing.T, actor entities.Actor) (vaultv1.SecretServiceClient, fakes) {
	t.Helper()

	f := newFakes()
	conn := pkgmocks.NewGRPCConn(t, actor, func(registrar grpc.ServiceRegistrar) error {
		return grpc_controllers.RegisterControllers(f.input(registrar))
	})

	return vaultv1.NewSecretServiceClient(conn), f
}
