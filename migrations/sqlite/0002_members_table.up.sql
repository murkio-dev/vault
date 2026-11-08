-- Membros: quem pertence a uma aplicação. user_id é o sub da conta no Auth.
-- uri_id é o identificador público da tabela de segredos do membro (a URI do
-- SDK); ele sozinho não dá acesso, a credencial é o token (access_tokens).
-- O papel fica aqui, não no Auth.
CREATE TABLE IF NOT EXISTS members (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id),
    user_id TEXT NOT NULL,
    uri_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'developer')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    invited_by TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME,
    deleted_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_members_application_user ON members(application_id, user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_members_uri ON members(uri_id);
CREATE INDEX IF NOT EXISTS idx_members_user ON members(user_id) WHERE deleted_at IS NULL;
