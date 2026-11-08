-- Versões do valor, cifradas (AES-256-GCM, AAD "secret_version:<id>"). Só
-- cresce: uma versão nunca é apagada nem reescrita. A única alteração aceita é
-- a do ciphertext, para recifrar com a chave nova quando a ENCRYPTION_KEY gira.
CREATE TABLE IF NOT EXISTS secret_versions (
    id TEXT PRIMARY KEY,
    secret_id TEXT NOT NULL REFERENCES secrets(id),
    version INTEGER NOT NULL CHECK (version >= 1),
    ciphertext BLOB NOT NULL,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    UNIQUE (secret_id, version)
);

CREATE TRIGGER IF NOT EXISTS secret_versions_no_delete
BEFORE DELETE ON secret_versions
BEGIN
    SELECT RAISE(ABORT, 'secret_versions is append-only');
END;

CREATE TRIGGER IF NOT EXISTS secret_versions_only_reseal
BEFORE UPDATE ON secret_versions
WHEN OLD.id <> NEW.id OR OLD.secret_id <> NEW.secret_id OR OLD.version <> NEW.version
    OR OLD.created_by <> NEW.created_by OR OLD.created_at <> NEW.created_at
BEGIN
    SELECT RAISE(ABORT, 'secret_versions only accepts a new ciphertext');
END;
