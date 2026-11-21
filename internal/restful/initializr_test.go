package restful_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/authn/authntest"
	"github.com/murkio-dev/vault/pkg/dependencies"
	"github.com/murkio-dev/vault/pkg/migration"
	pkgmocks "github.com/murkio-dev/vault/pkg/mocks"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/murkio-dev/vault/pkg/sqlite/sqlitetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Main test

func TestInit(t *testing.T) {
	t.Run("Quando falta o mux no registro HTTP.", func(t *testing.T) { whenInitHasNoMux(t) })
	t.Run("Quando sobe o gRPC com todos os services.", func(t *testing.T) { whenInitGRPCRegistersEveryService(t) })
	t.Run("Quando a porta do gRPC já está em uso.", func(t *testing.T) { whenInitGRPCPortIsInUse(t) })
}

func TestApplications(t *testing.T) {
	t.Run("Quando não há credencial.", func(t *testing.T) { whenThereIsNoCredential(t) })
	t.Run("Quando uma pessoa cria uma aplicação.", func(t *testing.T) { whenPersonCreatesApplication(t) })
	t.Run("Quando o corpo tenta definir o dono.", func(t *testing.T) { whenBodyTriesToSetTheOwner(t) })
	t.Run("Quando outra conta tenta ver a aplicação.", func(t *testing.T) { whenAnotherAccountLooksAtTheApplication(t) })
	t.Run("Quando cada conta lista as aplicações.", func(t *testing.T) { whenAccountsListApplications(t) })
	t.Run("Quando um developer tenta administrar.", func(t *testing.T) { whenDeveloperTriesToAdminister(t) })
	t.Run("Quando um membro é suspenso.", func(t *testing.T) { whenMemberIsSuspended(t) })
	t.Run("Quando a aplicação é apagada.", func(t *testing.T) { whenApplicationIsDeleted(t) })
	t.Run("Quando a auditoria falha.", func(t *testing.T) { whenAuditFails(t) })
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

func TestMembers(t *testing.T) {
	t.Run("Quando o admin lista os membros.", func(t *testing.T) { whenAdminListsMembers(t) })
	t.Run("Quando o developer olha o próprio membro.", func(t *testing.T) { whenDeveloperLooksAtHimself(t) })
	t.Run("Quando o papel de um membro muda.", func(t *testing.T) { whenMemberRoleChanges(t) })
	t.Run("Quando o último admin é rebaixado ou removido.", func(t *testing.T) { whenLastAdminIsDemotedOrRemoved(t) })
	t.Run("Quando um membro é suspenso.", func(t *testing.T) { whenAMemberIsSuspendedByAdmin(t) })
	t.Run("Quando um membro é removido.", func(t *testing.T) { whenAMemberIsRemoved(t) })
}

func TestSecrets(t *testing.T) {
	t.Run("Quando o segredo é criado.", func(t *testing.T) { whenSecretIsCreated(t) })
	t.Run("Quando o valor nunca aparece fora do reveal.", func(t *testing.T) { whenValueNeverLeaksOutsideReveal(t) })
	t.Run("Quando o valor é revelado.", func(t *testing.T) { whenValueIsRevealed(t) })
	t.Run("Quando o valor muda.", func(t *testing.T) { whenValueChanges(t) })
	t.Run("Quando cada papel acessa os segredos.", func(t *testing.T) { whenRolesAccessSecrets(t) })
	t.Run("Quando o corpo é inválido.", func(t *testing.T) { whenSecretBodyIsInvalid(t) })
	t.Run("Quando há referência entre segredos.", func(t *testing.T) { whenSecretsReferenceEachOther(t) })
	t.Run("Quando o nome muda ou repete.", func(t *testing.T) { whenSecretIsRenamed(t) })
	t.Run("Quando a auditoria falha no reveal.", func(t *testing.T) { whenAuditFailsOnReveal(t) })
	t.Run("Quando o ciphertext é trocado de linha.", func(t *testing.T) { whenCiphertextIsMoved(t) })
	t.Run("Quando o lote é misto.", func(t *testing.T) { whenBatchIsMixed(t) })
	t.Run("Quando o segredo é apagado.", func(t *testing.T) { whenSecretIsDeleted(t) })
}

func TestTokens(t *testing.T) {
	t.Run("Quando o token é criado.", func(t *testing.T) { whenTokenIsCreated(t) })
	t.Run("Quando cada papel cria tokens.", func(t *testing.T) { whenRolesCreateTokens(t) })
	t.Run("Quando o token é inválido na criação.", func(t *testing.T) { whenTokenInputIsInvalid(t) })
	t.Run("Quando o SDK lê o ambiente.", func(t *testing.T) { whenSDKReadsEnvironment(t) })
	t.Run("Quando a URI não é a do token.", func(t *testing.T) { whenURIDoesNotMatch(t) })
	t.Run("Quando o token é usado onde não deve.", func(t *testing.T) { whenTokenIsUsedWhereItShouldNot(t) })
	t.Run("Quando o token é revogado.", func(t *testing.T) { whenTokenIsRevoked(t) })
	t.Run("Quando o token deixa de valer por outro motivo.", func(t *testing.T) { whenTokenStopsWorking(t) })
	t.Run("Quando a auditoria falha na leitura do ambiente.", func(t *testing.T) { whenAuditFailsOnEnv(t) })
	t.Run("Quando a chave publishable tenta ler o ambiente.", func(t *testing.T) { whenPublishableKeyReadsEnvironment(t) })
	t.Run("Quando a chave é renomeada e rotacionada.", func(t *testing.T) { whenKeyIsRenamedAndRotated(t) })
}

func TestAuditLogs(t *testing.T) {
	t.Run("Quando o admin lê a trilha com cursor.", func(t *testing.T) { whenAdminReadsTrailWithCursor(t) })
	t.Run("Quando o developer lê a trilha.", func(t *testing.T) { whenDeveloperReadsTrail(t) })
	t.Run("Quando os filtros são inválidos.", func(t *testing.T) { whenAuditFiltersAreInvalid(t) })
	t.Run("Quando alguém tenta mexer na trilha.", func(t *testing.T) { whenSomeoneTriesToTamperTheTrail(t) })
}

func TestInternalGRPC(t *testing.T) {
	t.Run("Quando escuta.", func(t *testing.T) { whenGRPCListensOnLoopback(t) })
	t.Run("Quando o gRPC lê o ambiente com o token do SDK.", func(t *testing.T) { whenGRPCReadsEnvironment(t) })
	t.Run("Quando a credencial não é um token do SDK válido.", func(t *testing.T) { whenGRPCCredentialIsInvalid(t) })
	t.Run("Quando o uri_id não bate.", func(t *testing.T) { whenGRPCURIDoesNotMatch(t) })
	t.Run("Quando o token é revogado.", func(t *testing.T) { whenGRPCTokenIsRevoked(t) })
	t.Run("Quando a pessoa administra aplicações pelo gRPC.", func(t *testing.T) { whenGRPCPersonManagesApplications(t) })
	t.Run("Quando a credencial das aplicações não é um JWT válido.", func(t *testing.T) { whenGRPCApplicationCredentialIsInvalid(t) })
	t.Run("Quando outra conta tenta mexer na aplicação pelo gRPC.", func(t *testing.T) { whenGRPCAnotherAccountIsRefused(t) })
	t.Run("Quando o lote do gRPC é misto.", func(t *testing.T) { whenGRPCBatchIsMixed(t) })
	t.Run("Quando a mensagem do gRPC é grande demais.", func(t *testing.T) { whenGRPCMessageIsTooBig(t) })
	t.Run("Quando a equipe é montada pelo gRPC.", func(t *testing.T) { whenGRPCTeamIsAssembled(t) })
	t.Run("Quando os segredos são geridos pelo gRPC.", func(t *testing.T) { whenGRPCSecretsAreManaged(t) })
	t.Run("Quando um developer tenta a tabela de outro pelo gRPC.", func(t *testing.T) { whenGRPCDeveloperReachesAnotherTable(t) })
	t.Run("Quando o token criado pelo gRPC lê o ambiente.", func(t *testing.T) { whenGRPCTokenReadsEnvironment(t) })
	t.Run("Quando a trilha é lida pelo gRPC.", func(t *testing.T) { whenGRPCAuditIsRead(t) })
	t.Run("Quando o token do SDK tenta os serviços de pessoa.", func(t *testing.T) { whenGRPCSDKTokenReachesPersonServices(t) })
}

// Functions

func whenThereIsNoCredential(t *testing.T) {
	s := newStack(t)

	status, body := s.do(nil, http.MethodGet, "/api/v1/applications", nil)

	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Contains(t, body["error"].(map[string]any)["type"], "#unauthorized")
}

func whenPersonCreatesApplication(t *testing.T) {
	s := newStack(t)
	maria := s.person()

	status, body := s.do(&maria, http.MethodPost, "/api/v1/applications", map[string]any{"name": "minha-app"})

	require.Equal(t, http.StatusCreated, status)
	data := body["data"].(map[string]any)
	assert.Equal(t, "minha-app", data["name"])
	assert.Equal(t, maria.id.String(), data["created_by"], "o dono é o sub do JWT")
	assert.NotContains(t, data, "deleted_at")

	id := data["id"].(string)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM members WHERE application_id = ? AND user_id = ? AND role = 'admin' AND status = 'active'`, id, maria.id.String()), "o criador vira admin")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE application_id = ? AND action = 'application.created' AND outcome = 'allowed' AND actor_id = ?`, id, maria.id.String()))
}

func whenBodyTriesToSetTheOwner(t *testing.T) {
	s := newStack(t)
	maria, joao := s.person(), s.person()

	status, body := s.do(&maria, http.MethodPost, "/api/v1/applications", map[string]any{
		"name": "minha-app", "created_by": joao.id.String(), "creatorId": joao.id.String(),
	})

	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, maria.id.String(), body["data"].(map[string]any)["created_by"], "o que o cliente manda como dono é ignorado")
}

