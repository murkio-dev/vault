package useCases_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	"github.com/murkio-dev/vault/internal/restful/member/mocks"
	"github.com/murkio-dev/vault/internal/restful/member/useCases"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestRemoveMember(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenRemoveBuildSucceeds(t) })
	t.Run("Quando remove um developer.", func(t *testing.T) { whenRemoveSucceeds(t) })
	t.Run("Quando remove o último admin.", func(t *testing.T) { whenRemoveTheLastAdmin(t) })
	t.Run("Quando remove um admin e há outro.", func(t *testing.T) { whenRemoveAdminWithAnotherOne(t) })
	t.Run("Quando o membro não existe.", func(t *testing.T) { whenRemoveMemberIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenRemoveIsDenied(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenRemoveFailsInDb(t) })
}

// Functions

func whenRemoveBuildSucceeds(t *testing.T) {
	repo, guard, tx := &mocks.MemberRepo{}, &mocks.Guard{}, &audit_mocks.Tx{}

	useCase := useCases.NewRemoveMemberUseCase(repo, repo, repo, guard, tx)

	assert.Equal(t, repo, useCase.Finder)
	assert.Equal(t, guard, useCase.Guard)
	assert.Equal(t, tx, useCase.Tx)
}

func whenRemoveSucceeds(t *testing.T) {
	repo := &mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive)}
	tx := &audit_mocks.Tx{}

	output := useCases.NewRemoveMemberUseCase(repo, repo, repo, &mocks.Guard{}, tx).
		Perform(context.Background(), useCases.RemoveInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7()})

	assert.Nil(t, output.Error)
	assert.Len(t, repo.Deleted, 1)
	assert.Equal(t, []string{entities.AuditMemberRemoved}, tx.Actions())
}

func whenRemoveTheLastAdmin(t *testing.T) {
	repo := &mocks.MemberRepo{
		FindByIDFunc:    memberWith(entities.RoleAdmin, entities.MemberActive),
		CountAdminsFunc: adminsCount(1),
	}

	output := useCases.NewRemoveMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.RemoveInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeConflict, output.Error.Type)
	}
	assert.Empty(t, repo.Deleted)
}

func whenRemoveAdminWithAnotherOne(t *testing.T) {
	repo := &mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleAdmin, entities.MemberActive), CountAdminsFunc: adminsCount(2)}

	output := useCases.NewRemoveMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.RemoveInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7()})

	assert.Nil(t, output.Error)
	assert.Len(t, repo.Deleted, 1)
}

func whenRemoveMemberIsMissing(t *testing.T) {
	repo := &mocks.MemberRepo{FindByIDFunc: func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		return entities.Member{}, sql.ErrNoRows
	}}

	output := useCases.NewRemoveMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.RemoveInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status)
	}
}

func whenRemoveIsDenied(t *testing.T) {
	repo := &mocks.MemberRepo{}
	guard := (&mocks.Guard{}).Deny(entities.ForbiddenError())

	output := useCases.NewRemoveMemberUseCase(repo, repo, repo, guard, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.RemoveInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Empty(t, repo.Deleted)
}

func whenRemoveFailsInDb(t *testing.T) {
	repo := &mocks.MemberRepo{
		FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive),
		DeleteFunc:   func(context.Context, uuid.UUID, uuid.UUID) error { return errors.New("segredo-interno") },
	}

	output := useCases.NewRemoveMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.RemoveInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}
