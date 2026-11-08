-- Convites por e-mail. O token (inv_...) só existe em claro na resposta da
-- criação; aqui fica o hash. Uso único: accepted_at fecha o convite. O e-mail
-- serve para o envio e a conferência futura com a conta (o JWT do Auth não o
-- carrega hoje).
CREATE TABLE IF NOT EXISTS invitations (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id),
    email TEXT NOT NULL CHECK (length(email) <= 254),
    role TEXT NOT NULL CHECK (role IN ('admin', 'developer')),
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_by TEXT NOT NULL,
    accepted_by TEXT,
    accepted_at DATETIME,
    revoked_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_invitations_application ON invitations(application_id, created_at DESC);