func whenAnotherAccountLooksAtTheApplication(t *testing.T) {
	s := newStack(t)
	maria, joao := s.person(), s.person()
	id := s.createApplication(&maria, "minha-app")

	existing, existingBody := s.do(&joao, http.MethodGet, "/api/v1/applications/"+id, nil)
	missing, missingBody := s.do(&joao, http.MethodGet, "/api/v1/applications/"+uuid.NewV7().String(), nil)

	assert.Equal(t, http.StatusNotFound, existing)
	assert.Equal(t, existing, missing, "quem não é membro não distingue o que existe do que não existe")
	assert.Equal(t, existingBody["error"].(map[string]any)["title"], missingBody["error"].(map[string]any)["title"])
	assert.NotContains(t, existingBody["error"].(map[string]any)["detail"], id, "o erro não ecoa o id")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND actor_id = ? AND application_id = ?`, joao.id.String(), id), "a tentativa fica na trilha")

	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		status, _ := s.do(&joao, method, "/api/v1/applications/"+id, map[string]any{"name": "invadida"})
		assert.Equal(t, http.StatusNotFound, status, method)
	}
	status, body := s.do(&maria, http.MethodGet, "/api/v1/applications/"+id, nil)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "minha-app", body["data"].(map[string]any)["name"], "nada mudou")
}

func whenAccountsListApplications(t *testing.T) {
	s := newStack(t)
	maria, joao, root, dev := s.person(), s.person(), s.person("admin"), s.person()
	s.createApplication(&maria, "app-da-maria")
	s.createApplication(&joao, "app-do-joao")

	_, mine := s.do(&maria, http.MethodGet, "/api/v1/applications", nil)
	_, all := s.do(&root, http.MethodGet, "/api/v1/applications", nil)

	assert.Len(t, mine["data"], 1)
	assert.Equal(t, "app-da-maria", mine["data"].([]any)[0].(map[string]any)["name"])
	assert.Len(t, all["data"], 2, "o platform admin vê todas")

	_, empty := s.do(&dev, http.MethodGet, "/api/v1/applications", nil)
	assert.Equal(t, []any{}, empty["data"], "lista vazia é um array, nunca um campo ausente")
	assert.Contains(t, empty, "page")
}

func whenDeveloperTriesToAdminister(t *testing.T) {
	s := newStack(t)
	maria, dev := s.person(), s.person()
	id := s.createApplication(&maria, "minha-app")
	_, err := s.db.Exec(`INSERT INTO members (id, application_id, user_id, uri_id, role, created_at) VALUES (?, ?, ?, 'uri-dev', 'developer', ?)`,
		uuid.NewV7().String(), id, dev.id.String(), time.Now().UTC())
	require.NoError(t, err)

	read, _ := s.do(&dev, http.MethodGet, "/api/v1/applications/"+id, nil)
	update, _ := s.do(&dev, http.MethodPut, "/api/v1/applications/"+id, map[string]any{"name": "renomeada"})
	remove, _ := s.do(&dev, http.MethodDelete, "/api/v1/applications/"+id, nil)

	assert.Equal(t, http.StatusOK, read, "developer lê a aplicação")
	assert.Equal(t, http.StatusForbidden, update, "mas é membro, então a recusa é 403 e não 404")
	assert.Equal(t, http.StatusForbidden, remove)
	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND actor_id = ?`, dev.id.String()))
}

func whenMemberIsSuspended(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	id := s.createApplication(&maria, "minha-app")

	before, _ := s.do(&maria, http.MethodGet, "/api/v1/applications/"+id, nil)
	_, err := s.db.Exec(`UPDATE members SET status = 'suspended' WHERE application_id = ?`, id)
	require.NoError(t, err)
	after, _ := s.do(&maria, http.MethodGet, "/api/v1/applications/"+id, nil)

	assert.Equal(t, http.StatusOK, before)
	assert.Equal(t, http.StatusNotFound, after, "o efeito é imediato, mesmo com o JWT ainda válido")
}

func whenApplicationIsDeleted(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	id := s.createApplication(&maria, "minha-app")
	_, err := s.db.Exec(`INSERT INTO api_keys (id, application_id, member_id, name, prefix, hash, created_by, created_at)
		SELECT 'tok', application_id, id, 'n', 'sk_vt_x', 'hash', user_id, ? FROM members WHERE application_id = ?`, time.Now().UTC(), id)
	require.NoError(t, err)

	status, _ := s.do(&maria, http.MethodDelete, "/api/v1/applications/"+id, nil)
	read, _ := s.do(&maria, http.MethodGet, "/api/v1/applications/"+id, nil)

	assert.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, http.StatusNotFound, read)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM api_keys WHERE deleted_at IS NOT NULL`), "os tokens são revogados na cascata")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM members WHERE deleted_at IS NOT NULL`))
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'application.deleted'`), "a trilha continua")

	again := s.createApplication(&maria, "minha-app")
	assert.NotEqual(t, id, again, "o nome da aplicação apagada volta a ficar livre")
}

func whenAuditFails(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	_, err := s.db.Exec(`ALTER TABLE audit_logs RENAME TO audit_logs_broken`)
	require.NoError(t, err)

	status, _ := s.do(&maria, http.MethodPost, "/api/v1/applications", map[string]any{"name": "minha-app"})

	assert.Equal(t, http.StatusInternalServerError, status, "sem auditoria a operação não acontece")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM applications`), "a aplicação foi desfeita junto")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM members`), "o membro também")
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
	_, err := s.db.Exec(`INSERT INTO api_keys (id, application_id, member_id, name, prefix, hash, created_by, created_at) VALUES ('t', ?, ?, 'n', 'sk_vt_x', 'h', 'u', ?)`, app, memberID, now)
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
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM api_keys WHERE id = 't' AND deleted_at IS NOT NULL`), "os tokens dele são revogados")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM secrets WHERE id = 's' AND deleted_at IS NOT NULL`), "e os segredos, apagados")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'member.removed' AND target_id = ?`, memberID))
}

func whenSecretIsCreated(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	status, body := s.do(&maria, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "DATABASE_URL", "type": "database_uri", "value": plain})

	require.Equal(t, http.StatusCreated, status)
	data := body["data"].(map[string]any)
	assert.Equal(t, "DATABASE_URL", data["name"])
	assert.Equal(t, "static", data["source"])
	assert.EqualValues(t, 1, data["version"])
	assert.NotContains(t, data, "value", "o valor não faz parte do segredo")
	assert.Equal(t, maria.id.String(), data["created_by"])
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM secret_versions WHERE secret_id = ? AND version = 1`, data["id"]))
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'secret.created' AND metadata LIKE '%DATABASE_URL%'`))
}

func whenValueNeverLeaksOutsideReveal(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	member := s.memberID(&maria, app)
	id := s.createSecret(&maria, app, member, "DATABASE_URL", "database_uri", plain)
	status, _ := s.do(&maria, http.MethodPut, secretsPath(app, member)+"/"+id, map[string]any{"value": plain + "?v=2"})
	require.Equal(t, http.StatusOK, status)

	for _, accept := range []string{"application/json", "application/x-yaml", "application/xml", "text/plain"} {
		for _, path := range []string{secretsPath(app, member), secretsPath(app, member) + "/" + id, secretsPath(app, member) + "/" + id + "/versions", "/api/v1/applications/" + app + "/members"} {
			rec := s.raw(&maria, http.MethodGet, path, accept, "")
			assert.NotContains(t, rec.Body.String(), "s3cr3t", "%s %s", accept, path)
		}
	}

	reveal := s.raw(&maria, http.MethodPost, secretsPath(app, member)+"/"+id+"/reveal", "text/plain", "")
	assert.Contains(t, reveal.Header().Get("Content-Type"), "application/json", "o reveal ignora o Accept: só JSON")
	assert.Contains(t, reveal.Body.String(), "s3cr3t")

	files := []string{s.dbPath, s.dbPath + "-wal"}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		assert.NotContains(t, string(raw), "s3cr3t", "o valor não pode estar em claro no arquivo do banco (%s)", file)
	}
	for _, table := range []string{"audit_logs", "secrets", "secret_versions", "outbox_events"} {
		assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM `+table+` WHERE CAST(`+columnsOf(table)+` AS TEXT) LIKE '%s3cr3t%'`), table)
	}
}

func whenValueIsRevealed(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	member := s.memberID(&maria, app)
	id := s.createSecret(&maria, app, member, "DATABASE_URL", "database_uri", plain)

	status, body := s.reveal(&maria, app, "me", id)

	require.Equal(t, http.StatusOK, status)
	data := body["data"].(map[string]any)
	assert.Equal(t, plain, data["value"])
	assert.EqualValues(t, 1, data["version"])
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'secret.revealed' AND actor_id = ? AND target_id = ?`, maria.id.String(), id))
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM audit_logs WHERE metadata LIKE '%s3cr3t%'`), "a trilha leva nome e versão, nunca o valor")
}

func whenValueChanges(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	member := s.memberID(&maria, app)
	id := s.createSecret(&maria, app, member, "API_TOKEN", "api_key", "token-v1")

	status, body := s.do(&maria, http.MethodPut, secretsPath(app, member)+"/"+id, map[string]any{"value": "token-v2"})
	require.Equal(t, http.StatusOK, status)
	assert.EqualValues(t, 2, body["data"].(map[string]any)["version"])

	_, current := s.reveal(&maria, app, member, id)
	_, old := s.do(&maria, http.MethodPost, secretsPath(app, member)+"/"+id+"/reveal", map[string]any{"version": 1})
	missing, _ := s.do(&maria, http.MethodPost, secretsPath(app, member)+"/"+id+"/reveal", map[string]any{"version": 9})
	negative, _ := s.do(&maria, http.MethodPost, secretsPath(app, member)+"/"+id+"/reveal", map[string]any{"version": -1})

	assert.Equal(t, "token-v2", current["data"].(map[string]any)["value"])
	assert.Equal(t, "token-v1", old["data"].(map[string]any)["value"], "as versões antigas ficam")
	assert.Equal(t, http.StatusNotFound, missing)
	assert.Equal(t, http.StatusBadRequest, negative)

	_, versions := s.do(&maria, http.MethodGet, secretsPath(app, member)+"/"+id+"/versions", nil)
	list := versions["data"].([]any)
	require.Len(t, list, 2)
	assert.EqualValues(t, 2, list[0].(map[string]any)["version"])
	assert.NotContains(t, list[0], "value")
}

func whenRolesAccessSecrets(t *testing.T) {
	s := newStack(t)
	maria, root, outsider := s.person(), s.person("admin"), s.person()
	app := s.createApplication(&maria, "minha-app")
	dev1, dev1Member := s.join(&maria, app, "developer")
	dev2, dev2Member := s.join(&maria, app, "developer")
	id1 := s.createSecret(&dev1, app, dev1Member, "DEV1_SECRET", "string", "valor-do-dev1")

	ownList, _ := s.do(&dev1, http.MethodGet, secretsPath(app, "me"), nil)
	otherList, _ := s.do(&dev2, http.MethodGet, secretsPath(app, dev1Member), nil)
	otherGet, _ := s.do(&dev2, http.MethodGet, secretsPath(app, dev1Member)+"/"+id1, nil)
	otherReveal, _ := s.reveal(&dev2, app, dev1Member, id1)
	otherWrite, _ := s.do(&dev2, http.MethodPost, secretsPath(app, dev1Member), map[string]any{"name": "X", "type": "string", "value": "v"})
	otherDelete, _ := s.do(&dev2, http.MethodDelete, secretsPath(app, dev1Member)+"/"+id1, nil)
	adminReveal, adminBody := s.reveal(&maria, app, dev1Member, id1)
	adminEdit, _ := s.do(&maria, http.MethodPut, secretsPath(app, dev1Member)+"/"+id1, map[string]any{"value": "editado-pelo-admin"})
	platformReveal, _ := s.reveal(&root, app, dev1Member, id1)
	outsiderList, _ := s.do(&outsider, http.MethodGet, secretsPath(app, dev1Member), nil)
	_, ownReveal := s.reveal(&dev1, app, "me", id1)
	crossApp := s.createApplication(&outsider, "outra-app")
	crossStatus, _ := s.do(&outsider, http.MethodGet, secretsPath(crossApp, dev1Member)+"/"+id1, nil)

	assert.Equal(t, http.StatusOK, ownList)
	assert.Equal(t, http.StatusForbidden, otherList, "um developer não lê a tabela de outro")
	assert.Equal(t, http.StatusForbidden, otherGet)
	assert.Equal(t, http.StatusForbidden, otherReveal)
	assert.Equal(t, http.StatusForbidden, otherWrite)
	assert.Equal(t, http.StatusForbidden, otherDelete)
	assert.Equal(t, http.StatusOK, adminReveal, "o admin acessa a de todo mundo")
	assert.Equal(t, "valor-do-dev1", adminBody["data"].(map[string]any)["value"])
	assert.Equal(t, http.StatusOK, adminEdit, "e edita")
	assert.Equal(t, http.StatusOK, platformReveal, "o platform admin também")
	assert.Equal(t, http.StatusNotFound, outsiderList, "quem não é membro não vê nada")
	assert.Equal(t, "editado-pelo-admin", ownReveal["data"].(map[string]any)["value"], "o dono vê a edição do admin")
	assert.Equal(t, http.StatusNotFound, crossStatus, "o id de uma aplicação não abre segredos de outra")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'secret.revealed' AND actor_id = ? AND metadata LIKE ?`, maria.id.String(), "%"+dev1Member+"%"), "o reveal do admin na tabela do dev fica na trilha")
	assert.GreaterOrEqual(t, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND actor_id = ?`, dev2.id.String()), 5)
	_ = dev2Member
}

func whenSecretBodyIsInvalid(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	path := secretsPath(app, "me")

	cases := map[string]map[string]any{
		"nome minúsculo":      {"name": "database_url", "type": "string", "value": "v"},
		"tipo desconhecido":   {"name": "A", "type": "nope", "value": "v"},
		"valor vazio":         {"name": "A", "type": "string", "value": ""},
		"url sem host":        {"name": "A", "type": "url", "value": "isso-não-é-url-com-segredo-xyz"},
		"origem não liberada": {"name": "A", "type": "string", "source": "generated", "value": "v"},
		"reference sem alvo":  {"name": "A", "type": "string", "source": "reference"},
		"campo desconhecido":  {"name": "A", "type": "string", "value": "v", "member_id": "x"},
		"dono forjado":        {"name": "A", "type": "string", "value": "v", "created_by": "x"},
	}
	for name, body := range cases {
		status, response := s.do(&maria, http.MethodPost, path, body)
		assert.Equal(t, http.StatusBadRequest, status, name)
		if raw, ok := body["value"].(string); ok && len(raw) > 3 {
			assert.NotContains(t, toJSON(response), raw, "o erro não repete o valor: "+name)
		}
	}

	tooBig := s.raw(&maria, http.MethodPost, path, "application/json", `{"name":"BIG","type":"string","value":"`+strings.Repeat("a", 40<<10)+`"}`)
	assert.NotEqual(t, http.StatusCreated, tooBig.Code, "valor grande demais")
	yaml := s.raw(&maria, http.MethodPost, path, "application/json", "name: A\ntype: string\nvalue: v\n")
	assert.Equal(t, http.StatusBadRequest, yaml.Code, "o corpo é só JSON")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM secrets`))
}

