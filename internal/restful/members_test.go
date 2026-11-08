package restful_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invite cria um convite e devolve o token em claro.
func (s *stack) invite(admin *person, app, email, role string) (string, string) {
	s.t.Helper()
	status, body := s.do(admin, http.MethodPost, "/api/v1/applications/"+app+"/invitations", map[string]any{"email": email, "role": role})
	require.Equal(s.t, http.StatusCreated, status, "%v", body)
	data := body["data"].(map[string]any)
	return data["id"].(string), data["token"].(string)
}

// join convida e aceita: devolve a pessoa já membro, com o id do membro.
func (s *stack) join(admin *person, app, role string) (person, string) {
	s.t.Helper()
	who := s.person()
	_, token := s.invite(admin, app, "dev@example.com", role)
	status, body := s.do(&who, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})
	require.Equal(s.t, http.StatusCreated, status, "%v", body)
	return who, body["data"].(map[string]any)["id"].(string)
}

func TestInvitations(t *testing.T) {
	t.Run("Quando o admin cria um convite.", func(t *testing.T) { whenAdminCreatesInvitation(t) })
	t.Run("Quando quem não é admin tenta convidar.", func(t *testing.T) { whenNonAdminTriesToInvite(t) })
	t.Run("Quando o convite é aceito.", func(t *testing.T) { whenInvitationIsAccepted(t) })
	t.Run("Quando o convite é reaproveitado ou inválido.", func(t *testing.T) { whenInvitationCannotBeUsed(t) })
	t.Run("Quando o convite vence ou é revogado.", func(t *testing.T) { whenInvitationExpiresOrIsRevoked(t) })
	t.Run("Quando dois aceitam o mesmo link.", func(t *testing.T) { whenTwoAcceptTheSameLink(t) })
	t.Run("Quando o corpo traz campos desconhecidos.", func(t *testing.T) { whenBodyHasUnknownFields(t) })
}

func whenAdminCreatesInvitation(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+app+"/invitations", strings.NewReader(`{"email":"Joao@Example.com","role":"developer"}`))
	req.Header.Set("Authorization", "Bearer "+maria.token)
	req.Header.Set("Accept", "application/x-yaml")
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", "a resposta com o token é sempre JSON, mesmo pedindo yaml")
	assert.Contains(t, rec.Body.String(), `"token":"inv_`)

	token := strings.Split(strings.Split(rec.Body.String(), `"token":"`)[1], `"`)[0]
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM invitations WHERE token_hash = ?`, token), "o banco guarda o hash, não o token")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM invitations WHERE email = 'joao@example.com' AND role = 'developer'`), "e-mail normalizado")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM outbox_events WHERE type = 'email.send' AND exchange = 'messaging.commands' AND envelope = 0 AND payload LIKE '%vault_invitation%'`), "o pedido de e-mail entrou na outbox")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'invitation.created' AND metadata NOT LIKE '%example.com%' AND metadata NOT LIKE '%inv_%'`), "a auditoria não leva e-mail nem token")

	status, list := s.do(&maria, http.MethodGet, "/api/v1/applications/"+app+"/invitations", nil)
	require.Equal(t, http.StatusOK, status)
	item := list["data"].([]any)[0].(map[string]any)
	assert.Equal(t, "pending", item["status"])
	assert.NotContains(t, item, "token", "a listagem nunca traz o token")
}

