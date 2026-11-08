package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/runtime/useCases"
	secret_mocks "github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestReadEnv(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenEnvBuildSucceeds(t) })
	t.Run("Quando lê o ambiente do membro.", func(t *testing.T) { whenEnvSucceeds(t) })
	t.Run("Quando filtra por nomes.", func(t *testing.T) { whenEnvIsFiltered(t) })
	t.Run("Quando o membro não tem segredos.", func(t *testing.T) { whenEnvIsEmpty(t) })
	t.Run("Quando a referência perdeu o alvo.", func(t *testing.T) { whenEnvReferenceLostItsTarget(t) })
	t.Run("Quando o uri_id não é o do membro.", func(t *testing.T) { whenEnvURIDoesNotMatch(t) })
	t.Run("Quando quem pede não é um token.", func(t *testing.T) { whenEnvRequesterIsNotAToken(t) })
	t.Run("Quando pede nomes demais.", func(t *testing.T) { whenEnvAsksForTooManyNames(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenEnvIsDenied(t) })
	t.Run("Quando a auditoria falha.", func(t *testing.T) { whenEnvAuditFails(t) })
	t.Run("Quando ocorre erro de banco ou de decifragem.", func(t *testing.T) { whenEnvFails(t) })
}

// Functions

type envRig struct {
	secrets *secret_mocks.SecretRepo
	values  *secret_mocks.Values
	guard   *member_mocks.Guard
	tx      *audit_mocks.Tx
	audit   *audit_mocks.AuditRepo
	member  entities.Member
	useCase useCases.ReadEnvUseCase
}

func newEnvRig() *envRig {
	member := member_mocks.NewMember(entities.RoleDeveloper)
	rig := &envRig{
		secrets: &secret_mocks.SecretRepo{}, values: &secret_mocks.Values{ValueResult: "valor-secreto"},
		guard: &member_mocks.Guard{Member: member}, tx: &audit_mocks.Tx{}, audit: &audit_mocks.AuditRepo{}, member: member,
	}
	rig.useCase = useCases.NewReadEnvUseCase(rig.secrets, rig.values, rig.guard, rig.tx, rig.audit)
	return rig
}

func (r *envRig) withSecrets(names ...string) {
	r.secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		list := make([]entities.Secret, len(names))
		for i, name := range names {
			list[i] = secret_mocks.NewSecret()
			list[i].Name = name
		}
		return list, len(list), nil
	}
}

func (r *envRig) input(names ...string) useCases.ReadEnvInput {
	return useCases.ReadEnvInput{Requester: member_mocks.NewTokenRequester(r.member), UriID: r.member.UriID, Names: names}
}

func whenEnvBuildSucceeds(t *testing.T) {
	rig := newEnvRig()

	assert.Equal(t, rig.secrets, rig.useCase.Secrets)
	assert.Equal(t, rig.values, rig.useCase.Values)
	assert.Equal(t, rig.guard, rig.useCase.Guard)
	assert.Equal(t, rig.tx, rig.useCase.Tx)
	assert.Equal(t, rig.audit, rig.useCase.Audit)
}

func whenEnvSucceeds(t *testing.T) {
	rig := newEnvRig()
	rig.withSecrets("DATABASE_URL", "API_KEY")

	output := rig.useCase.Perform(context.Background(), rig.input())

	assert.Nil(t, output.Error)
	assert.Equal(t, rig.member.UriID, output.Data.UriID)
	if assert.Len(t, output.Data.Variables, 2) {
		assert.Equal(t, "DATABASE_URL", output.Data.Variables[0].Name)
		assert.Equal(t, "valor-secreto", output.Data.Variables[0].Value)
	}
	assert.Equal(t, []entities.Action{entities.ActionEnvRead}, rig.guard.Actions())
	if assert.Len(t, rig.tx.Entries, 1) {
		assert.Equal(t, entities.AuditEnvRead, rig.tx.Entries[0].Action)
		assert.NotContains(t, string(rig.tx.Entries[0].Metadata), "valor-secreto", "a trilha nunca leva o valor")
	}
}

func whenEnvIsFiltered(t *testing.T) {
	rig := newEnvRig()
	rig.withSecrets("DATABASE_URL", "API_KEY")

	output := rig.useCase.Perform(context.Background(), rig.input("API_KEY", "NAO_EXISTE"))

	assert.Nil(t, output.Error)
	if assert.Len(t, output.Data.Variables, 1) {
		assert.Equal(t, "API_KEY", output.Data.Variables[0].Name)
	}
}