func whenSecretsReferenceEachOther(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	adminMember := s.memberID(&maria, app)
	dev1, dev1Member := s.join(&maria, app, "developer")
	dev2, _ := s.join(&maria, app, "developer")
	target := s.createSecret(&dev1, app, dev1Member, "SHARED_DB", "database_uri", plain)

	ownRef, ownBody := s.do(&dev1, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "ALIAS", "type": "database_uri", "source": "reference", "reference_secret_id": target})
	devRef, _ := s.do(&dev2, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "STOLEN", "type": "database_uri", "source": "reference", "reference_secret_id": target})
	adminRef, adminBody := s.do(&maria, http.MethodPost, secretsPath(app, adminMember), map[string]any{"name": "ADMIN_ALIAS", "type": "database_uri", "source": "reference", "reference_secret_id": target})
	require.Equal(t, http.StatusCreated, ownRef, "%v", ownBody)
	require.Equal(t, http.StatusCreated, adminRef, "%v", adminBody)
	alias := ownBody["data"].(map[string]any)["id"].(string)

	chain, _ := s.do(&maria, http.MethodPost, secretsPath(app, adminMember), map[string]any{"name": "CHAIN", "type": "database_uri", "source": "reference", "reference_secret_id": alias})
	wrongType, _ := s.do(&dev1, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "WRONG", "type": "string", "source": "reference", "reference_secret_id": target})
	withValue, _ := s.do(&dev1, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "BOTH", "type": "database_uri", "source": "reference", "reference_secret_id": target, "value": plain})
	missing, _ := s.do(&dev1, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "GHOST", "type": "string", "source": "reference", "reference_secret_id": "01a1b42b-9194-7c05-a341-fca9346bafad"})
	putOnRef, _ := s.do(&dev1, http.MethodPut, secretsPath(app, "me")+"/"+alias, map[string]any{"value": plain})

	assert.Equal(t, http.StatusBadRequest, devRef, "apontar para o segredo de outro dev é ler esse segredo")
	assert.Equal(t, http.StatusBadRequest, chain, "referência para referência não existe (e por isso não há ciclo)")
	assert.Equal(t, http.StatusBadRequest, wrongType)
	assert.Equal(t, http.StatusBadRequest, withValue)
	assert.Equal(t, http.StatusBadRequest, missing)
	assert.Equal(t, http.StatusBadRequest, putOnRef, "uma referência não tem valor próprio")

	_, resolved := s.reveal(&dev1, app, "me", alias)
	assert.Equal(t, plain, resolved["data"].(map[string]any)["value"], "o reveal da referência resolve para o alvo")

	blocked, _ := s.do(&dev1, http.MethodDelete, secretsPath(app, "me")+"/"+target, nil)
	assert.Equal(t, http.StatusConflict, blocked, "não se apaga o que outros apontam")
	gone, _ := s.do(&dev1, http.MethodDelete, secretsPath(app, "me")+"/"+alias, nil)
	assert.Equal(t, http.StatusNoContent, gone)
	_ = adminBody
	adminAlias := adminBody["data"].(map[string]any)["id"].(string)
	_, _ = s.do(&maria, http.MethodDelete, secretsPath(app, adminMember)+"/"+adminAlias, nil)
	freed, _ := s.do(&dev1, http.MethodDelete, secretsPath(app, "me")+"/"+target, nil)
	assert.Equal(t, http.StatusNoContent, freed, "sem referências, apaga")
}

func whenSecretIsRenamed(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	a := s.createSecret(&maria, app, "me", "FIRST", "string", "1")
	s.createSecret(&maria, app, "me", "SECOND", "string", "2")

	renamed, body := s.do(&maria, http.MethodPatch, secretsPath(app, "me")+"/"+a, map[string]any{"name": "THIRD"})
	conflict, _ := s.do(&maria, http.MethodPatch, secretsPath(app, "me")+"/"+a, map[string]any{"name": "SECOND"})
	invalid, _ := s.do(&maria, http.MethodPatch, secretsPath(app, "me")+"/"+a, map[string]any{"name": "minúsculo"})
	duplicate, _ := s.do(&maria, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "SECOND", "type": "string", "value": "x"})

	assert.Equal(t, http.StatusOK, renamed)
	assert.Equal(t, "THIRD", body["data"].(map[string]any)["name"])
	assert.Equal(t, http.StatusConflict, conflict)
	assert.Equal(t, http.StatusBadRequest, invalid)
	assert.Equal(t, http.StatusConflict, duplicate, "nome repetido: o único item falha com 409")
}

func whenAuditFailsOnReveal(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	id := s.createSecret(&maria, app, "me", "DATABASE_URL", "database_uri", plain)
	_, err := s.db.Exec(`ALTER TABLE audit_logs RENAME TO audit_logs_broken`)
	require.NoError(t, err)

	rec := s.raw(&maria, http.MethodPost, secretsPath(app, "me")+"/"+id+"/reveal", "application/json", "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code, "sem trilha, o valor não sai")
	assert.NotContains(t, rec.Body.String(), "s3cr3t")
}

func whenCiphertextIsMoved(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	a := s.createSecret(&maria, app, "me", "AAA", "string", "valor-de-a-nao-vazar")
	b := s.createSecret(&maria, app, "me", "BBB", "string", "valor-de-b")
	_, err := s.db.Exec(`UPDATE secret_versions SET ciphertext = (SELECT ciphertext FROM secret_versions WHERE secret_id = ?) WHERE secret_id = ?`, a, b)
	require.NoError(t, err)

	rec := s.raw(&maria, http.MethodPost, secretsPath(app, "me")+"/"+b+"/reveal", "application/json", "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code, "o ciphertext de uma linha não abre em outra")
	assert.NotContains(t, rec.Body.String(), "valor-de-a")
	assert.NotContains(t, rec.Body.String(), "decrypt", "o erro é genérico")
}

func whenBatchIsMixed(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	status, body := s.do(&maria, http.MethodPost, secretsPath(app, "me"), []map[string]any{
		{"name": "OK_ONE", "type": "string", "value": "1"},
		{"name": "bad", "type": "string", "value": "2"},
		{"name": "OK_TWO", "type": "url", "value": "https://example.com"},
	})

	assert.Equal(t, http.StatusMultiStatus, status)
	assert.Len(t, body["data"], 2)
	assert.Len(t, body["errors"], 1)
	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM secrets`))
}

func whenSecretIsDeleted(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	id := s.createSecret(&maria, app, "me", "TEMP", "string", "v")

	status, _ := s.do(&maria, http.MethodDelete, secretsPath(app, "me")+"/"+id, nil)
	get, _ := s.do(&maria, http.MethodGet, secretsPath(app, "me")+"/"+id, nil)
	again, _ := s.do(&maria, http.MethodDelete, secretsPath(app, "me")+"/"+id, nil)
	reveal, _ := s.reveal(&maria, app, "me", id)
	recreate, _ := s.do(&maria, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "TEMP", "type": "string", "value": "novo"})

	assert.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, http.StatusNotFound, get)
	assert.Equal(t, http.StatusNotFound, again)
	assert.Equal(t, http.StatusNotFound, reveal)
	assert.Equal(t, http.StatusCreated, recreate, "o nome apagado volta a ficar livre")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM secret_versions WHERE secret_id = ?`, id), "as versões cifradas ficam para a auditoria")
}