func whenNonAdminTriesToInvite(t *testing.T) {
	s := newStack(t)
	maria, outsider := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, _ := s.join(&maria, app, "developer")

	forbidden, _ := s.do(&dev, http.MethodPost, "/api/v1/applications/"+app+"/invitations", map[string]any{"email": "x@example.com", "role": "admin"})
	hidden, _ := s.do(&outsider, http.MethodPost, "/api/v1/applications/"+app+"/invitations", map[string]any{"email": "x@example.com", "role": "admin"})
	list, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app+"/invitations", nil)

	assert.Equal(t, http.StatusForbidden, forbidden, "developer não promove ninguém a admin")
	assert.Equal(t, http.StatusNotFound, hidden)
	assert.Equal(t, http.StatusForbidden, list)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM invitations`), "só o convite do join")
}

func whenInvitationIsAccepted(t *testing.T) {
	s := newStack(t)
	maria, joao := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	id, token := s.invite(&maria, app, "joao@example.com", "developer")

	before, _ := s.do(&joao, http.MethodGet, "/api/v1/applications/"+app, nil)
	status, body := s.do(&joao, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})
	after, _ := s.do(&joao, http.MethodGet, "/api/v1/applications/"+app, nil)

	require.Equal(t, http.StatusCreated, status)
	member := body["data"].(map[string]any)
	assert.Equal(t, "developer", member["role"])
	assert.Equal(t, joao.id.String(), member["user_id"], "o membro é a conta que aceitou")
	assert.NotEmpty(t, member["uri_id"])
	assert.Equal(t, http.StatusNotFound, before)
	assert.Equal(t, http.StatusOK, after)

	_, list := s.do(&maria, http.MethodGet, "/api/v1/applications/"+app+"/invitations", nil)
	assert.Equal(t, "accepted", list["data"].([]any)[0].(map[string]any)["status"])
	assert.Equal(t, id, list["data"].([]any)[0].(map[string]any)["id"])
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'invitation.accepted' AND actor_id = ?`, joao.id.String()))
}

func whenInvitationCannotBeUsed(t *testing.T) {
	s := newStack(t)
	maria, joao, ana := s.person(), s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	_, token := s.invite(&maria, app, "joao@example.com", "developer")
	status, _ := s.do(&joao, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})
	require.Equal(t, http.StatusCreated, status)

	replay, replayBody := s.do(&ana, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})
	garbage, garbageBody := s.do(&ana, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": "inv_" + strings.Repeat("A", 43)})
	noPrefix, _ := s.do(&ana, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": "qualquer-coisa"})
	anonymous, _ := s.do(nil, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})

	assert.Equal(t, http.StatusNotFound, replay, "link já usado")
	assert.Equal(t, http.StatusNotFound, garbage)
	assert.Equal(t, http.StatusNotFound, noPrefix)
	assert.Equal(t, http.StatusUnauthorized, anonymous)
	assert.Equal(t, replayBody["error"], garbageBody["error"], "usado e inexistente são indistinguíveis")
	assert.Equal(t, 3, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND actor_id = ?`, ana.id.String()), "as tentativas ficam na trilha")

	again, _ := s.do(&joao, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})
	assert.Equal(t, http.StatusNotFound, again)
}

func whenInvitationExpiresOrIsRevoked(t *testing.T) {
	s := newStack(t)
	maria, joao := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")

	expiredID, expired := s.invite(&maria, app, "a@example.com", "developer")
	_, err := s.db.Exec(`UPDATE invitations SET expires_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Minute), expiredID)
	require.NoError(t, err)
	revokedID, revoked := s.invite(&maria, app, "b@example.com", "developer")
	status, _ := s.do(&maria, http.MethodDelete, "/api/v1/applications/"+app+"/invitations/"+revokedID, nil)
	require.Equal(t, http.StatusNoContent, status)

	expiredStatus, _ := s.do(&joao, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": expired})
	revokedStatus, _ := s.do(&joao, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": revoked})
	revokeAgain, _ := s.do(&maria, http.MethodDelete, "/api/v1/applications/"+app+"/invitations/"+revokedID, nil)

	assert.Equal(t, http.StatusNotFound, expiredStatus)
	assert.Equal(t, http.StatusNotFound, revokedStatus)
	assert.Equal(t, http.StatusNotFound, revokeAgain, "revogar de novo não existe")

	_, list := s.do(&maria, http.MethodGet, "/api/v1/applications/"+app+"/invitations", nil)
	statuses := map[string]bool{}
	for _, item := range list["data"].([]any) {
		statuses[item.(map[string]any)["status"].(string)] = true
	}
	assert.True(t, statuses["expired"])
	assert.True(t, statuses["revoked"])
}

func whenTwoAcceptTheSameLink(t *testing.T) {
	s := newStack(t)
	maria, joao, ana := s.person(), s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	_, token := s.invite(&maria, app, "x@example.com", "developer")

	first, _ := s.do(&joao, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})
	second, _ := s.do(&ana, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token})

	assert.Equal(t, http.StatusCreated, first)
	assert.Equal(t, http.StatusNotFound, second)
	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM members WHERE application_id = ?`, app), "o admin e um só convidado")

	_, token2 := s.invite(&maria, app, "y@example.com", "developer")
	already, body := s.do(&joao, http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token2})
	assert.Equal(t, http.StatusConflict, already, "quem já é membro não entra duas vezes: %v", body)
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM invitations WHERE accepted_at IS NOT NULL AND email = 'y@example.com'`), "e o convite não é consumido à toa")
}

