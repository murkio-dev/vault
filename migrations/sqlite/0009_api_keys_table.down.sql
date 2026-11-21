-- Volta api_keys para access_tokens. As chaves publishable somem: o formato
-- antigo só conhece o token secreto.
DELETE FROM api_keys WHERE type = 'publishable';
DROP INDEX IF EXISTS idx_api_keys_member;
ALTER TABLE api_keys DROP COLUMN updated_at;
ALTER TABLE api_keys DROP COLUMN type;
ALTER TABLE api_keys RENAME COLUMN deleted_by TO revoked_by;
ALTER TABLE api_keys RENAME COLUMN deleted_at TO revoked_at;
ALTER TABLE api_keys RENAME COLUMN hash TO token_hash;
ALTER TABLE api_keys RENAME TO access_tokens;
CREATE INDEX IF NOT EXISTS idx_access_tokens_member ON access_tokens(member_id, created_at DESC);