func whenTokenIsCreated(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	req := httptest.NewRequest(http.MethodPost, tokensPath(app, "me"), strings.NewReader(`{"name":"notebook","type":"secret"}`))
	req.Header.Set("Authorization", "Bearer "+maria.token)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", "a resposta com o token é sempre JSON")
	body := rec.Body.String()
	raw := strings.Split(strings.Split(body, `"key":"`)[1], `"`)[0]
	assert.True(t, strings.HasPrefix(raw, "sk_vt_"))
	assert.Contains(t, body, `"uri":"https://vault.example.com/`)
	assert.Contains(t, body, `"prefix":"`+raw[:entities.KeyDisplayLength]+`"`)

	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM api_keys WHERE hash = ?`, raw), "o banco guarda o hash, não o token")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM api_keys WHERE prefix = ?`, raw[:entities.KeyDisplayLength]))
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM audit_logs WHERE metadata LIKE ?`, "%"+raw+"%"), "a trilha nunca leva o token")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'api_key.created'`))

	status, list := s.do(&maria, http.MethodGet, tokensPath(app, "me"), nil)
	require.Equal(t, http.StatusOK, status)
	item := list["data"].([]any)[0].(map[string]any)
	assert.NotContains(t, item, "key", "a lista só mostra o prefixo")
	assert.Equal(t, "secret", item["type"])
	assert.Equal(t, "notebook", item["name"])
}

func whenRolesCreateTokens(t *testing.T) {
	s := newStack(t)
	maria, outsider := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, devMember := s.join(&maria, app, "developer")
	other, otherMember := s.join(&maria, app, "developer")

	own, _ := s.do(&dev, http.MethodPost, tokensPath(app, "me"), map[string]any{"name": "meu", "type": "secret"})
	forOther, _ := s.do(&dev, http.MethodPost, tokensPath(app, otherMember), map[string]any{"name": "roubado", "type": "secret"})
	byAdmin, _ := s.do(&maria, http.MethodPost, tokensPath(app, devMember), map[string]any{"name": "do-admin", "type": "secret"})
	hidden, _ := s.do(&outsider, http.MethodPost, tokensPath(app, devMember), map[string]any{"name": "x", "type": "secret"})
	listOther, _ := s.do(&other, http.MethodGet, tokensPath(app, devMember), nil)
	missing, _ := s.do(&maria, http.MethodPost, tokensPath(app, uuid.NewV7().String()), map[string]any{"name": "x", "type": "secret"})

	assert.Equal(t, http.StatusCreated, own)
	assert.Equal(t, http.StatusForbidden, forOther)
	assert.Equal(t, http.StatusCreated, byAdmin, "o admin gera token de qualquer membro")
	assert.Equal(t, http.StatusNotFound, hidden)
	assert.Equal(t, http.StatusForbidden, listOther)
	assert.Equal(t, http.StatusNotFound, missing)

	_, mine := s.do(&dev, http.MethodGet, tokensPath(app, "me"), nil)
	assert.Len(t, mine["data"], 2)
}

func whenTokenInputIsInvalid(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	member := s.memberID(&maria, app)
	_, err := s.db.Exec(`UPDATE members SET status = 'suspended' WHERE id = ?`, member)
	require.NoError(t, err)
	_, err = s.db.Exec(`UPDATE members SET status = 'active' WHERE id = ?`, member)
	require.NoError(t, err)

	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	far := time.Now().Add(20 * 365 * 24 * time.Hour).Format(time.RFC3339)
	good := time.Now().Add(time.Hour).Format(time.RFC3339)

	cases := map[string]map[string]any{
		"sem nome":              {"name": "", "type": "secret"},
		"nome com controle":     {"name": "a\nb", "type": "secret"},
		"nome longo":            {"name": strings.Repeat("a", 101), "type": "secret"},
		"vencimento no passado": {"name": "x", "type": "secret", "expires_at": past},
		"vencimento distante":   {"name": "x", "type": "secret", "expires_at": far},
		"campo desconhecido":    {"name": "x", "type": "secret", "member_id": member},
		"sem tipo":              {"name": "x"},
		"tipo inválido":         {"name": "x", "type": "admin"},
	}
	for name, body := range cases {
		status, _ := s.do(&maria, http.MethodPost, tokensPath(app, "me"), body)
		assert.Equal(t, http.StatusBadRequest, status, name)
	}
	ok, _ := s.do(&maria, http.MethodPost, tokensPath(app, "me"), map[string]any{"name": "x", "type": "secret", "expires_at": good})
	assert.Equal(t, http.StatusCreated, ok)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM api_keys`))

	_, err = s.db.Exec(`UPDATE members SET status = 'suspended' WHERE id = ?`, member)
	require.NoError(t, err)
	root := s.person("admin")
	suspended, _ := s.do(&root, http.MethodPost, tokensPath(app, member), map[string]any{"name": "x", "type": "secret"})
	assert.Equal(t, http.StatusConflict, suspended, "membro suspenso não recebe token novo")
}

func whenSDKReadsEnvironment(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	adminMember := s.memberID(&maria, app)
	dev, devMember := s.join(&maria, app, "developer")
	s.createSecret(&maria, app, adminMember, "ADMIN_ONLY", "string", "valor-do-admin")
	target := s.createSecret(&dev, app, devMember, "DATABASE_URL", "database_uri", plain)
	s.createSecret(&dev, app, devMember, "API_KEY", "api_key", "sk-123")
	status, _ := s.do(&dev, http.MethodPost, secretsPath(app, "me"), map[string]any{"name": "DB_ALIAS", "type": "database_uri", "source": "reference", "reference_secret_id": target})
	require.Equal(t, http.StatusCreated, status)
	_, _ = s.do(&dev, http.MethodPut, secretsPath(app, "me")+"/"+target, map[string]any{"value": plain + "?v=2"})
	token := s.newToken(&dev, app, "me", "ci")

	rec := s.env(token.token, token.uri, "", "application/x-yaml")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", "o runtime ignora o Accept")
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	body := rec.Body.String()
	assert.Contains(t, body, "DATABASE_URL")
	assert.Contains(t, body, "API_KEY")
	assert.Contains(t, body, "sk-123")
	assert.NotContains(t, body, "valor-do-admin", "o token do dev só lê a tabela do dev")
	assert.Equal(t, 2, strings.Count(body, "s3cr3t-P4ss"), "o alias resolve para o valor atual do alvo")
	assert.Contains(t, body, "?v=2")

	filtered := s.env(token.token, token.uri, "?names=API_KEY,NAO_EXISTE", "")
	assert.Equal(t, http.StatusOK, filtered.Code)
	assert.Contains(t, filtered.Body.String(), "API_KEY")
	assert.NotContains(t, filtered.Body.String(), "DATABASE_URL")

	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'env.read' AND token_id = ? AND actor_type = 'token'`, token.id))
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM audit_logs WHERE metadata LIKE '%s3cr3t%' OR metadata LIKE '%sk-123%'`), "a trilha leva ids, nunca valores")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM api_keys WHERE id = ? AND last_used_at IS NOT NULL`, token.id), "o último uso é registrado")

	empty := s.person()
	emptyApp := s.createApplication(&empty, "app-vazia")
	emptyToken := s.newToken(&empty, emptyApp, "me", "ci")
	none := s.env(emptyToken.token, emptyToken.uri, "", "")
	assert.Equal(t, http.StatusOK, none.Code)
	assert.Contains(t, none.Body.String(), `"variables":[]`, "sem segredos é uma lista vazia, não ausente")
}

func whenURIDoesNotMatch(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, _ := s.join(&maria, app, "developer")
	adminToken := s.newToken(&maria, app, "me", "a")
	devToken := s.newToken(&dev, app, "me", "d")

	crossed := s.env(devToken.token, adminToken.uri, "", "")
	garbage := s.env(devToken.token, "nao-existe", "", "")

	assert.Equal(t, http.StatusNotFound, crossed.Code, "o token do dev não abre a URI do admin")
	assert.Equal(t, http.StatusNotFound, garbage.Code)
	assert.JSONEq(t, stripInstance(crossed.Body.Bytes()), stripInstance(garbage.Body.Bytes()), "as duas respostas são iguais")
	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND token_id = ?`, devToken.id))
}

func whenPublishableKeyReadsEnvironment(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	s.createSecret(&maria, app, "me", "API_KEY", "api_key", "valor-secreto")
	public := s.newKey(&maria, app, "me", "web", "publishable")

	env := s.env(public.token, public.uri, "", "")
	asManager := s.raw(&person{token: public.token}, http.MethodGet, secretsPath(app, "me"), "", "")
	_, revealErr := vaultv1.NewVaultServiceClient(s.grpcConn()).ReadEnv(bearer(public.token), &vaultv1.ReadEnvRequest{UriId: public.uri})

	assert.True(t, strings.HasPrefix(public.token, "pk_vt_"))
	assert.Equal(t, http.StatusUnauthorized, env.Code, "a pk_vt_ nunca lê segredo")
	assert.NotContains(t, env.Body.String(), "valor-secreto")
	assert.Equal(t, http.StatusUnauthorized, asManager.Code)
	assert.Equal(t, codes.Unauthenticated, status.Code(revealErr), "nem pelo gRPC")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM api_keys WHERE id = ? AND last_used_at IS NOT NULL`, public.id))
}

