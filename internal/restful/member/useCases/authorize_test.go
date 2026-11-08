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

func TestAuthorize(t *testing.T) {
	t.Run("Quando o usecase builda corretamente.", func(t *testing.T) { whenAuthorizeBuildSucceeds(t) })
	t.Run("Quando o admin é membro ativo.", func(t *testing.T) { whenAdminIsAllowed(t) })
	t.Run("Quando o developer age no que é dele.", func(t *testing.T) { whenDeveloperActsOnOwnData(t) })
	t.Run("Quando o developer age no que é de outro.", func(t *testing.T) { whenDeveloperActsOnOthersData(t) })
	t.Run("Quando quem pede não é membro.", func(t *testing.T) { whenRequesterIsNotMember(t) })
	t.Run("Quando o membro está suspenso.", func(t *testing.T) { whenMemberIsSuspended(t) })
	t.Run("Quando não há credencial.", func(t *testing.T) { whenThereIsNoCredential(t) })
	t.Run("Quando o platform admin não é membro.", func(t *testing.T) { whenPlatformAdminIsNotMember(t) })
	t.Run("Quando o dono precisa existir.", func(t *testing.T) { whenOwnerMustExist(t) })
	t.Run("Quando o token do SDK pede o ambiente.", func(t *testing.T) { whenSDKTokenReadsEnv(t) })
	t.Run("Quando o banco falha.", func(t *testing.T) { whenAuthorizeRepositoryFails(t) })
	t.Run("Quando gravar a negação falha.", func(t *testing.T) { whenRecordingTheDenialFails(t) })
}

// Functions

func newUseCase(repo *mocks.MemberRepo, recorder *audit_mocks.AuditRepo) useCases.AuthorizeUseCase {
	return useCases.NewAuthorizeUseCase(repo, repo, recorder)
}

// memberOf devolve o repositório em que a conta do requester é membro da
// aplicação, com o papel e o status dados (o Authorize a acha pelo FindByUser).
func memberOf(role entities.Role, status entities.MemberStatus, app uuid.UUID, requester entities.ApiHeader) *mocks.MemberRepo {
	member := mocks.NewMember(role)
	member.Status, member.ApplicationID, member.UserID = status, app, requester.Actor.UserID
	repo := mocks.NewRepo()
	repo.FindByUserFunc = func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		return member, nil
	}
	return repo
}

// notAMember devolve o repositório em que a conta não é membro.
func notAMember() *mocks.MemberRepo {
	return &mocks.MemberRepo{}
}

func whenAuthorizeBuildSucceeds(t *testing.T) {
	repo, recorder := &mocks.MemberRepo{}, &audit_mocks.AuditRepo{}

	useCase := newUseCase(repo, recorder)

	assert.Equal(t, repo, useCase.Finder)
	assert.Equal(t, recorder, useCase.Audit)
}

func whenAdminIsAllowed(t *testing.T) {
	requester, app := mocks.NewUserRequester(), uuid.NewV7()
	recorder := &audit_mocks.AuditRepo{}

	output := newUseCase(memberOf(entities.RoleAdmin, entities.MemberActive, app, requester), recorder).
		Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionSecretReveal, Owner: uuid.NewV7()})

	assert.Nil(t, output.Error)
	assert.Equal(t, entities.RoleAdmin, output.Member.Role)
	assert.Empty(t, recorder.Created, "ação permitida não grava negação")
}

func whenDeveloperActsOnOwnData(t *testing.T) {
	requester, app := mocks.NewUserRequester(), uuid.NewV7()
	repo := memberOf(entities.RoleDeveloper, entities.MemberActive, app, requester)

	output := newUseCase(repo, &audit_mocks.AuditRepo{}).
		Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionSecretWrite, Owner: entities.SelfOwner})

	assert.Nil(t, output.Error)
	assert.NotEqual(t, entities.SelfOwner, output.Owner, "o me vira o id do membro")
	assert.Equal(t, output.Member.ID, output.Owner)
}

func whenDeveloperActsOnOthersData(t *testing.T) {
	requester, app := mocks.NewUserRequester(), uuid.NewV7()
	recorder := &audit_mocks.AuditRepo{}

	output := newUseCase(memberOf(entities.RoleDeveloper, entities.MemberActive, app, requester), recorder).
		Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionSecretReveal, Owner: uuid.NewV7()})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeForbidden, output.Error.Type)
		assert.Equal(t, http.StatusForbidden, output.Error.Status)
	}
	if assert.Len(t, recorder.Created, 1) {
		assert.Equal(t, entities.AuditAccessDenied, recorder.Created[0].Action)
		assert.Equal(t, entities.AuditDenied, recorder.Created[0].Outcome)
	}
}

func whenRequesterIsNotMember(t *testing.T) {
	repo := notAMember()
	recorder := &audit_mocks.AuditRepo{}

	output := newUseCase(repo, recorder).
		Perform(context.Background(), entities.AuthorizeInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Action: entities.ActionApplicationRead})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeNotFound, output.Error.Type)
		assert.Equal(t, http.StatusNotFound, output.Error.Status, "quem não é membro não sabe o que existe")
	}
	assert.Len(t, recorder.Created, 1)
}

