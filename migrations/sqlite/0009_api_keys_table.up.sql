-- access_tokens vira api_keys, no mesmo formato das chaves do Identity e do
-- Messaging: tipo publishable (pk_vt_) ou secret (sk_vt_), hash, deleted_at
-- no lugar de revoked_at. A chave continua presa a um membro (member_id), que
-- é de onde vem a permissão. Os tokens sk_vt_ existentes viram chaves secret e
-- continuam valendo, porque o hash não muda.
ALTER TABLE access_tokens RENAME TO api_keys;
ALTER TABLE api_keys RENAME COLUMN token_hash TO hash;
ALTER TABLE api_keys RENAME COLUMN revoked_at TO deleted_at;
ALTER TABLE api_keys RENAME COLUMN revoked_by TO deleted_by;
ALTER TABLE api_keys ADD COLUMN type TEXT NOT NULL DEFAULT 'secret' CHECK (type IN ('publishable', 'secret'));
ALTER TABLE api_keys ADD COLUMN updated_at DATETIME;

DROP INDEX IF EXISTS idx_access_tokens_member;
CREATE INDEX IF NOT EXISTS idx_api_keys_member ON api_keys(member_id, created_at DESC);