func whenKeyIsRenamedAndRotated(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	old := s.newToken(&maria, app, "me", "ci")

	renamed, renamedBody := s.do(&maria, http.MethodPatch, keyPath(app, "me", "/"+old.id), map[string]any{"name": "deploy"})
	rotated, rotatedBody := s.do(&maria, http.MethodPost, keyPath(app, "me", "/"+old.id+"/rotate"), nil)
	require.Equal(t, http.StatusCreated, rotated, "%v", rotatedBody)
	next := rotatedBody["data"].(map[string]any)
	got, gotBody := s.do(&maria, http.MethodGet, keyPath(app, "me", "/"+old.id), nil)

	assert.Equal(t, http.StatusOK, renamed, "%v", renamedBody)
	assert.Equal(t, "deploy", next["name"], "a nova chave herda o nome")
	assert.NotEqual(t, old.id, next["id"])
	assert.Equal(t, http.StatusOK, s.env(next["key"].(string), old.uri, "", "").Code, "a nova vale")
	assert.Equal(t, http.StatusOK, s.env(old.token, old.uri, "", "").Code, "a antiga ainda vale durante a sobreposição")
	assert.Equal(t, http.StatusOK, got)
	assert.NotNil(t, gotBody["data"].(map[string]any)["expires_at"], "a antiga ganhou vencimento")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'api_key.rotated' AND target_id = ?`, old.id))
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'api_key.updated' AND target_id = ?`, old.id))
}

func whenTokenIsUsedWhereItShouldNot(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	token := s.newToken(&maria, app, "me", "a")

	asManager := s.raw(&person{token: token.token}, http.MethodGet, "/api/v1/applications", "", "")
	asManagerWrite := s.raw(&person{token: token.token}, http.MethodPost, secretsPath(app, "me"), "", `{"name":"A","type":"string","value":"v"}`)
	jwtOnRuntime := s.raw(&maria, http.MethodGet, "/api/v1/runtime/"+token.uri+"/env", "", "")
	random := s.env("sk_vt_"+strings.Repeat("A", 43), token.uri, "", "")
	short := s.env("sk_vt_", token.uri, "", "")
	anonymous := httptest.NewRecorder()
	s.handler.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/runtime/"+token.uri+"/env", nil))

	assert.Equal(t, http.StatusUnauthorized, asManager.Code, "o token do SDK não administra")
	assert.Equal(t, http.StatusUnauthorized, asManagerWrite.Code)
	assert.Equal(t, http.StatusUnauthorized, jwtOnRuntime.Code, "e o JWT não lê o runtime")
	assert.Equal(t, http.StatusUnauthorized, random.Code)
	assert.Equal(t, http.StatusUnauthorized, short.Code)
	assert.Equal(t, http.StatusUnauthorized, anonymous.Code)
	assert.Equal(t, random.Body.String(), short.Body.String(), "token errado não diz por quê")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM secrets`))
}

func whenTokenIsRevoked(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, devMember := s.join(&maria, app, "developer")
	other, _ := s.join(&maria, app, "developer")
	s.createSecret(&dev, app, "me", "A", "string", "valor")
	token := s.newToken(&dev, app, "me", "ci")
	again := s.newToken(&dev, app, "me", "outro")
	path := keyPath(app, devMember, "/"+token.id)

	before := s.env(token.token, token.uri, "", "")
	byOther, _ := s.do(&other, http.MethodDelete, path, nil)
	byAdmin, _ := s.do(&maria, http.MethodDelete, path, nil)
	after := s.env(token.token, token.uri, "", "")
	repeat, _ := s.do(&maria, http.MethodDelete, path, nil)
	wrongMember, _ := s.do(&maria, http.MethodDelete, keyPath(app, "me", "/"+again.id), nil)
	unknown, _ := s.do(&maria, http.MethodDelete, keyPath(app, devMember, "/"+uuid.NewV7().String()), nil)
	untouched := s.env(again.token, again.uri, "", "")

	assert.Equal(t, http.StatusOK, before.Code)
	assert.Equal(t, http.StatusForbidden, byOther, "um developer não revoga o token de outro")
	assert.Equal(t, http.StatusNoContent, byAdmin, "o admin revoga qualquer um")
	assert.Equal(t, http.StatusUnauthorized, after.Code, "revogado vale na hora")
	assert.Equal(t, http.StatusNoContent, repeat, "revogar de novo é idempotente")
	assert.Equal(t, http.StatusNotFound, wrongMember, "o token tem de ser do membro do caminho")
	assert.Equal(t, http.StatusNotFound, unknown)
	assert.Equal(t, http.StatusOK, untouched.Code, "os outros tokens do dev seguem valendo")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'api_key.revoked' AND target_id = ?`, token.id), "uma linha só, mesmo revogando duas vezes")

	_, list := s.do(&dev, http.MethodGet, tokensPath(app, "me"), nil)
	revoked := 0
	for _, item := range list["data"].([]any) {
		if item.(map[string]any)["deleted_at"] != nil {
			revoked++
		}
	}
	assert.Equal(t, 1, revoked, "o histórico mostra o revogado")
}

func whenTokenStopsWorking(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	expired := s.newToken(&maria, app, "me", "vence")
	suspended, suspendedMember := s.join(&maria, app, "developer")
	suspendedToken := s.newToken(&suspended, app, "me", "s")
	removed, removedMember := s.join(&maria, app, "developer")
	removedToken := s.newToken(&removed, app, "me", "r")
	_, _, _ = suspended, removed, expired

	_, err := s.db.Exec(`UPDATE api_keys SET expires_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Second), expired.id)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, s.env(expired.token, expired.uri, "", "").Code, "vencido")

	assert.Equal(t, http.StatusOK, s.env(suspendedToken.token, suspendedToken.uri, "", "").Code)
	status, _ := s.do(&maria, http.MethodPatch, "/api/v1/applications/"+app+"/members/"+suspendedMember, map[string]any{"status": "suspended"})
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, http.StatusUnauthorized, s.env(suspendedToken.token, suspendedToken.uri, "", "").Code, "membro suspenso")

	status, _ = s.do(&maria, http.MethodDelete, "/api/v1/applications/"+app+"/members/"+removedMember, nil)
	require.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, http.StatusUnauthorized, s.env(removedToken.token, removedToken.uri, "", "").Code, "membro removido")

	live := s.newToken(&maria, app, "me", "vivo")
	assert.Equal(t, http.StatusOK, s.env(live.token, live.uri, "", "").Code)
	status, _ = s.do(&maria, http.MethodDelete, "/api/v1/applications/"+app, nil)
	require.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, http.StatusUnauthorized, s.env(live.token, live.uri, "", "").Code, "aplicação apagada")
}

func whenAuditFailsOnEnv(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	s.createSecret(&maria, app, "me", "DATABASE_URL", "database_uri", plain)
	token := s.newToken(&maria, app, "me", "ci")
	_, err := s.db.Exec(`ALTER TABLE audit_logs RENAME TO audit_logs_broken`)
	require.NoError(t, err)

	rec := s.env(token.token, token.uri, "", "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code, "sem trilha, o ambiente não sai")
	assert.NotContains(t, rec.Body.String(), "s3cr3t")
}

func whenAdminReadsTrailWithCursor(t *testing.T) {
	s := newStack(t)
	maria, joao := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	for i := 0; i < 5; i++ {
		s.createSecret(&maria, app, "me", "KEY_"+strings.Repeat("A", i+1), "string", "v")
	}
	_, _ = s.do(&joao, http.MethodGet, "/api/v1/applications/"+app, nil)

	total := s.count(`SELECT COUNT(*) FROM audit_logs WHERE application_id = ?`, app)
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		query := "?limit=3"
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		status, body := s.do(&maria, http.MethodGet, auditPath(app, query), nil)
		require.Equal(t, http.StatusOK, status, "%v", body)
		for _, item := range body["data"].([]any) {
			id := item.(map[string]any)["id"].(string)
			assert.False(t, seen[id], "uma linha nunca aparece em duas páginas")
			seen[id] = true
		}
		pages++
		next := body["page"].(map[string]any)["next_cursor"]
		if next == nil {
			break
		}
		cursor = next.(string)
		require.Less(t, pages, 20)
	}
	assert.Equal(t, total, len(seen), "a paginação por cursor percorre a trilha inteira")
	assert.GreaterOrEqual(t, pages, 3)

	_, denied := s.do(&maria, http.MethodGet, auditPath(app, "?outcome=denied"), nil)
	assert.Len(t, denied["data"], 1, "a tentativa do outro fica na trilha da aplicação")
	_, created := s.do(&maria, http.MethodGet, auditPath(app, "?action=secret.created&limit=200"), nil)
	assert.Len(t, created["data"], 5)
	first := created["data"].([]any)[0].(map[string]any)
	assert.Equal(t, "secret", first["target_type"])
	assert.NotContains(t, toJSON(first["metadata"]), "valor")

	hidden, _ := s.do(&joao, http.MethodGet, auditPath(app, ""), nil)
	assert.Equal(t, http.StatusNotFound, hidden, "quem não é membro não lê a trilha")
}

func whenDeveloperReadsTrail(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, _ := s.join(&maria, app, "developer")
	s.createSecret(&dev, app, "me", "MINE", "string", "v")
	s.createSecret(&maria, app, "me", "ADMINS", "string", "v")

	_, own := s.do(&dev, http.MethodGet, auditPath(app, ""), nil)
	_, forged := s.do(&dev, http.MethodGet, auditPath(app, "?actor_id="+maria.id.String()), nil)

	for _, body := range []map[string]any{own, forged} {
		require.NotEmpty(t, body["data"])
		for _, item := range body["data"].([]any) {
			assert.Equal(t, dev.id.String(), item.(map[string]any)["actor_id"], "o developer só enxerga o que ele mesmo fez")
		}
	}
}

func whenAuditFiltersAreInvalid(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	for _, query := range []string{"?limit=0", "?limit=x", "?cursor=nao-e-uuid", "?actor_id=x", "?outcome=talvez", "?from=ontem", "?to=2020", "?action=" + strings.Repeat("a", 65)} {
		status, _ := s.do(&maria, http.MethodGet, auditPath(app, query), nil)
		assert.Equal(t, http.StatusBadRequest, status, query)
	}
	status, _ := s.do(&maria, http.MethodGet, auditPath(app, "?limit=9999"), nil)
	assert.Equal(t, http.StatusOK, status, "limite acima do teto é aparado, não é erro")
	status, _ = s.do(&maria, http.MethodGet, auditPath(app, "?from=2020-01-01T00:00:00Z&to=2099-01-01T00:00:00Z"), nil)
	assert.Equal(t, http.StatusOK, status)
}

func whenSomeoneTriesToTamperTheTrail(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		status, _ := s.do(&maria, method, auditPath(app, ""), map[string]any{"action": "x"})
		assert.Equal(t, http.StatusMethodNotAllowed, status, "a API não tem rota de escrita na trilha: %s", method)
	}
	_, err := s.db.Exec(`UPDATE audit_logs SET outcome = 'denied'`)
	assert.Error(t, err, "e o banco recusa")
	_, err = s.db.Exec(`DELETE FROM audit_logs`)
	assert.Error(t, err)
}

func whenGRPCListensOnLoopback(t *testing.T) {
	_, addr := newStack(t).grpcClient()

	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"), "o gRPC interno escuta só em loopback")
}

func whenGRPCReadsEnvironment(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	adminMember := s.memberID(&maria, app)
	s.createSecret(&maria, app, adminMember, "DATABASE_URL", "string", "valor-secreto")
	s.createSecret(&maria, app, adminMember, "API_KEY", "api_key", "sk-123")
	token := s.newToken(&maria, app, "me", "ci")
	client, _ := s.grpcClient()

	all, err := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri})
	filtered, filteredErr := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri, Names: []string{"API_KEY"}})

	require.NoError(t, err)
	assert.Equal(t, token.uri, all.UriId)
	values := map[string]string{}
	for _, variable := range all.Variables {
		values[variable.Name] = variable.Value
	}
	assert.Equal(t, map[string]string{"DATABASE_URL": "valor-secreto", "API_KEY": "sk-123"}, values)
	require.NoError(t, filteredErr)
	require.Len(t, filtered.Variables, 1)
	assert.Equal(t, "API_KEY", filtered.Variables[0].Name)
	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'env.read' AND token_id = ?`, token.id), "cada leitura deixa rastro")
}

