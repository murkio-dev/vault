package restful_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/restful"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/authn/authntest"
	"github.com/murkio-dev/vault/pkg/migration"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stack é o Vault inteiro, como roda em produção (autenticação, guard,
// transações, auditoria e SQLite reais), com só o Auth falso.
type stack struct {
	t       *testing.T
	db      *sql.DB
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
	return &stack{t: t, db: db, dbPath: dbPath, auth: auth, handler: authn.Authenticate(auth.Verifier(), tokens, mux)}
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
	_, err := s.db.Exec(`INSERT INTO access_tokens (id, application_id, member_id, name, prefix, token_hash, created_by, created_at)
		SELECT 'tok', application_id, id, 'n', 'vt_x', 'hash', user_id, ? FROM members WHERE application_id = ?`, time.Now().UTC(), id)
	require.NoError(t, err)

	status, _ := s.do(&maria, http.MethodDelete, "/api/v1/applications/"+id, nil)
	read, _ := s.do(&maria, http.MethodGet, "/api/v1/applications/"+id, nil)

	assert.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, http.StatusNotFound, read)
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM access_tokens WHERE revoked_at IS NOT NULL`), "os tokens são revogados na cascata")
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
