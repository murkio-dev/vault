package restful_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
