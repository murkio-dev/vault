package useCases_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful/application/mocks"
	"github.com/murkio-dev/vault/internal/restful/application/useCases"
	audit_mocks "github.com/murkio-dev/vault/internal/restful/audit/mocks"
	member_mocks "github.com/murkio-dev/vault/internal/restful/member/mocks"
	secret_mocks "github.com/murkio-dev/vault/internal/restful/secret/mocks"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestApplicationAuthorization(t *testing.T) {
	t.Run("Quando o dono vem do ator e não do corpo.", func(t *testing.T) { whenOwnerComesFromTheActor(t) })
	t.Run("Quando a criação vira admin do criador.", func(t *testing.T) { whenCreateMakesTheCreatorAdmin(t) })
	t.Run("Quando a criação é negada.", func(t *testing.T) { whenCreateIsDenied(t) })
	t.Run("Quando a auditoria falha na criação.", func(t *testing.T) { whenCreateAuditFails(t) })
	t.Run("Quando o nome já existe na corrida.", func(t *testing.T) { whenCreateLosesTheNameRace(t) })
	t.Run("Quando a leitura, a troca e a remoção são negadas.", func(t *testing.T) { whenChangesAreDenied(t) })
	t.Run("Quando a auditoria falha na mudança.", func(t *testing.T) { whenChangeAuditFails(t) })
	t.Run("Quando a pessoa lista as próprias aplicações.", func(t *testing.T) { whenPersonListsTheirApplications(t) })
	t.Run("Quando o platform admin lista.", func(t *testing.T) { whenPlatformAdminLists(t) })
	t.Run("Quando quem lista não é uma pessoa autenticada.", func(t *testing.T) { whenListerIsNotAuthenticated(t) })
}

// Functions

func whenOwnerComesFromTheActor(t *testing.T) {
	repo, members, tx := &mocks.ApplicationRepo{}, &member_mocks.MemberRepo{}, &audit_mocks.Tx{}
	requester := mocks.NewRequester()
	forged := mocks.NewApp()
	forged.CreatedBy = uuid.NewV7()

	output := useCases.NewCreateApplicationUseCase(repo, repo, members, &member_mocks.Guard{}, tx).
		Perform(context.Background(), useCases.CreateInput{Requester: requester, Application: []entities.Application{forged}})

	assert.Empty(t, output.Errors)
	if assert.Len(t, output.Data, 1) {
		assert.Equal(t, requester.Actor.UserID, output.Data[0].CreatedBy, "o created_by do corpo é ignorado")
		assert.NotEqual(t, forged.CreatedBy, output.Data[0].CreatedBy)
	}
}

func whenCreateMakesTheCreatorAdmin(t *testing.T) {
	members, tx := &member_mocks.MemberRepo{}, &audit_mocks.Tx{}
	requester := mocks.NewRequester()

	output := useCases.NewCreateApplicationUseCase(&mocks.ApplicationRepo{}, &mocks.ApplicationRepo{}, members, &member_mocks.Guard{}, tx).
		Perform(context.Background(), useCases.CreateInput{Requester: requester, Application: []entities.Application{mocks.NewApp()}})

	assert.Empty(t, output.Errors)
	if assert.Len(t, members.Created, 1) {
		assert.Equal(t, entities.RoleAdmin, members.Created[0].Role)
		assert.Equal(t, requester.Actor.UserID, members.Created[0].UserID)
		assert.Equal(t, output.Data[0].ID, members.Created[0].ApplicationID)
	}
	assert.Equal(t, []string{entities.AuditApplicationCreated}, tx.Actions())
}

func whenCreateIsDenied(t *testing.T) {
	repo, members := &mocks.ApplicationRepo{}, &member_mocks.MemberRepo{}
	guard := (&member_mocks.Guard{}).Deny(entities.UnauthorizedError())

	output := useCases.NewCreateApplicationUseCase(repo, repo, members, guard, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.CreateInput{Requester: entities.ApiHeader{}, Application: []entities.Application{mocks.NewApp()}})

	assert.Nil(t, output.Data)
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusUnauthorized, output.Errors[0].Status)
	}
	assert.Zero(t, len(repo.Created))
	assert.Empty(t, members.Created)
}

func whenCreateAuditFails(t *testing.T) {
	tx := &audit_mocks.Tx{AuditErr: errors.New("disk full")}

	output := useCases.NewCreateApplicationUseCase(&mocks.ApplicationRepo{}, &mocks.ApplicationRepo{}, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, tx).
		Perform(context.Background(), useCases.CreateInput{Requester: mocks.NewRequester(), Application: []entities.Application{mocks.NewApp()}})

	assert.Nil(t, output.Data, "sem trilha, a aplicação não nasce")
	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, entities.ErrTypeInternal, output.Errors[0].Type)
	}
}