func whenEnvIsEmpty(t *testing.T) {
	rig := newEnvRig()

	output := rig.useCase.Perform(context.Background(), rig.input())

	assert.Nil(t, output.Error)
	assert.NotNil(t, output.Data.Variables, "sem segredos é lista vazia, não nula")
	assert.Empty(t, output.Data.Variables)
	assert.Len(t, rig.tx.Entries, 1, "a leitura é auditada mesmo vazia")
}

func whenEnvReferenceLostItsTarget(t *testing.T) {
	rig := newEnvRig()
	rig.withSecrets("GONE", "OK")
	calls := 0
	rig.values.ValueErr = nil
	rig.secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		gone, ok := secret_mocks.NewSecret(), secret_mocks.NewSecret()
		gone.Name, ok.Name = "GONE", "OK"
		return []entities.Secret{gone, ok}, 2, nil
	}
	failing := &failOnFirst{inner: rig.values, calls: &calls}
	useCase := useCases.NewReadEnvUseCase(rig.secrets, failing, rig.guard, rig.tx, rig.audit)

	output := useCase.Perform(context.Background(), rig.input())

	assert.Nil(t, output.Error)
	if assert.Len(t, output.Data.Variables, 1) {
		assert.Equal(t, "OK", output.Data.Variables[0].Name, "a referência órfã é omitida, o resto sai")
	}
}

// failOnFirst faz o primeiro Value falhar com sql.ErrNoRows (alvo apagado).
type failOnFirst struct {
	inner *secret_mocks.Values
	calls *int
}

func (f *failOnFirst) Seal(id uuid.UUID, value string) ([]byte, error) {
	return f.inner.Seal(id, value)
}

func (f *failOnFirst) Value(ctx context.Context, secret entities.Secret, version int) (string, int, error) {
	*f.calls++
	if *f.calls == 1 {
		return "", 0, sql.ErrNoRows
	}
	return f.inner.Value(ctx, secret, version)
}

func whenEnvURIDoesNotMatch(t *testing.T) {
	rig := newEnvRig()
	rig.withSecrets("A")
	input := rig.input()
	input.UriID = strings.Repeat("0", len(rig.member.UriID))

	output := rig.useCase.Perform(context.Background(), input)

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Empty(t, output.Data.Variables)
	assert.Empty(t, rig.tx.Entries)
	if assert.Len(t, rig.audit.Created, 1) {
		assert.Equal(t, entities.AuditAccessDenied, rig.audit.Created[0].Action, "a tentativa fica na trilha")
	}
}

func whenEnvRequesterIsNotAToken(t *testing.T) {
	rig := newEnvRig()

	output := rig.useCase.Perform(context.Background(), useCases.ReadEnvInput{Requester: member_mocks.NewUserRequester(), UriID: rig.member.UriID})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusUnauthorized, output.Error.Status, "o JWT não lê o runtime")
	}
	assert.Zero(t, rig.guard.Calls)
}

func whenEnvAsksForTooManyNames(t *testing.T) {
	rig := newEnvRig()

	output := rig.useCase.Perform(context.Background(), rig.input(make([]string, useCases.MaxNames+1)...))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeValidation, output.Error.Type)
	}
}

func whenEnvIsDenied(t *testing.T) {
	rig := newEnvRig()
	rig.guard.Deny(entities.ForbiddenError())

	output := rig.useCase.Perform(context.Background(), rig.input())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Zero(t, rig.tx.Runs)
}

func whenEnvAuditFails(t *testing.T) {
	rig := newEnvRig()
	rig.withSecrets("A")
	rig.tx.AuditErr = errors.New("disk full")

	output := rig.useCase.Perform(context.Background(), rig.input())

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
	}
	assert.Empty(t, output.Data.Variables, "sem trilha, nenhum valor sai")
}

func whenEnvFails(t *testing.T) {
	list := newEnvRig()
	list.secrets.ListFunc = func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]entities.Secret, int, error) {
		return nil, 0, errors.New("segredo-interno")
	}
	decrypt := newEnvRig()
	decrypt.withSecrets("A")
	decrypt.values.ValueErr = errors.New("segredo-interno")

	for name, rig := range map[string]*envRig{"listagem": list, "decifragem": decrypt} {
		output := rig.useCase.Perform(context.Background(), rig.input())

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, entities.ErrTypeInternal, output.Error.Type, name)
			assert.NotContains(t, output.Error.Detail, "segredo-interno", name)
		}
		assert.Empty(t, output.Data.Variables, name)
	}
}
