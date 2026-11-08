package entities_test

import (
	"testing"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/stretchr/testify/assert"
)

var allActions = []entities.Action{
	entities.ActionApplicationCreate, entities.ActionApplicationRead, entities.ActionApplicationUpdate, entities.ActionApplicationDelete, entities.ActionInvitationAccept,
	entities.ActionMemberList, entities.ActionMemberUpdate, entities.ActionMemberRemove, entities.ActionInvitationManage,
	entities.ActionSecretRead, entities.ActionSecretWrite, entities.ActionSecretReveal,
	entities.ActionTokenCreate, entities.ActionTokenList, entities.ActionTokenRevoke,
	entities.ActionAuditRead, entities.ActionEnvRead,
}

// Ações que um developer faz só no que é dele.
var ownerScoped = map[entities.Action]bool{
	entities.ActionSecretRead: true, entities.ActionSecretWrite: true, entities.ActionSecretReveal: true,
	entities.ActionTokenCreate: true, entities.ActionTokenList: true, entities.ActionTokenRevoke: true,
	entities.ActionAuditRead: true,
}

func user() entities.Actor {
	return entities.Actor{Kind: entities.ActorUser, UserID: uuid.NewV7()}
}

func member(role entities.Role, status entities.MemberStatus) *entities.Member {
	return &entities.Member{ID: uuid.NewV7(), Role: role, Status: status}
}

func TestDecide(t *testing.T) {
	t.Run("Quando o ator não está autenticado.", func(t *testing.T) { whenActorIsAnonymous(t) })
	t.Run("Quando o ator é platform admin.", func(t *testing.T) { whenActorIsPlatformAdmin(t) })
	t.Run("Quando o ator é admin da aplicação.", func(t *testing.T) { whenActorIsAdmin(t) })
	t.Run("Quando o ator é developer.", func(t *testing.T) { whenActorIsDeveloper(t) })
	t.Run("Quando o ator não é membro ou está suspenso.", func(t *testing.T) { whenActorIsNotAnActiveMember(t) })
	t.Run("Quando o ator é um token do SDK.", func(t *testing.T) { whenActorIsAToken(t) })
	t.Run("Quando a ação é desconhecida.", func(t *testing.T) { whenActionIsUnknown(t) })
}

func whenActorIsAnonymous(t *testing.T) {
	for _, action := range allActions {
		assert.Equal(t, entities.Deny, entities.Decide(entities.Actor{}, member(entities.RoleAdmin, entities.MemberActive), action, uuid.UUID{}), action)
	}
}

func whenActorIsPlatformAdmin(t *testing.T) {
	actor := user()
	actor.PlatformAdmin = true

	for _, action := range allActions {
		if action == entities.ActionEnvRead {
			continue
		}
		assert.Equal(t, entities.Allow, entities.Decide(actor, nil, action, uuid.NewV7()), action)
	}
}

func whenActorIsAdmin(t *testing.T) {
	admin := member(entities.RoleAdmin, entities.MemberActive)

	for _, action := range allActions {
		if action == entities.ActionEnvRead {
			continue
		}
		assert.Equal(t, entities.Allow, entities.Decide(user(), admin, action, uuid.NewV7()), "admin mexe em tudo da aplicação, de qualquer dono: %s", action)
	}
}

func whenActorIsDeveloper(t *testing.T) {
	dev := member(entities.RoleDeveloper, entities.MemberActive)
	other := uuid.NewV7()

	for _, action := range allActions {
		switch {
		case action == entities.ActionApplicationCreate || action == entities.ActionApplicationRead || action == entities.ActionInvitationAccept:
			assert.Equal(t, entities.Allow, entities.Decide(user(), dev, action, uuid.UUID{}), action)
		case ownerScoped[action]:
			assert.Equal(t, entities.Allow, entities.Decide(user(), dev, action, dev.ID), "no que é dele: %s", action)
			assert.Equal(t, entities.Forbidden, entities.Decide(user(), dev, action, other), "no que é de outro: %s", action)
			assert.Equal(t, entities.Forbidden, entities.Decide(user(), dev, action, uuid.UUID{}), "sem dono definido nega: %s", action)
		default:
			assert.Equal(t, entities.Forbidden, entities.Decide(user(), dev, action, dev.ID), "só admin: %s", action)
		}
	}
}

func whenActorIsNotAnActiveMember(t *testing.T) {
	suspended := member(entities.RoleAdmin, entities.MemberSuspended)

	for _, action := range allActions {
		if action == entities.ActionApplicationCreate || action == entities.ActionInvitationAccept || action == entities.ActionEnvRead {
			continue
		}
		assert.Equal(t, entities.Hidden, entities.Decide(user(), nil, action, uuid.NewV7()), "quem não é membro não vê nada: %s", action)
		assert.Equal(t, entities.Hidden, entities.Decide(user(), suspended, action, suspended.ID), "suspenso perde tudo, até sendo admin: %s", action)
	}

	assert.Equal(t, entities.Allow, entities.Decide(user(), nil, entities.ActionApplicationCreate, uuid.UUID{}), "qualquer conta cria uma aplicação")
	assert.Equal(t, entities.Allow, entities.Decide(user(), nil, entities.ActionInvitationAccept, uuid.UUID{}), "qualquer conta tenta aceitar um convite (o token decide)")
}

func whenActorIsAToken(t *testing.T) {
	dev := member(entities.RoleDeveloper, entities.MemberActive)
	token := entities.Actor{Kind: entities.ActorToken, UserID: uuid.NewV7(), TokenID: uuid.NewV7(), MemberID: dev.ID}

	for _, action := range allActions {
		if action == entities.ActionEnvRead {
			assert.Equal(t, entities.Allow, entities.Decide(token, dev, action, dev.ID))
			continue
		}
		assert.Equal(t, entities.Forbidden, entities.Decide(token, dev, action, dev.ID), "o token do SDK só lê o env: %s", action)
	}

	other := member(entities.RoleDeveloper, entities.MemberActive)
	assert.Equal(t, entities.Forbidden, entities.Decide(token, other, entities.ActionEnvRead, other.ID), "o token não lê o env de outro membro")
	assert.Equal(t, entities.Forbidden, entities.Decide(token, member(entities.RoleDeveloper, entities.MemberSuspended), entities.ActionEnvRead, dev.ID), "membro suspenso")
	assert.Equal(t, entities.Forbidden, entities.Decide(token, nil, entities.ActionEnvRead, dev.ID))
}

func whenActionIsUnknown(t *testing.T) {
	admin := member(entities.RoleAdmin, entities.MemberActive)
	platform := user()
	platform.PlatformAdmin = true

	assert.Equal(t, entities.Deny, entities.Decide(user(), admin, entities.Action("segredo.apagar-tudo"), uuid.UUID{}), "admin: ação fora da lista nega")
	assert.Equal(t, entities.Deny, entities.Decide(platform, nil, entities.Action("qualquer"), uuid.UUID{}), "platform admin: ação fora da lista nega")
	assert.Equal(t, entities.Deny, entities.Decide(user(), member(entities.RoleDeveloper, entities.MemberActive), entities.Action(""), uuid.UUID{}))
}