func whenCreateLosesTheNameRace(t *testing.T) {
	repo := &mocks.ApplicationRepo{CreateFunc: func(context.Context, entities.Application) (entities.Application, error) {
		return entities.Application{}, secret_mocks.NewUniqueViolation()
	}}

	output := useCases.NewCreateApplicationUseCase(repo, repo, &member_mocks.MemberRepo{}, &member_mocks.Guard{}, &audit_mocks.Tx{}).
		Perform(context.Background(), useCases.CreateInput{Requester: mocks.NewRequester(), Application: []entities.Application{mocks.NewApp()}})

	if assert.Len(t, output.Errors, 1) {
		assert.Equal(t, http.StatusConflict, output.Errors[0].Status, "a restrição do banco vira conflito, não 500")
	}
}

func whenChangesAreDenied(t *testing.T) {
	repo, tx := &mocks.ApplicationRepo{}, &audit_mocks.Tx{}
	denied := (&member_mocks.Guard{}).Deny(entities.NotFoundError("Resource"))
	app := mocks.NewApp()
	requester := mocks.NewRequester()

	get := useCases.NewGetApplicationByIDUseCase(repo, denied).Perform(context.Background(), useCases.GetByIDInput{Requester: requester, Application: app.ID})
	update := useCases.NewUpdateApplicationUseCase(repo, denied, tx).Perform(context.Background(), useCases.UpdateInput{Requester: requester, Applications: []entities.Application{app}})
	upgrade := useCases.NewUpgradeApplicationUseCase(repo, denied, tx).Perform(context.Background(), useCases.UpgradeInput{Requester: requester, Applications: []entities.Application{app}})
	remove := useCases.NewDeleteApplicationUseCase(repo, denied, tx).Perform(context.Background(), useCases.DeleteInput{Requester: requester, IDs: []uuid.UUID{app.ID}})

	if assert.NotNil(t, get.Error) {
		assert.Equal(t, http.StatusNotFound, get.Error.Status)
	}
	for _, errs := range [][]entities.Error{update.Errors, upgrade.Errors, remove.Errors} {
		if assert.Len(t, errs, 1) {
			assert.Equal(t, http.StatusNotFound, errs[0].Status)
		}
	}
	assert.Zero(t, len(repo.Updated)+len(repo.Upgraded)+len(repo.Deleted), "negado, o repositório não é chamado")
	assert.Zero(t, tx.Runs)
}

func whenChangeAuditFails(t *testing.T) {
	repo, tx := &mocks.ApplicationRepo{}, &audit_mocks.Tx{AuditErr: errors.New("disk full")}
	app := mocks.NewApp()
	requester := mocks.NewRequester()

	update := useCases.NewUpdateApplicationUseCase(repo, &member_mocks.Guard{}, tx).Perform(context.Background(), useCases.UpdateInput{Requester: requester, Applications: []entities.Application{app}})
	remove := useCases.NewDeleteApplicationUseCase(repo, &member_mocks.Guard{}, tx).Perform(context.Background(), useCases.DeleteInput{Requester: requester, IDs: []uuid.UUID{app.ID}})

	for _, errs := range [][]entities.Error{update.Errors, remove.Errors} {
		if assert.Len(t, errs, 1) {
			assert.Equal(t, entities.ErrTypeInternal, errs[0].Type, "sem trilha, a mudança falha")
		}
	}
}

func whenPersonListsTheirApplications(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	requester := mocks.NewRequester()

	output := useCases.NewListApplicationsUseCase(repo, repo).Perform(context.Background(), useCases.ListInput{Requester: requester})

	assert.Nil(t, output.Error)
	assert.Equal(t, []uuid.UUID{requester.Actor.UserID}, repo.Users, "só as aplicações de que a pessoa é membro")
}

func whenPlatformAdminLists(t *testing.T) {
	called := false
	repo := &mocks.ApplicationRepo{ListFunc: func(context.Context, int, int) ([]entities.Application, int, error) {
		called = true
		return []entities.Application{}, 0, nil
	}}
	requester := mocks.NewRequester()
	requester.Actor.PlatformAdmin = true

	output := useCases.NewListApplicationsUseCase(repo, repo).Perform(context.Background(), useCases.ListInput{Requester: requester})

	assert.Nil(t, output.Error)
	assert.True(t, called, "o platform admin vê todas")
	assert.Empty(t, repo.Users, "o platform admin lista sem filtro de conta")
}

func whenListerIsNotAuthenticated(t *testing.T) {
	repo := &mocks.ApplicationRepo{}
	token := member_mocks.NewTokenRequester(member_mocks.NewMember(entities.RoleDeveloper))

	for name, requester := range map[string]entities.ApiHeader{"anônimo": {}, "token do SDK": token} {
		output := useCases.NewListApplicationsUseCase(repo, repo).Perform(context.Background(), useCases.ListInput{Requester: requester})

		if assert.NotNil(t, output.Error, name) {
			assert.Equal(t, http.StatusUnauthorized, output.Error.Status, name)
		}
	}
	assert.Empty(t, repo.Users, "quem não está autenticado nem chega ao repositório")
	assert.Empty(t, repo.Windows)
}
