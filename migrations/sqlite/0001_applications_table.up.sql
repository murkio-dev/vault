-- Aplicações do Vault: o espaço de um time. Quem cria vira admin (members).
-- O nome é único só entre as ativas, para uma aplicação apagada liberar o nome.
CREATE TABLE IF NOT EXISTS applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 3 AND 50),
    created_by TEXT NOT NULL,
    updated_at DATETIME,
    created_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_applications_name ON applications(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_applications_created_by ON applications(created_by);