func whenGRPCCredentialIsInvalid(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	token := s.newToken(&maria, app, "me", "ci")
	client, _ := s.grpcClient()
	request := &vaultv1.ReadEnvRequest{UriId: token.uri}

	_, anonymous := client.ReadEnv(context.Background(), request)
	_, garbage := client.ReadEnv(bearer("sk_vt_"+strings.Repeat("A", 43)), request)
	_, person := client.ReadEnv(bearer(maria.token), request)
	_, wrongScheme := client.ReadEnv(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Basic "+token.token), request)

	assert.Equal(t, codes.Unauthenticated, status.Code(anonymous))
	assert.Equal(t, codes.Unauthenticated, status.Code(garbage))
	assert.Equal(t, codes.Unauthenticated, status.Code(person), "o JWT de pessoa não lê o runtime")
	assert.Equal(t, codes.Unauthenticated, status.Code(wrongScheme))
}

func whenGRPCURIDoesNotMatch(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, _ := s.join(&maria, app, "developer")
	adminToken := s.newToken(&maria, app, "me", "a")
	devToken := s.newToken(&dev, app, "me", "d")
	client, _ := s.grpcClient()

	_, crossed := client.ReadEnv(bearer(devToken.token), &vaultv1.ReadEnvRequest{UriId: adminToken.uri})

	assert.Equal(t, codes.NotFound, status.Code(crossed), "o token do dev não abre a URI do admin")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND token_id = ?`, devToken.id))
}

func whenGRPCTokenIsRevoked(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	token := s.newToken(&maria, app, "me", "ci")
	client, _ := s.grpcClient()
	_, err := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri})
	require.NoError(t, err)

	_, execErr := s.db.Exec(`UPDATE api_keys SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, token.id)
	require.NoError(t, execErr)
	_, afterRevoke := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri})

	assert.Equal(t, codes.Unauthenticated, status.Code(afterRevoke), "token revogado perde acesso na hora")
}

func whenGRPCPersonManagesApplications(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	client := s.grpcApplications()
	ctx := bearer(maria.token)

	created, err := client.CreateApplications(ctx, &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "minha-app"}}})
	require.NoError(t, err)
	require.Len(t, created.GetApplications(), 1)
	app := created.GetApplications()[0]
	read, readErr := client.GetApplication(ctx, &vaultv1.GetApplicationRequest{Id: app.GetId()})
	listed, listErr := client.ListApplications(ctx, &vaultv1.ListApplicationsRequest{})
	updated, updateErr := client.UpdateApplications(ctx, &vaultv1.UpdateApplicationsRequest{Applications: []*vaultv1.ApplicationChange{{Id: app.GetId(), Name: "renomeada"}}})
	deleted, deleteErr := client.DeleteApplications(ctx, &vaultv1.DeleteApplicationsRequest{Ids: []string{app.GetId()}})
	_, afterDelete := client.GetApplication(ctx, &vaultv1.GetApplicationRequest{Id: app.GetId()})

	assert.Equal(t, maria.id.String(), app.GetCreatedBy(), "o dono é o sub do JWT")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM members WHERE application_id = ? AND user_id = ? AND role = 'admin'`, app.GetId(), maria.id.String()), "o criador vira admin")
	require.NoError(t, readErr)
	assert.Equal(t, "minha-app", read.GetName())
	require.NoError(t, listErr)
	assert.Len(t, listed.GetApplications(), 1)
	assert.Equal(t, int32(1), listed.GetPage().GetTotal())
	require.NoError(t, updateErr)
	if assert.Len(t, updated.GetApplications(), 1) {
		assert.Equal(t, "renomeada", updated.GetApplications()[0].GetName())
	}
	require.NoError(t, deleteErr)
	assert.Equal(t, []string{app.GetId()}, deleted.GetIds())
	assert.Equal(t, codes.NotFound, status.Code(afterDelete))
	for _, action := range []string{"application.created", "application.updated", "application.deleted"} {
		assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = ? AND actor_id = ?`, action, maria.id.String()), action)
	}
}

func whenGRPCApplicationCredentialIsInvalid(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	token := s.newToken(&maria, app, "me", "ci")
	client := s.grpcApplications()
	request := &vaultv1.ListApplicationsRequest{}

	_, anonymous := client.ListApplications(context.Background(), request)
	_, sdkToken := client.ListApplications(bearer(token.token), request)
	_, garbage := client.ListApplications(bearer("nao.e.jwt"), request)
	_, wrongScheme := client.ListApplications(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Basic "+maria.token), request)
	_, twice := client.ListApplications(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+maria.token, "authorization", "Bearer "+maria.token), request)
	_, sdkDelete := client.DeleteApplications(bearer(token.token), &vaultv1.DeleteApplicationsRequest{Ids: []string{app}})

	assert.Equal(t, codes.Unauthenticated, status.Code(anonymous))
	assert.Equal(t, codes.Unauthenticated, status.Code(sdkToken), "o token do SDK não administra nada")
	assert.Equal(t, codes.Unauthenticated, status.Code(garbage))
	assert.Equal(t, codes.Unauthenticated, status.Code(wrongScheme))
	assert.Equal(t, codes.Unauthenticated, status.Code(twice), "credencial ambígua é recusada")
	assert.Equal(t, codes.Unauthenticated, status.Code(sdkDelete))
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM applications WHERE deleted_at IS NULL`), "nada foi apagado")
}

func whenGRPCAnotherAccountIsRefused(t *testing.T) {
	s := newStack(t)
	maria, joao := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	client := s.grpcApplications()
	ctx := bearer(joao.token)

	_, existing := client.GetApplication(ctx, &vaultv1.GetApplicationRequest{Id: app})
	_, missing := client.GetApplication(ctx, &vaultv1.GetApplicationRequest{Id: uuid.NewV7().String()})
	_, update := client.UpdateApplications(ctx, &vaultv1.UpdateApplicationsRequest{Applications: []*vaultv1.ApplicationChange{{Id: app, Name: "invadida"}}})
	_, remove := client.DeleteApplications(ctx, &vaultv1.DeleteApplicationsRequest{Ids: []string{app}})
	listed, listErr := client.ListApplications(ctx, &vaultv1.ListApplicationsRequest{})

	assert.Equal(t, codes.NotFound, status.Code(existing))
	assert.Equal(t, status.Convert(existing).Message(), status.Convert(missing).Message(), "quem não é membro não distingue o que existe do que não existe")
	assert.Equal(t, codes.NotFound, status.Code(update))
	assert.Equal(t, codes.NotFound, status.Code(remove))
	require.NoError(t, listErr)
	assert.Empty(t, listed.GetApplications(), "a lista só traz as aplicações da própria conta")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM applications WHERE name = 'minha-app' AND deleted_at IS NULL`), "nada mudou")
	assert.Equal(t, 3, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND actor_id = ? AND application_id = ?`, joao.id.String(), app), "as tentativas ficam na trilha")
}

func whenGRPCBatchIsMixed(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	client := s.grpcApplications()

	mixed, mixedErr := client.CreateApplications(bearer(maria.token), &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "app-valida"}, {Name: "Nome Inválido"}}})
	_, allFail := client.CreateApplications(bearer(maria.token), &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: "Nome Inválido"}}})
	_, empty := client.CreateApplications(bearer(maria.token), &vaultv1.CreateApplicationsRequest{})

	require.NoError(t, mixedErr, "com um sucesso, a chamada é OK e os erros vêm na resposta")
	assert.Len(t, mixed.GetApplications(), 1)
	if assert.Len(t, mixed.GetErrors(), 1) {
		assert.Equal(t, int32(http.StatusBadRequest), mixed.GetErrors()[0].GetStatus())
	}
	assert.Equal(t, codes.InvalidArgument, status.Code(allFail))
	assert.Equal(t, codes.InvalidArgument, status.Code(empty))
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM applications`))
}