func whenMemberIsSuspended(t *testing.T) {
	requester, app := mocks.NewUserRequester(), uuid.NewV7()

	output := newUseCase(memberOf(entities.RoleAdmin, entities.MemberSuspended, app, requester), &audit_mocks.AuditRepo{}).
		Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionApplicationRead})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusNotFound, output.Error.Status, "suspenso perde o acesso, até sendo admin")
	}
}

func whenThereIsNoCredential(t *testing.T) {
	repo, recorder := &mocks.MemberRepo{}, &audit_mocks.AuditRepo{}

	output := newUseCase(repo, recorder).
		Perform(context.Background(), entities.AuthorizeInput{Requester: entities.ApiHeader{}, Application: uuid.NewV7(), Action: entities.ActionApplicationRead})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeUnauthorized, output.Error.Type)
		assert.Equal(t, http.StatusUnauthorized, output.Error.Status)
	}
	assert.Empty(t, recorder.Created, "sem identidade não há autor para a trilha")
}

func whenPlatformAdminIsNotMember(t *testing.T) {
	requester := mocks.NewUserRequester()
	requester.Actor.PlatformAdmin = true
	repo := notAMember()

	output := newUseCase(repo, &audit_mocks.AuditRepo{}).
		Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: uuid.NewV7(), Action: entities.ActionSecretReveal, Owner: uuid.NewV7()})

	assert.Nil(t, output.Error)
	assert.Equal(t, uuid.UUID{}, output.Member.ID, "sem vínculo, o membro volta zerado")
}

func whenOwnerMustExist(t *testing.T) {
	requester, app, owner := mocks.NewUserRequester(), uuid.NewV7(), uuid.NewV7()
	repo := memberOf(entities.RoleAdmin, entities.MemberActive, app, requester)
	repo.FindByIDFunc = func(_ context.Context, _, id uuid.UUID) (entities.Member, error) {
		if id == owner {
			return mocks.NewMember(entities.RoleDeveloper), nil
		}
		return entities.Member{}, sql.ErrNoRows
	}
	useCase := newUseCase(repo, &audit_mocks.AuditRepo{})

	found := useCase.Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionSecretRead, Owner: owner, OwnerMustExist: true})
	missing := useCase.Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionSecretRead, Owner: uuid.NewV7(), OwnerMustExist: true})
	zero := useCase.Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionSecretRead, OwnerMustExist: true})

	assert.Nil(t, found.Error)
	assert.Equal(t, owner, found.Owner)
	if assert.NotNil(t, missing.Error) {
		assert.Equal(t, http.StatusNotFound, missing.Error.Status)
	}
	if assert.NotNil(t, zero.Error) {
		assert.Equal(t, http.StatusNotFound, zero.Error.Status)
	}
}

func whenSDKTokenReadsEnv(t *testing.T) {
	member := mocks.NewMember(entities.RoleDeveloper)
	repo := &mocks.MemberRepo{FindByIDFunc: func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) { return member, nil }}
	useCase := newUseCase(repo, &audit_mocks.AuditRepo{})

	env := useCase.Perform(context.Background(), entities.AuthorizeInput{Requester: mocks.NewTokenRequester(member), Application: member.ApplicationID, Action: entities.ActionEnvRead, Owner: member.ID})
	manage := useCase.Perform(context.Background(), entities.AuthorizeInput{Requester: mocks.NewTokenRequester(member), Application: member.ApplicationID, Action: entities.ActionApplicationRead})

	assert.Nil(t, env.Error)
	if assert.NotNil(t, manage.Error) {
		assert.Equal(t, http.StatusForbidden, manage.Error.Status, "o token do SDK só lê o ambiente")
	}
}

func whenAuthorizeRepositoryFails(t *testing.T) {
	repo := &mocks.MemberRepo{FindByUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (entities.Member, error) {
		return entities.Member{}, errors.New("database is locked: /var/lib/vault.db")
	}}

	output := newUseCase(repo, &audit_mocks.AuditRepo{}).
		Perform(context.Background(), entities.AuthorizeInput{Requester: mocks.NewUserRequester(), Application: uuid.NewV7(), Action: entities.ActionApplicationRead})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, entities.ErrTypeInternal, output.Error.Type)
		assert.NotContains(t, output.Error.Detail, "vault.db", "o erro interno não vaza")
	}
}

func whenRecordingTheDenialFails(t *testing.T) {
	requester, app := mocks.NewUserRequester(), uuid.NewV7()
	recorder := &audit_mocks.AuditRepo{CreateFunc: func(context.Context, entities.AuditLog) (entities.AuditLog, error) {
		return entities.AuditLog{}, errors.New("disk full")
	}}

	output := newUseCase(memberOf(entities.RoleDeveloper, entities.MemberActive, app, requester), recorder).
		Perform(context.Background(), entities.AuthorizeInput{Requester: requester, Application: app, Action: entities.ActionMemberList})

	if assert.NotNil(t, output.Error) {
		assert.Equal(t, http.StatusForbidden, output.Error.Status, "a ação continua negada mesmo sem gravar a negação")
	}
}
