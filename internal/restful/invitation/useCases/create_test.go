package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/mocks"
	"github.com/murkio-dev/vault/internal/restful/invitation/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestCreateInvitation(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenCreateBuildSucceeds(t) })
	t.Run("Quando cria o convite.", func(t *testing.T) { whenCreateSucceeds(t) })
	t.Run("Quando o e-mail ou o papel são inválidos.", func(t *testing.T) { whenCreateInputIsInvalid(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenCreateIsDenied(t) })
	t.Run("Quando o repositório falha.", func(t *testing.T) { whenCreateRepositoryFails(t) })
	t.Run("Quando a outbox falha.", func(t *testing.T) { whenCreateOutboxFails(t) })
	t.Run("Quando a auditoria falha.", func(t *testing.T) { whenCreateAuditFails(t) })
}

// Functions

func newCreate(repo *mocks.InvitationRepo, outbox *mocks.OutboxRepo, guard *member_mocks.Guard, tx *audit_mocks.Tx) useCases.CreateInvitationUseCase {
	return useCases.NewCreateInvitationUseCase(repo, outbox, guard, tx)
}

func validCreate() useCases.CreateInput {
	return useCases.CreateInput{Requester: member_mocks.NewUserRequester(), Application: uuid.NewV7(), Email: utils.GenRandomEmail(), Role: entities.RoleDeveloper}
}

func whenCreateBuildSucceeds(t *testing.T) {
	repo, outbox, guard, tx := &mocks.InvitationRepo{}, &mocks.OutboxRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{}

	useCase := newCreate(repo, outbox, guard, tx)

	assert.Equal(t, repo, useCase.Creator)
	assert.Equal(t, outbox, useCase.Outbox)
	assert.Equal(t, guard, useCase.Guard)
	assert.Equal(t, tx, useCase.Tx)
}

func whenCreateSucceeds(t *testing.T) {
	repo, outbox, guard, tx := &mocks.InvitationRepo{}, &mocks.OutboxRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{}
	input := validCreate()

	output := newCreate(repo, outbox, guard, tx).Perform(context.Background(), input)

	assert.Nil(t, output.Error)
	assert.True(t, strings.HasPrefix(output.Data.Token, entities.InvitationTokenPrefix))
	assert.Equal(t, input.Application, output.Data.ApplicationID)
	assert.Equal(t, []entities.Action{entities.ActionInvitationManage}, guard.Actions())
	if assert.Len(t, repo.Created, 1) {
		assert.Equal(t, entities.HashToken(output.Data.Token), repo.Created[0].TokenHash, "o banco recebe o hash")
		assert.NotEqual(t, output.Data.Token, repo.Created[0].TokenHash)
		assert.Equal(t, input.Application, repo.Application(), "grava no escopo da aplicação")
	}
	if assert.Len(t, outbox.Created, 1) {
		assert.Equal(t, entities.EmailSendRoutingKey, outbox.Created[0].Type)
		assert.False(t, outbox.Created[0].Envelope, "comando: o payload é apagado depois de publicado")
		assert.Contains(t, string(outbox.Created[0].Payload), output.Data.Token, "o e-mail leva o token")
	}
	if assert.Len(t, tx.Entries, 1) {
		assert.Equal(t, entities.AuditInvitationCreated, tx.Entries[0].Action)
		assert.NotContains(t, string(tx.Entries[0].Metadata), output.Data.Token, "a trilha nunca leva o token")
		assert.NotContains(t, string(tx.Entries[0].Metadata), input.Email, "nem o e-mail")
	}
}

func whenCreateInputIsInvalid(t *testing.T) {
	repo, outbox := &mocks.InvitationRepo{}, &mocks.OutboxRepo{}
	useCase := newCreate(repo, outbox, &member_mocks.Guard{}, &audit_mocks.Tx{})

	for name, mutate := range map[string]func(*useCases.CreateInput){
		"e-mail vazio":       func(i *useCases.CreateInput) { i.Email = "" },
		"e-mail sem domínio": func(i *useCases.CreateInput) { i.Email = "maria@localhost" },
		"e-mail com nome":    func(i *useCases.CreateInput) { i.Email = "Maria <maria@example.com>" },
		"e-mail enorme":      func(i *useCases.CreateInput) { i.Email = strings.Repeat("a", 250) + "@example.com" },
		"papel desconhecido": func(i *useCases.CreateInput) { i.Role = "root" },
		"papel vazio":        func(i *useCases.CreateInput) { i.Role = "" },
	} {
		input := validCreate()
		mutate(&input)

		output := useCase.Perform(context.Background(), input)

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeValidation, output.Error.Type, name)
			assert.Equal(t, http.StatusBadRequest, output.Error.Status, name)
		}
	}
	assert.Empty(t, repo.Created, "inválido, o repositório não é chamado")
	assert.Empty(t, outbox.Created)
}

func whenCreateIsDenied(t *testing.T) {
	repo, outbox := &mocks.InvitationRepo{}, &mocks.OutboxRepo{}
	guard := (&member_mocks.Guard{}).Deny(entities.ForbiddenError())

	output := newCreate(repo, outbox, guard, &audit_mocks.Tx{}).Perform(context.Background(), validCreate())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, repo.Created)
	assert.Empty(t, outbox.Created)
	assert.Empty(t, output.Data.Token, "negado, nenhum token sai")
}

func whenCreateRepositoryFails(t *testing.T) {
	repo := &mocks.InvitationRepo{CreateFunc: func(context.Context, entities.Invitation) (entities.Invitation, error) {
		return entities.Invitation{}, errors.New("segredo-interno")
	}}
	outbox := &mocks.OutboxRepo{}

	output := newCreate(repo, outbox, &member_mocks.Guard{}, &audit_mocks.Tx{}).Perform(context.Background(), validCreate())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
	assert.Empty(t, output.Data.Token, "se falhou, o token não vai para o cliente")
	assert.Empty(t, outbox.Created)
}

func whenCreateOutboxFails(t *testing.T) {
	outbox := &mocks.OutboxRepo{CreateFunc: func(context.Context, entities.OutboxEvent) (entities.OutboxEvent, error) {
		return entities.OutboxEvent{}, errors.New("disk full")
	}}

	output := newCreate(&mocks.InvitationRepo{}, outbox, &member_mocks.Guard{}, &audit_mocks.Tx{}).Perform(context.Background(), validCreate())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
	}
	assert.Empty(t, output.Data.Token)
}

func whenCreateAuditFails(t *testing.T) {
	tx := &audit_mocks.Tx{AuditErr: errors.New("disk full")}

	output := newCreate(&mocks.InvitationRepo{}, &mocks.OutboxRepo{}, &member_mocks.Guard{}, tx).Perform(context.Background(), validCreate())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
	}
	assert.Empty(t, output.Data.Token, "sem trilha, o convite não nasce")
}