func whenGRPCMessageIsTooBig(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	client := s.grpcApplications()

	_, err := client.CreateApplications(bearer(maria.token), &vaultv1.CreateApplicationsRequest{Applications: []*vaultv1.NewApplication{{Name: strings.Repeat("a", 128<<10)}}})

	assert.Equal(t, codes.ResourceExhausted, status.Code(err), "o teto da mensagem é o mesmo do corpo HTTP")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM applications`))
}

func whenGRPCTeamIsAssembled(t *testing.T) {
	s := newStack(t)
	maria, joao := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	conn := s.grpcConn()
	invitations, members := vaultv1.NewInvitationServiceClient(conn), vaultv1.NewMemberServiceClient(conn)
	admin := bearer(maria.token)

	created, createErr := invitations.CreateInvitation(admin, &vaultv1.CreateInvitationRequest{ApplicationId: app, Email: "joao@example.com", Role: "developer"})
	require.NoError(t, createErr)
	pending, pendingErr := invitations.ListInvitations(admin, &vaultv1.ListInvitationsRequest{ApplicationId: app})
	joined, acceptErr := invitations.AcceptInvitation(bearer(joao.token), &vaultv1.AcceptInvitationRequest{Token: created.GetToken()})
	require.NoError(t, acceptErr)
	_, reused := invitations.AcceptInvitation(bearer(joao.token), &vaultv1.AcceptInvitationRequest{Token: created.GetToken()})
	listed, listErr := members.ListMembers(admin, &vaultv1.ListMembersRequest{ApplicationId: app})
	me, meErr := members.GetMe(bearer(joao.token), &vaultv1.GetMeRequest{ApplicationId: app})
	role := "admin"
	_, selfPromotion := members.UpdateMember(bearer(joao.token), &vaultv1.UpdateMemberRequest{ApplicationId: app, MemberId: joined.GetId(), Role: &role})
	promoted, promoteErr := members.UpdateMember(admin, &vaultv1.UpdateMemberRequest{ApplicationId: app, MemberId: joined.GetId(), Role: &role})
	_, removeErr := members.RemoveMember(admin, &vaultv1.RemoveMemberRequest{ApplicationId: app, MemberId: joined.GetId()})
	_, afterRemoval := members.GetMe(bearer(joao.token), &vaultv1.GetMeRequest{ApplicationId: app})

	assert.NotEmpty(t, created.GetToken())
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM invitations WHERE token_hash = ?`, created.GetToken()), "o banco guarda o hash, não o token")
	require.NoError(t, pendingErr)
	if assert.Len(t, pending.GetInvitations(), 1) {
		assert.Equal(t, "pending", pending.GetInvitations()[0].GetStatus())
	}
	assert.Equal(t, "developer", joined.GetRole())
	assert.Equal(t, joao.id.String(), joined.GetUserId(), "o vínculo é da conta do JWT")
	assert.Equal(t, codes.NotFound, status.Code(reused), "convite usado responde igual a convite inexistente")
	require.NoError(t, listErr)
	assert.Len(t, listed.GetMembers(), 2)
	require.NoError(t, meErr)
	assert.Equal(t, joined.GetId(), me.GetId())
	assert.Equal(t, codes.PermissionDenied, status.Code(selfPromotion), "um developer não se promove")
	require.NoError(t, promoteErr)
	assert.Equal(t, "admin", promoted.GetRole())
	require.NoError(t, removeErr)
	assert.Equal(t, codes.NotFound, status.Code(afterRemoval), "removido, perde o acesso na hora")
}

func whenGRPCSecretsAreManaged(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	client := vaultv1.NewSecretServiceClient(s.grpcConn())
	ctx := bearer(maria.token)

	created, createErr := client.CreateSecrets(ctx, &vaultv1.CreateSecretsRequest{ApplicationId: app, MemberId: "me", Items: []*vaultv1.SecretDraft{
		{Name: "DATABASE_URL", Type: "string", Source: "static", Value: plain},
		{Name: "nome invalido", Type: "string", Source: "static", Value: "outro-valor"},
	}})
	require.NoError(t, createErr)
	require.Len(t, created.GetSecrets(), 1)
	id := created.GetSecrets()[0].GetId()
	request := &vaultv1.SecretRequest{ApplicationId: app, MemberId: "me", SecretId: id}
	listed, listErr := client.ListSecrets(ctx, &vaultv1.ListSecretsRequest{ApplicationId: app, MemberId: "me"})
	got, getErr := client.GetSecret(ctx, request)
	updated, updateErr := client.UpdateSecret(ctx, &vaultv1.UpdateSecretRequest{ApplicationId: app, MemberId: "me", SecretId: id, Value: "valor-novo"})
	renamed, renameErr := client.RenameSecret(ctx, &vaultv1.RenameSecretRequest{ApplicationId: app, MemberId: "me", SecretId: id, Name: "DB_URL"})
	versions, versionsErr := client.ListSecretVersions(ctx, request)
	current, currentErr := client.RevealSecret(ctx, &vaultv1.RevealSecretRequest{ApplicationId: app, MemberId: "me", SecretId: id})
	old, oldErr := client.RevealSecret(ctx, &vaultv1.RevealSecretRequest{ApplicationId: app, MemberId: "me", SecretId: id, Version: 1})
	_, deleteErr := client.DeleteSecret(ctx, request)
	_, afterDelete := client.GetSecret(ctx, request)

	assert.Len(t, created.GetErrors(), 1, "o item inválido vira um erro e não impede o outro")
	for _, response := range []string{created.String(), listed.String(), got.String(), updated.String(), renamed.String(), versions.String()} {
		assert.NotContains(t, response, plain, "só o reveal devolve valor")
		assert.NotContains(t, response, "valor-novo")
	}
	require.NoError(t, listErr)
	assert.Len(t, listed.GetSecrets(), 1)
	require.NoError(t, getErr)
	assert.Equal(t, "DATABASE_URL", got.GetName())
	require.NoError(t, updateErr)
	assert.Equal(t, int32(2), updated.GetVersion())
	require.NoError(t, renameErr)
	assert.Equal(t, "DB_URL", renamed.GetName())
	require.NoError(t, versionsErr)
	assert.Len(t, versions.GetVersions(), 2)
	require.NoError(t, currentErr)
	assert.Equal(t, "valor-novo", current.GetValue())
	require.NoError(t, oldErr)
	assert.Equal(t, plain, old.GetValue())
	require.NoError(t, deleteErr)
	assert.Equal(t, codes.NotFound, status.Code(afterDelete))
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM secret_versions WHERE CAST(ciphertext AS TEXT) LIKE ?`, "%"+plain+"%"), "o valor fica cifrado no banco")
	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'secret.revealed' AND actor_id = ?`, maria.id.String()), "cada reveal deixa rastro")
}

func whenGRPCDeveloperReachesAnotherTable(t *testing.T) {
	s := newStack(t)
	maria, outsider := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	dev1, dev1Member := s.join(&maria, app, "developer")
	dev2, _ := s.join(&maria, app, "developer")
	id := s.createSecret(&dev1, app, dev1Member, "DEV1_SECRET", "string", "valor-do-dev1")
	client := vaultv1.NewSecretServiceClient(s.grpcConn())
	request := &vaultv1.SecretRequest{ApplicationId: app, MemberId: dev1Member, SecretId: id}
	reveal := &vaultv1.RevealSecretRequest{ApplicationId: app, MemberId: dev1Member, SecretId: id}

	_, otherList := client.ListSecrets(bearer(dev2.token), &vaultv1.ListSecretsRequest{ApplicationId: app, MemberId: dev1Member})
	_, otherGet := client.GetSecret(bearer(dev2.token), request)
	otherRevealed, otherReveal := client.RevealSecret(bearer(dev2.token), reveal)
	_, otherWrite := client.UpdateSecret(bearer(dev2.token), &vaultv1.UpdateSecretRequest{ApplicationId: app, MemberId: dev1Member, SecretId: id, Value: "invadido"})
	_, otherDelete := client.DeleteSecret(bearer(dev2.token), request)
	_, outsiderReveal := client.RevealSecret(bearer(outsider.token), reveal)
	own, ownErr := client.RevealSecret(bearer(dev1.token), &vaultv1.RevealSecretRequest{ApplicationId: app, MemberId: "me", SecretId: id})
	adminRevealed, adminErr := client.RevealSecret(bearer(maria.token), reveal)

	for _, err := range []error{otherList, otherGet, otherReveal, otherWrite, otherDelete} {
		assert.Equal(t, codes.PermissionDenied, status.Code(err), "um developer não toca a tabela de outro")
		assert.NotContains(t, status.Convert(err).Message(), "valor-do-dev1")
	}
	assert.Nil(t, otherRevealed)
	assert.Equal(t, codes.NotFound, status.Code(outsiderReveal), "quem não é membro não distingue o que existe")
	require.NoError(t, ownErr)
	assert.Equal(t, "valor-do-dev1", own.GetValue())
	require.NoError(t, adminErr)
	assert.Equal(t, "valor-do-dev1", adminRevealed.GetValue(), "o admin acessa a de todo mundo")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM secrets WHERE id = ? AND deleted_at IS NULL`, id), "nada foi apagado")
}

func whenGRPCTokenReadsEnvironment(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	s.createSecret(&maria, app, s.memberID(&maria, app), "API_KEY", "api_key", "sk-123")
	conn := s.grpcConn()
	tokens, runtime := vaultv1.NewApiKeyServiceClient(conn), vaultv1.NewVaultServiceClient(conn)
	ctx := bearer(maria.token)

	created, createErr := tokens.CreateApiKey(ctx, &vaultv1.CreateApiKeyRequest{ApplicationId: app, MemberId: "me", Name: "ci", Type: "secret"})
	require.NoError(t, createErr)
	listed, listErr := tokens.ListApiKeys(ctx, &vaultv1.ListApiKeysRequest{ApplicationId: app, MemberId: "me"})
	env, envErr := runtime.ReadEnv(bearer(created.GetKey()), &vaultv1.ReadEnvRequest{UriId: created.GetApiKey().GetUriId()})
	_, revokeErr := tokens.DeleteApiKey(ctx, &vaultv1.ApiKeyRequest{ApplicationId: app, MemberId: "me", Id: created.GetApiKey().GetId()})
	_, afterRevoke := runtime.ReadEnv(bearer(created.GetKey()), &vaultv1.ReadEnvRequest{UriId: created.GetApiKey().GetUriId()})

	assert.True(t, strings.HasPrefix(created.GetKey(), "sk_vt_"))
	assert.Contains(t, created.GetUri(), "https://vault.example.com")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM api_keys WHERE hash = ?`, created.GetKey()), "o banco guarda o hash, não o token")
	require.NoError(t, listErr)
	if assert.Len(t, listed.GetApiKeys(), 1) {
		assert.NotContains(t, listed.String(), created.GetKey(), "a listagem só traz o prefixo")
	}
	require.NoError(t, envErr)
	if assert.Len(t, env.GetVariables(), 1) {
		assert.Equal(t, "sk-123", env.GetVariables()[0].GetValue())
	}
	require.NoError(t, revokeErr)
	assert.Equal(t, codes.Unauthenticated, status.Code(afterRevoke), "revogado pelo gRPC, perde o acesso na hora")
}

