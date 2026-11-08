-- Segredos: só os metadados. O valor nunca fica aqui, fica cifrado em
-- secret_versions. type e source são validados no Go (e não por CHECK) para
-- novos tipos e origens não exigirem recriar a tabela. Uma reference aponta
-- para outra secret da mesma aplicação (EnvironmentRef).
CREATE TABLE IF NOT EXISTS secrets (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id),
    member_id TEXT NOT NULL REFERENCES members(id),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    type TEXT NOT NULL,
    source TEXT NOT NULL,
    reference_secret_id TEXT REFERENCES secrets(id),
    current_version INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME,
    deleted_at DATETIME,
    CHECK ((source = 'reference') = (reference_secret_id IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_secrets_member_name ON secrets(member_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_secrets_application ON secrets(application_id);
CREATE INDEX IF NOT EXISTS idx_secrets_reference ON secrets(reference_secret_id) WHERE reference_secret_id IS NOT NULL;