func whenBodyHasUnknownFields(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	extra, _ := s.do(&maria, http.MethodPost, "/api/v1/applications/"+app+"/invitations", map[string]any{"email": "a@example.com", "role": "developer", "created_by": uuid.NewV7().String()})
	badRole, _ := s.do(&maria, http.MethodPost, "/api/v1/applications/"+app+"/invitations", map[string]any{"email": "a@example.com", "role": "root"})
	badEmail, body := s.do(&maria, http.MethodPost, "/api/v1/applications/"+app+"/invitations", map[string]any{"email": "não é e-mail", "role": "developer"})

	assert.Equal(t, http.StatusBadRequest, extra, "campo desconhecido é erro, não é ignorado")
	assert.Equal(t, http.StatusBadRequest, badRole)
	assert.Equal(t, http.StatusBadRequest, badEmail)
	assert.NotContains(t, body["error"].(map[string]any)["detail"], "não é e-mail")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM invitations`))
}

func TestMembers(t *testing.T) {
	t.Run("Quando o admin lista os membros.", func(t *testing.T) { whenAdminListsMembers(t) })
	t.Run("Quando o developer olha o próprio membro.", func(t *testing.T) { whenDeveloperLooksAtHimself(t) })
	t.Run("Quando o papel de um membro muda.", func(t *testing.T) { whenMemberRoleChanges(t) })
	t.Run("Quando o último admin é rebaixado ou removido.", func(t *testing.T) { whenLastAdminIsDemotedOrRemoved(t) })
	t.Run("Quando um membro é suspenso.", func(t *testing.T) { whenAMemberIsSuspendedByAdmin(t) })
	t.Run("Quando um membro é removido.", func(t *testing.T) { whenAMemberIsRemoved(t) })
}

func whenAdminListsMembers(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, _ := s.join(&maria, app, "developer")

	status, body := s.do(&maria, http.MethodGet, "/api/v1/applications/"+app+"/members", nil)
	forbidden, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app+"/members", nil)

	require.Equal(t, http.StatusOK, status)
	assert.Len(t, body["data"], 2)
	assert.Equal(t, http.StatusForbidden, forbidden)
}

func whenDeveloperLooksAtHimself(t *testing.T) {
	s := newStack(t)
	maria, root := s.person(), s.person("admin")
	app := s.createApplication(&maria, "minha-app")
	dev, memberID := s.join(&maria, app, "developer")

	status, body := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app+"/members/me", nil)
	rootStatus, _ := s.do(&root, http.MethodGet, "/api/v1/applications/"+app+"/members/me", nil)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, memberID, body["data"].(map[string]any)["id"])
	assert.Equal(t, http.StatusNotFound, rootStatus, "o platform admin não tem membro na aplicação dos outros")
}

func whenMemberRoleChanges(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, memberID := s.join(&maria, app, "developer")

	denied, _ := s.do(&dev, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+memberID, map[string]any{"role": "admin"})
	promoted, body := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+memberID, map[string]any{"role": "admin"})
	invalid, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+memberID, map[string]any{"role": "root"})
	empty, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+memberID, map[string]any{})
	unknownField, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+memberID, map[string]any{"user_id": uuid.NewV7().String()})
	missing, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+uuid.NewV7().String(), map[string]any{"role": "admin"})

	assert.Equal(t, http.StatusForbidden, denied, "o developer não se promove")
	require.Equal(t, http.StatusOK, promoted)
	assert.Equal(t, "admin", body["data"].(map[string]any)["role"])
	assert.Equal(t, http.StatusBadRequest, invalid)
	assert.Equal(t, http.StatusBadRequest, empty)
	assert.Equal(t, http.StatusBadRequest, unknownField)
	assert.Equal(t, http.StatusNotFound, missing)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'member.updated' AND target_id = ?`, memberID))

	again, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app+"/members", nil)
	assert.Equal(t, http.StatusOK, again, "o novo admin já enxerga os membros")
}

