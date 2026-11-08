package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/audit/mocks"
	"github.com/murkio-dev/vault/internal/restful/audit/useCases"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestListAuditLogs(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenListBuildSucceeds(t) })
	t.Run("Quando o admin lê a trilha toda.", func(t *testing.T) { whenAdminReadsEverything(t) })
	t.Run("Quando o developer lê a trilha.", func(t *testing.T) { whenDeveloperReadsOnlyHisOwn(t) })
	t.Run("Quando o platform admin lê a trilha.", func(t *testing.T) { whenPlatformAdminReadsEverything(t) })
	t.Run("Quando há próxima página.", func(t *testing.T) { whenListHasANextPage(t) })
	t.Run("Quando o limite é inválido ou passa do teto.", func(t *testing.T) { whenListLimitIsClamped(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenListIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenListFailsInDb(t) })
}

// Functions

func listInput(requester entities.ApiHeader) useCases.ListInput {
	return useCases.ListInput{Requester: requester, Application: uuid.NewV7()}
}

// logs devolve n linhas de trilha com ids em ordem decrescente, como o repositório.
func logs(n int) []entities.AuditLog {
	list := make([]entities.AuditLog, n)
	for i := range list {
		list[i] = entities.AuditLog{ID: uuid.NewV7()}
	}
	return list
}

func whenListBuildSucceeds(t *testing.T) {
	repo, guard := &mocks.AuditRepo{}, &member_mocks.Guard{}

	useCase := useCases.NewListAuditLogsUseCase(repo, guard)

	assert.Equal(t, repo, useCase.Lister)
	assert.Equal(t, guard, useCase.Guard)
}

func whenAdminReadsEverything(t *testing.T) {
	repo, guard := &mocks.AuditRepo{}, &member_mocks.Guard{Member: member_mocks.NewMember(entities.RoleAdmin)}
	other := uuid.NewV7()
	input := listInput(member_mocks.NewUserRequester())
	input.Filter.ActorID = &other

	output := useCases.NewListAuditLogsUseCase(repo, guard).Perform(context.Background(), input)

	assert.Nil(t, output.Error)
	if assert.Len(t, repo.Filters, 1) {
		assert.Equal(t, &other, repo.Filters[0].ActorID, "o admin filtra por quem quiser")
		assert.Equal(t, input.Application, repo.Filters[0].ApplicationID)
	}
	assert.Equal(t, []entities.Action{entities.ActionAuditRead}, guard.Actions())
	assert.Equal(t, entities.SelfOwner, guard.Inputs[0].Owner)
}

func whenDeveloperReadsOnlyHisOwn(t *testing.T) {
	repo, guard := &mocks.AuditRepo{}, &member_mocks.Guard{Member: member_mocks.NewMember(entities.RoleDeveloper)}
	requester := member_mocks.NewUserRequester()
	forged := uuid.NewV7()
	input := listInput(requester)
	input.Filter.ActorID = &forged

	output := useCases.NewListAuditLogsUseCase(repo, guard).Perform(context.Background(), input)

	assert.Nil(t, output.Error)
	if assert.Len(t, repo.Filters, 1) && assert.NotNil(t, repo.Filters[0].ActorID) {
		assert.Equal(t, requester.Actor.UserID, *repo.Filters[0].ActorID, "o filtro de autor é trocado pelo próprio ator")
	}
}

func whenPlatformAdminReadsEverything(t *testing.T) {
	repo, guard := &mocks.AuditRepo{}, &member_mocks.Guard{PerformFunc: func(context.Context, entities.AuthorizeInput) entities.AuthorizeOutput {
		return entities.AuthorizeOutput{}
	}}
	requester := member_mocks.NewUserRequester()
	requester.Actor.PlatformAdmin = true

	output := useCases.NewListAuditLogsUseCase(repo, guard).Perform(context.Background(), listInput(requester))

	assert.Nil(t, output.Error)
	assert.Nil(t, repo.Filters[0].ActorID, "o platform admin, sem vínculo, vê tudo")
}

func whenListHasANextPage(t *testing.T) {
	admin := &member_mocks.Guard{Member: member_mocks.NewMember(entities.RoleAdmin)}
	full := logs(useCases.DefaultLimit + 1)
	repo := &mocks.AuditRepo{ListFunc: func(context.Context, entities.AuditFilter, int) ([]entities.AuditLog, error) { return full, nil }}

	withNext := useCases.NewListAuditLogsUseCase(repo, admin).Perform(context.Background(), listInput(member_mocks.NewUserRequester()))
	last := useCases.NewListAuditLogsUseCase(&mocks.AuditRepo{}, admin).Perform(context.Background(), listInput(member_mocks.NewUserRequester()))

	assert.Len(t, withNext.Data, useCases.DefaultLimit, "a linha a mais não sai")
	if assert.NotNil(t, withNext.Page.NextCursor) {
		assert.Equal(t, full[useCases.DefaultLimit-1].ID.String(), *withNext.Page.NextCursor, "o cursor é o id da última linha devolvida")
	}
	assert.Nil(t, last.Page.NextCursor, "na última página o cursor é nulo")
	assert.NotNil(t, last.Data, "lista vazia é um array")
	assert.Equal(t, []int{useCases.DefaultLimit + 1}, repo.Windows, "pede uma linha a mais para saber se há próxima página")
}

func whenListLimitIsClamped(t *testing.T) {
	repo := &mocks.AuditRepo{}
	useCase := useCases.NewListAuditLogsUseCase(repo, &member_mocks.Guard{Member: member_mocks.NewMember(entities.RoleAdmin)})

	for _, limit := range []int{0, -5, useCases.MaxLimit + 1, 100000, 7} {
		input := listInput(member_mocks.NewUserRequester())
		input.Filter.Limit = limit
		useCase.Perform(context.Background(), input)
	}

	d := useCases.DefaultLimit + 1
	assert.Equal(t, []int{d, d, d, d, 8}, repo.Windows)
}

func whenListIsDenied(t *testing.T) {
	repo := &mocks.AuditRepo{}

	output := useCases.NewListAuditLogsUseCase(repo, (&member_mocks.Guard{}).Deny(entities.NotFoundError("Resource"))).
		Perform(context.Background(), listInput(member_mocks.NewUserRequester()))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
	assert.Empty(t, repo.Filters)
}

func whenListFailsInDb(t *testing.T) {
	repo := &mocks.AuditRepo{ListFunc: func(context.Context, entities.AuditFilter, int) ([]entities.AuditLog, error) {
		return nil, errors.New("segredo-interno")
	}}

	output := useCases.NewListAuditLogsUseCase(repo, &member_mocks.Guard{Member: member_mocks.NewMember(entities.RoleAdmin)}).
		Perform(context.Background(), listInput(member_mocks.NewUserRequester()))

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}
