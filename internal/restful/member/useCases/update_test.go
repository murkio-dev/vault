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

func TestUpdateMember(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenUpdateBuildSucceeds(t) })
	t.Run("Quando muda o papel.", func(t *testing.T) { whenUpdateChangesRole(t) })
	t.Run("Quando muda o status.", func(t *testing.T) { whenUpdateChangesStatus(t) })
	t.Run("Quando não manda nada para mudar.", func(t *testing.T) { whenUpdateHasNothingToChange(t) })
	t.Run("Quando papel ou status são inválidos.", func(t *testing.T) { whenUpdateValuesAreInvalid(t) })
	t.Run("Quando rebaixa o último admin.", func(t *testing.T) { whenUpdateDemotesTheLastAdmin(t) })
	t.Run("Quando rebaixa um admin e há outro.", func(t *testing.T) { whenUpdateDemotesWithAnotherAdmin(t) })
	t.Run("Quando o membro não existe.", func(t *testing.T) { whenUpdateMemberIsMissing(t) })
	t.Run("Quando o acesso é negado.", func(t *testing.T) { whenUpdateIsDenied(t) })
	t.Run("Quando a auditoria falha.", func(t *testing.T) { whenUpdateAuditFails(t) })
	t.Run("Quando ocorre erro de banco.", func(t *testing.T) { whenUpdateFailsInDb(t) })
}

// Functions

func role(r entities.Role) *entities.Role { return &r }

func status(s entities.MemberStatus) *entities.MemberStatus { return &s }

// memberWith devolve o FindById de um membro com o papel e o status dados.
func memberWith(r entities.Role, s entities.MemberStatus) func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
	return func(_ context.Context, application, id uuid.UUID) (entities.Member, error) {
		member := mocks.NewMember(r)
		member.ID, member.ApplicationID, member.Status = id, application, s
		return member, nil
	}
}

// adminsCount devolve o CountActiveAdmins que conta os admins ativos.
func adminsCount(total int) func(context.Context, uuid.UUID) (int, error) {
	return func(context.Context, uuid.UUID) (int, error) { return total, nil }
}

func whenUpdateBuildSucceeds(t *testing.T) {
	repo, guard, tx := &mocks.MemberRepo{}, &mocks.Guard{}, &audit_mocks.Tx{}

	useCase := useCases.NewUpdateMemberUseCase(repo, repo, repo, guard, tx)

	assert.Equal(t, repo, useCase.Finder)
	assert.Equal(t, guard, useCase.Guard)
	assert.Equal(t, tx, useCase.Tx)
}

func whenUpdateChangesRole(t *testing.T) {
	repo := &mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive)}
	tx := &audit_mocks.Tx{}

	output := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, tx).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role(entities.RoleAdmin)})

	assert.Nil(t, output.Error)
	if assert.Len(t, repo.Upgraded, 1) {
		assert.Equal(t, entities.RoleAdmin, repo.Upgraded[0].Role)
	}
	assert.Equal(t, []string{entities.AuditMemberUpdated}, tx.Actions())
}

func whenUpdateChangesStatus(t *testing.T) {
	repo := &mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive)}

	output := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Status: status(entities.MemberSuspended)})

	assert.Nil(t, output.Error)
	if assert.Len(t, repo.Upgraded, 1) {
		assert.Equal(t, entities.MemberSuspended, repo.Upgraded[0].Status)
	}
}

func whenUpdateHasNothingToChange(t *testing.T) {
	repo, guard := &mocks.MemberRepo{}, &mocks.Guard{}

	output := useCases.NewUpdateMemberUseCase(repo, repo, repo, guard, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeValidation, output.Error.Type)
	}
	assert.Zero(t, guard.Calls, "entrada inválida nem chega ao porteiro")
	assert.Empty(t, repo.Upgraded)
}

func whenUpdateValuesAreInvalid(t *testing.T) {
	repo := &mocks.MemberRepo{}
	useCase := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{})

	badRole := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role("root")})
	badStatus := useCase.Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Status: status("banido")})

	for _, output := range []useCases.UpdateOutput{badRole, badStatus} {
		if assert.NotNil(t, output.Error) {
			assert.Equal(t, entities.ErrTypeValidation, output.Error.Type)
			assert.Equal(t, http.StatusBadRequest, output.Error.Status)
		}
	}
	assert.Empty(t, repo.Upgraded)
}

func whenUpdateDemotesTheLastAdmin(t *testing.T) {
	repo := &mocks.MemberRepo{
		FindByIDFunc:    memberWith(entities.RoleAdmin, entities.MemberActive),
		CountAdminsFunc: adminsCount(1),
	}

	demote := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role(entities.RoleDeveloper)})
	suspend := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Status: status(entities.MemberSuspended)})

	for _, output := range []useCases.UpdateOutput{demote, suspend} {
		if assert.NotNil(t, output.Error) {
			assert.Equal(t, entities.ErrTypeConflict, output.Error.Type)
			assert.Equal(t, http.StatusConflict, output.Error.Status)
		}
	}
	assert.Empty(t, repo.Upgraded, "o último admin não é tocado")
}

func whenUpdateDemotesWithAnotherAdmin(t *testing.T) {
	repo := &mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleAdmin, entities.MemberActive), CountAdminsFunc: adminsCount(2)}

	output := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role(entities.RoleDeveloper)})

	assert.Nil(t, output.Error)
	assert.Len(t, repo.Upgraded, 1)
	assert.Len(t, repo.Applications, 3, "acha o membro, conta os admins e grava na mesma aplicação")
}

func whenUpdateMemberIsMissing(t *testing.T) {
	repo := &mocks.MemberRepo{FindByIDFunc: func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		return entities.Member{}, sql.ErrNoRows
	}}

	output := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role(entities.RoleAdmin)})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Error.Type)
	}
}

func whenUpdateIsDenied(t *testing.T) {
	repo := &mocks.MemberRepo{}
	guard := (&mocks.Guard{}).Deny(entities.ForbiddenError())
	tx := &audit_mocks.Tx{}

	output := useCases.NewUpdateMemberUseCase(repo, repo, repo, guard, tx).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role(entities.RoleAdmin)})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	assert.Zero(t, tx.Runs, "negado, nada roda")
	assert.Empty(t, repo.Upgraded)
}

func whenUpdateAuditFails(t *testing.T) {
	tx := &audit_mocks.Tx{AuditErr: errors.New("disk full")}

	output := useCases.NewUpdateMemberUseCase(&mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive)}, &mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive)}, &mocks.MemberRepo{FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive)}, &mocks.Guard{}, tx).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role(entities.RoleAdmin)})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type, "sem trilha, a mudança falha")
	}
}

func whenUpdateFailsInDb(t *testing.T) {
	repo := &mocks.MemberRepo{
		FindByIDFunc: memberWith(entities.RoleDeveloper, entities.MemberActive),
		UpgradeFunc: func(context.Context, entities.Member) (entities.Member, error) {
			return entities.Member{}, errors.New("segredo-interno")
		},
	}

	output := useCases.NewUpdateMemberUseCase(repo, repo, repo, &mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.UpdateInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Member: uuid.NewV7(), Role: role(entities.RoleAdmin)})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "segredo-interno")
	}
}