func whenLastAdminIsDemotedOrRemoved(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	_, me := s.do(&maria, http.MethodGet, "/api/v1/applications/"+app+"/members/me", nil)
	mariaMember := me["data"].(map[string]any)["id"].(string)

	demote, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+mariaMember, map[string]any{"role": "developer"})
	suspend, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+mariaMember, map[string]any{"status": "suspended"})
	remove, _ := s.do(&maria, http.MethodDelete, "/api/v1/applications/"+app+"/members/"+mariaMember, nil)

	assert.Equal(t, http.StatusConflict, demote)
	assert.Equal(t, http.StatusConflict, suspend)
	assert.Equal(t, http.StatusConflict, remove)

	_, second := s.join(&maria, app, "admin")
	_ = second
	demoteNow, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+mariaMember, map[string]any{"role": "developer"})
	assert.Equal(t, http.StatusOK, demoteNow, "com outro admin ativo, pode")
	assert.GreaterOrEqual(t, s.count(`SELECT COUNT(*) FROM members WHERE application_id = ? AND role = 'admin' AND status = 'active' AND deleted_at IS NULL`, app), 1)
}

func whenAMemberIsSuspendedByAdmin(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, memberID := s.join(&maria, app, "developer")

	before, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app, nil)
	status, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+memberID, map[string]any{"status": "suspended"})
	after, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app, nil)
	_, _ = s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+memberID, map[string]any{"status": "active"})
	back, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app, nil)

	assert.Equal(t, http.StatusOK, before)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, http.StatusNotFound, after, "suspenso perde o acesso na hora")
	assert.Equal(t, http.StatusOK, back)
}

func whenAMemberIsRemoved(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, memberID := s.join(&maria, app, "developer")
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO access_tokens (id, application_id, member_id, name, prefix, token_hash, created_by, created_at) VALUES ('t', ?, ?, 'n', 'vt_x', 'h', 'u', ?)`, app, memberID, now)
	require.NoError(t, err)
	_, err = s.db.Exec(`INSERT INTO secrets (id, application_id, member_id, name, type, source, created_by, created_at) VALUES ('s', ?, ?, 'A', 'string', 'static', 'u', ?)`, app, memberID, now)
	require.NoError(t, err)

	forbidden, _ := s.do(&dev, http.MethodDelete, "/api/v1/applications/"+app+"/members/"+memberID, nil)
	status, _ := s.do(&maria, http.MethodDelete, "/api/v1/applications/"+app+"/members/"+memberID, nil)
	after, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+app, nil)
	missing, _ := s.do(&maria, http.MethodDelete, "/api/v1/applications/"+app+"/members/"+memberID, nil)

	assert.Equal(t, http.StatusForbidden, forbidden)
	assert.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, http.StatusNotFound, after)
	assert.Equal(t, http.StatusNotFound, missing)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM access_tokens WHERE id = 't' AND revoked_at IS NOT NULL`), "os tokens dele são revogados")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM secrets WHERE id = 's' AND deleted_at IS NOT NULL`), "e os segredos, apagados")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'member.removed' AND target_id = ?`, memberID))
}
