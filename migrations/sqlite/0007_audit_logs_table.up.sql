-- Auditoria. Só aceita inserção: o próprio banco recusa UPDATE e DELETE, então
-- nem um bug no serviço adultera a trilha. metadata guarda nomes, versões e
-- ids, nunca valores de segredo. application_id é nulo só em eventos fora de
-- uma aplicação (ex.: acesso negado antes de saber qual).
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    application_id TEXT,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('user', 'token', 'system')),
    actor_id TEXT,
    token_id TEXT,
    action TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('allowed', 'denied')),
    target_type TEXT,
    target_id TEXT,
    ip TEXT,
    user_agent TEXT,
    request_id TEXT,
    metadata TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata)),
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_application ON audit_logs(application_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(application_id, actor_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(application_id, action, id DESC);

CREATE TRIGGER IF NOT EXISTS audit_logs_no_update
BEFORE UPDATE ON audit_logs
BEGIN
    SELECT RAISE(ABORT, 'audit_logs is append-only');
END;

CREATE TRIGGER IF NOT EXISTS audit_logs_no_delete
BEFORE DELETE ON audit_logs
BEGIN
    SELECT RAISE(ABORT, 'audit_logs is append-only');
END;
