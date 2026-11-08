package restful_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tokensPath(app, member string) string {
	return "/api/v1/applications/" + app + "/members/" + member + "/tokens"
}

// sdk guarda um token do SDK e o uri_id do membro dono.
type sdk struct {
	id    string
	token string
	uri   string
}

func (s *stack) newToken(who *person, app, member, name string) sdk {
	s.t.Helper()
	status, body := s.do(who, http.MethodPost, tokensPath(app, member), map[string]any{"name": name})
	require.Equal(s.t, http.StatusCreated, status, "%v", body)
	data := body["data"].(map[string]any)
	return sdk{id: data["id"].(string), token: data["token"].(string), uri: data["uri_id"].(string)}
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
}

func whenTokenIsCreated(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")

	req := httptest.NewRequest(http.MethodPost, tokensPath(app, "me"), strings.NewReader(`{"name":"notebook"}`))
	req.Header.Set("Authorization", "Bearer "+maria.token)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", "a resposta com o token é sempre JSON")
	body := rec.Body.String()
	raw := strings.Split(strings.Split(body, `"token":"`)[1], `"`)[0]
	assert.True(t, strings.HasPrefix(raw, "vt_"))
	assert.Contains(t, body, `"uri":"https://vault.example.com/`)
	assert.Contains(t, body, `"prefix":"`+raw[:9]+`"`)

	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM access_tokens WHERE token_hash = ?`, raw), "o banco guarda o hash, não o token")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM access_tokens WHERE prefix = ?`, raw[:9]))
	assert.Equal(t, 0, s.count(`SELECT COUNT(*) FROM audit_logs WHERE metadata LIKE ?`, "%"+raw+"%"), "a trilha nunca leva o token")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'token.created'`))

	status, list := s.do(&maria, http.MethodGet, tokensPath(app, "me"), nil)
	require.Equal(t, http.StatusOK, status)
	item := list["data"].([]any)[0].(map[string]any)
	assert.NotContains(t, item, "token", "a lista só mostra o prefixo")
	assert.Equal(t, "notebook", item["name"])
}

func whenRolesCreateTokens(t *testing.T) {
	s := newStack(t)
	maria, outsider := s.person(), s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, devMember := s.join(&maria, app, "developer")
	other, otherMember := s.join(&maria, app, "developer")

	own, _ := s.do(&dev, http.MethodPost, tokensPath(app, "me"), map[string]any{"name": "meu"})
	forOther, _ := s.do(&dev, http.MethodPost, tokensPath(app, otherMember), map[string]any{"name": "roubado"})
	byAdmin, _ := s.do(&maria, http.MethodPost, tokensPath(app, devMember), map[string]any{"name": "do-admin"})
	hidden, _ := s.do(&outsider, http.MethodPost, tokensPath(app, devMember), map[string]any{"name": "x"})
	listOther, _ := s.do(&other, http.MethodGet, tokensPath(app, devMember), nil)
	missing, _ := s.do(&maria, http.MethodPost, tokensPath(app, uuid.NewV7().String()), map[string]any{"name": "x"})

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
		"sem nome":              {"name": ""},
		"nome com controle":     {"name": "a\nb"},
		"nome longo":            {"name": strings.Repeat("a", 101)},
		"vencimento no passado": {"name": "x", "expires_at": past},
		"vencimento distante":   {"name": "x", "expires_at": far},
		"campo desconhecido":    {"name": "x", "member_id": member},
	}
	for name, body := range cases {
		status, _ := s.do(&maria, http.MethodPost, tokensPath(app, "me"), body)
		assert.Equal(t, http.StatusBadRequest, status, name)
	}
	ok, _ := s.do(&maria, http.MethodPost, tokensPath(app, "me"), map[string]any{"name": "x", "expires_at": good})
	assert.Equal(t, http.StatusCreated, ok)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM access_tokens`))

	_, err = s.db.Exec(`UPDATE members SET status = 'suspended' WHERE id = ?`, member)
	require.NoError(t, err)
	root := s.person("admin")
	suspended, _ := s.do(&root, http.MethodPost, tokensPath(app, member), map[string]any{"name": "x"})
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
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM access_tokens WHERE id = ? AND last_used_at IS NOT NULL`, token.id), "o último uso é registrado")

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

func whenTokenIsUsedWhereItShouldNot(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	token := s.newToken(&maria, app, "me", "a")

	asManager := s.raw(&person{token: token.token}, http.MethodGet, "/api/v1/applications", "", "")
	asManagerWrite := s.raw(&person{token: token.token}, http.MethodPost, secretsPath(app, "me"), "", `{"name":"A","type":"string","value":"v"}`)
	jwtOnRuntime := s.raw(&maria, http.MethodGet, "/api/v1/runtime/"+token.uri+"/env", "", "")
	random := s.env("vt_"+strings.Repeat("A", 43), token.uri, "", "")
	short := s.env("vt_", token.uri, "", "")
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
	path := tokensPath(app, devMember) + "/" + token.id + "/revoke"

	before := s.env(token.token, token.uri, "", "")
	byOther, _ := s.do(&other, http.MethodPost, path, nil)
	byAdmin, _ := s.do(&maria, http.MethodPost, path, nil)
	after := s.env(token.token, token.uri, "", "")
	repeat, _ := s.do(&maria, http.MethodPost, path, nil)
	wrongMember, _ := s.do(&maria, http.MethodPost, tokensPath(app, "me")+"/"+again.id+"/revoke", nil)
	unknown, _ := s.do(&maria, http.MethodPost, tokensPath(app, devMember)+"/"+uuid.NewV7().String()+"/revoke", nil)
	untouched := s.env(again.token, again.uri, "", "")

	assert.Equal(t, http.StatusOK, before.Code)
	assert.Equal(t, http.StatusForbidden, byOther, "um developer não revoga o token de outro")
	assert.Equal(t, http.StatusNoContent, byAdmin, "o admin revoga qualquer um")
	assert.Equal(t, http.StatusUnauthorized, after.Code, "revogado vale na hora")
	assert.Equal(t, http.StatusNoContent, repeat, "revogar de novo é idempotente")
	assert.Equal(t, http.StatusNotFound, wrongMember, "o token tem de ser do membro do caminho")
	assert.Equal(t, http.StatusNotFound, unknown)
	assert.Equal(t, http.StatusOK, untouched.Code, "os outros tokens do dev seguem valendo")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'token.revoked' AND target_id = ?`, token.id), "uma linha só, mesmo revogando duas vezes")

	_, list := s.do(&dev, http.MethodGet, tokensPath(app, "me"), nil)
	revoked := 0
	for _, item := range list["data"].([]any) {
		if item.(map[string]any)["revoked_at"] != nil {
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

	_, err := s.db.Exec(`UPDATE access_tokens SET expires_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Second), expired.id)
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

func TestAuditLogs(t *testing.T) {
	t.Run("Quando o admin lê a trilha com cursor.", func(t *testing.T) { whenAdminReadsTrailWithCursor(t) })
	t.Run("Quando o developer lê a trilha.", func(t *testing.T) { whenDeveloperReadsTrail(t) })
	t.Run("Quando os filtros são inválidos.", func(t *testing.T) { whenAuditFiltersAreInvalid(t) })
	t.Run("Quando alguém tenta mexer na trilha.", func(t *testing.T) { whenSomeoneTriesToTamperTheTrail(t) })
}

func auditPath(app, query string) string {
	return "/api/v1/applications/" + app + "/audit-logs" + query
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

// stripInstance remove o campo instance (o caminho, que difere) para comparar o resto do erro.
func stripInstance(raw []byte) string {
	var parsed map[string]map[string]any
	_ = json.Unmarshal(raw, &parsed)
	delete(parsed["error"], "instance")
	out, _ := json.Marshal(parsed)
	return string(out)
}
