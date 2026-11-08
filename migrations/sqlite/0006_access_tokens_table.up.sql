-- Tokens do SDK (vt_...). Só o hash SHA-256 fica aqui; prefix é um trecho
-- curto para a pessoa reconhecer o token na lista. Revogado ou expirado deixa
-- de valer na hora (a busca é por hash a cada requisição).
CREATE TABLE IF NOT EXISTS access_tokens (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id),
    member_id TEXT NOT NULL REFERENCES members(id),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    prefix TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME,
    last_used_at DATETIME,
    revoked_at DATETIME,
    revoked_by TEXT,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_access_tokens_member ON access_tokens(member_id, created_at DESC);