func whenGRPCAuditIsRead(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, _ := s.join(&maria, app, "developer")
	other := s.person()
	client := vaultv1.NewAuditServiceClient(s.grpcConn())

	all, allErr := client.ListAuditLogs(bearer(maria.token), &vaultv1.ListAuditLogsRequest{ApplicationId: app})
	firstPage, firstErr := client.ListAuditLogs(bearer(maria.token), &vaultv1.ListAuditLogsRequest{ApplicationId: app, Limit: 1})
	require.NoError(t, firstErr)
	secondPage, secondErr := client.ListAuditLogs(bearer(maria.token), &vaultv1.ListAuditLogsRequest{ApplicationId: app, Limit: 1, Cursor: firstPage.GetNextCursor()})
	created, createdErr := client.ListAuditLogs(bearer(maria.token), &vaultv1.ListAuditLogsRequest{ApplicationId: app, Action: "application.created"})
	own, ownErr := client.ListAuditLogs(bearer(dev.token), &vaultv1.ListAuditLogsRequest{ApplicationId: app})
	_, outsider := client.ListAuditLogs(bearer(other.token), &vaultv1.ListAuditLogsRequest{ApplicationId: app})

	require.NoError(t, allErr)
	assert.GreaterOrEqual(t, len(all.GetLogs()), 3, "criação, convite e aceite")
	assert.Len(t, firstPage.GetLogs(), 1)
	assert.NotEmpty(t, firstPage.GetNextCursor())
	require.NoError(t, secondErr)
	if assert.Len(t, secondPage.GetLogs(), 1) {
		assert.NotEqual(t, firstPage.GetLogs()[0].GetId(), secondPage.GetLogs()[0].GetId(), "o cursor anda para a próxima linha")
	}
	require.NoError(t, createdErr)
	if assert.Len(t, created.GetLogs(), 1) {
		assert.Equal(t, maria.id.String(), created.GetLogs()[0].GetActorId())
		assert.Equal(t, "allowed", created.GetLogs()[0].GetOutcome())
	}
	require.NoError(t, ownErr)
	for _, log := range own.GetLogs() {
		assert.Equal(t, dev.id.String(), log.GetActorId(), "o developer só vê a própria trilha")
	}
	assert.Equal(t, codes.NotFound, status.Code(outsider))
}

func whenGRPCSDKTokenReachesPersonServices(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	member := s.memberID(&maria, app)
	id := s.createSecret(&maria, app, member, "API_KEY", "api_key", "sk-123")
	token := s.newToken(&maria, app, "me", "ci")
	conn := s.grpcConn()
	sdk, anonymous := bearer(token.token), context.Background()

	for name, ctx := range map[string]context.Context{"token do SDK": sdk, "anônimo": anonymous} {
		_, members := vaultv1.NewMemberServiceClient(conn).ListMembers(ctx, &vaultv1.ListMembersRequest{ApplicationId: app})
		_, invitations := vaultv1.NewInvitationServiceClient(conn).CreateInvitation(ctx, &vaultv1.CreateInvitationRequest{ApplicationId: app, Email: "x@example.com", Role: "admin"})
		_, tokens := vaultv1.NewApiKeyServiceClient(conn).CreateApiKey(ctx, &vaultv1.CreateApiKeyRequest{ApplicationId: app, MemberId: member, Name: "outro", Type: "secret"})
		revealed, secrets := vaultv1.NewSecretServiceClient(conn).RevealSecret(ctx, &vaultv1.RevealSecretRequest{ApplicationId: app, MemberId: member, SecretId: id})
		_, audit := vaultv1.NewAuditServiceClient(conn).ListAuditLogs(ctx, &vaultv1.ListAuditLogsRequest{ApplicationId: app})

		for _, err := range []error{members, invitations, tokens, secrets, audit} {
			assert.Equal(t, codes.Unauthenticated, status.Code(err), name)
		}
		assert.Nil(t, revealed, name)
	}
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM api_keys`), "o token do SDK não cria outro token")
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM invitations`))
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'secret.revealed'`), "nenhum valor foi revelado")
}

// Helpers

// stack é o Vault inteiro, como roda em produção (autenticação, guard,
// transações, auditoria e SQLite reais), com só o Auth falso.
type stack struct {
	t       *testing.T
	db      *sql.DB
	box     *secretbox.Box
	dbPath  string
	auth    *authntest.Auth
	handler http.Handler
}

func newStack(t *testing.T) *stack {
	t.Helper()

	dbPath := t.TempDir() + "/vault.db"
	db, err := sqlite.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, migration.New(db, migrations_embed.FS).Run(context.Background(), "sqlite"))

	mux := http.NewServeMux()
	key := make([]byte, secretbox.KeySize)
	_, err = rand.Read(key)
	require.NoError(t, err)
	box, err := secretbox.New(key)
	require.NoError(t, err)
	tokens, err := restful.Init(db, box, "https://vault.example.com", mux)
	require.NoError(t, err)

	auth := authntest.New(t)
	return &stack{t: t, db: db, box: box, dbPath: dbPath, auth: auth, handler: authn.Authenticate(auth.Verifier(), tokens, mux)}
}

type person struct {
	id    uuid.UUID
	token string
}

func (s *stack) person(roles ...string) person {
	id := uuid.NewV7()
	return person{id: id, token: s.auth.Token(s.t, id, roles, time.Minute)}
}

func (s *stack) do(who *person, method, path string, body any) (int, map[string]any) {
	s.t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(s.t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if who != nil {
		req.Header.Set("Authorization", "Bearer "+who.token)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)

	assert.Equal(s.t, "no-store", rec.Header().Get("Cache-Control"), "toda resposta da API é no-store")

	var parsed map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	return rec.Code, parsed
}

func (s *stack) createApplication(who *person, name string) string {
	s.t.Helper()
	status, body := s.do(who, http.MethodPost, "/api/v1/applications", map[string]any{"name": name})
	require.Equal(s.t, http.StatusCreated, status, "%v", body)
	return body["data"].(map[string]any)["id"].(string)
}

func toJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func (s *stack) count(query string, args ...any) int {
	s.t.Helper()
	var n int
	require.NoError(s.t, s.db.QueryRow(query, args...).Scan(&n))
	return n
}

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

const plain = "postgres://app:s3cr3t-P4ss@db.internal:5432/app"

func secretsPath(app, member string) string {
	return "/api/v1/applications/" + app + "/members/" + member + "/secrets"
}

func (s *stack) memberID(who *person, app string) string {
	s.t.Helper()
	status, body := s.do(who, http.MethodGet, "/api/v1/applications/"+app+"/members/me", nil)
	require.Equal(s.t, http.StatusOK, status)
	return body["data"].(map[string]any)["id"].(string)
}

func (s *stack) createSecret(who *person, app, member, name, kind, value string) string {
	s.t.Helper()
	status, body := s.do(who, http.MethodPost, secretsPath(app, member), map[string]any{"name": name, "type": kind, "value": value})
	require.Equal(s.t, http.StatusCreated, status, "%v", body)
	return body["data"].(map[string]any)["id"].(string)
}

func (s *stack) reveal(who *person, app, member, id string) (int, map[string]any) {
	return s.do(who, http.MethodPost, secretsPath(app, member)+"/"+id+"/reveal", nil)
}

// raw faz a chamada com um Accept e devolve o corpo cru.
func (s *stack) raw(who *person, method, path, accept, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+who.token)
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func columnsOf(table string) string {
	switch table {
	case "audit_logs":
		return "metadata || ifnull(target_id,'') || action"
	case "secrets":
		return "name || type || source"
	case "secret_versions":
		return "ciphertext"
	}
	return "payload"
}

// tokensPath é a rota das chaves com o dono na query; keyPath acrescenta o
// trecho por id (ex.: "/<id>/rotate") antes da query.
func tokensPath(app, member string) string {
	return keyPath(app, member, "")
}

func keyPath(app, member, sub string) string {
	return "/api/v1/api-keys" + sub + "?application_id=" + app + "&member_id=" + member
}

// sdk guarda um token do SDK e o uri_id do membro dono.
type sdk struct {
	id    string
	token string
	uri   string
}

func (s *stack) newToken(who *person, app, member, name string) sdk {
	s.t.Helper()
	return s.newKey(who, app, member, name, "secret")
}

func (s *stack) newKey(who *person, app, member, name, keyType string) sdk {
	s.t.Helper()
	status, body := s.do(who, http.MethodPost, tokensPath(app, member), map[string]any{"name": name, "type": keyType})
	require.Equal(s.t, http.StatusCreated, status, "%v", body)
	data := body["data"].(map[string]any)
	return sdk{id: data["id"].(string), token: data["key"].(string), uri: data["uri_id"].(string)}
}

func (s *stack) env(token, uri, query, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/"+uri+"/env"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func auditPath(app, query string) string {
	return "/api/v1/applications/" + app + "/audit-logs" + query
}

// stripInstance remove o campo instance (o caminho, que difere) para comparar o resto do erro.
func stripInstance(raw []byte) string {
	var parsed map[string]map[string]any
	_ = json.Unmarshal(raw, &parsed)
	delete(parsed["error"], "instance")
	out, _ := json.Marshal(parsed)
	return string(out)
}

func (s *stack) grpcConn() *grpc.ClientConn {
	s.t.Helper()

	deps, tokens := dependencies.New(s.db, s.box)
	server, err := restful.InitGRPC(s.db, deps, s.auth.Verifier(), tokens, "https://vault.example.com", "0")
	require.NoError(s.t, err)
	go func() { _ = server.Serve() }()
	s.t.Cleanup(server.GracefulStop)

	conn, err := grpc.NewClient(server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.t, err)
	s.t.Cleanup(func() { _ = conn.Close() })

	return conn
}

func (s *stack) grpcClient() (vaultv1.VaultServiceClient, string) {
	conn := s.grpcConn()
	return vaultv1.NewVaultServiceClient(conn), conn.Target()
}

func (s *stack) grpcApplications() vaultv1.ApplicationServiceClient {
	return vaultv1.NewApplicationServiceClient(s.grpcConn())
}

func bearer(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func whenInitHasNoMux(t *testing.T) {
	db := sqlitetest.New(t)
	box, err := secretbox.New(make([]byte, secretbox.KeySize))
	require.NoError(t, err)

	tokens, err := restful.Init(db, box, "https://vault.example.com", nil)

	assert.Error(t, err, "o primeiro service que falha derruba o Init")
	assert.Nil(t, tokens)
}

func whenInitGRPCRegistersEveryService(t *testing.T) {
	db := sqlitetest.New(t)
	deps := pkgmocks.NewDependencies(t, db)

	server, err := restful.InitGRPC(db, deps, authntest.New(t).Verifier(), nil, "https://vault.example.com", "0")
	require.NoError(t, err)
	defer server.Close()

	info := server.Registrar().(*grpc.Server).GetServiceInfo()
	for _, name := range []string{"VaultService", "ApplicationService", "MemberService", "InvitationService", "SecretService", "ApiKeyService", "AuditService"} {
		assert.Contains(t, info, "murkio.vault.v1."+name)
	}
}

func whenInitGRPCPortIsInUse(t *testing.T) {
	db := sqlitetest.New(t)
	deps := pkgmocks.NewDependencies(t, db)
	first, err := restful.InitGRPC(db, deps, nil, nil, "https://vault.example.com", "0")
	require.NoError(t, err)
	defer first.Close()
	_, port, err := net.SplitHostPort(first.Addr())
	require.NoError(t, err)

	second, err := restful.InitGRPC(db, deps, nil, nil, "https://vault.example.com", port)

	assert.Error(t, err)
	assert.Nil(t, second)
}
