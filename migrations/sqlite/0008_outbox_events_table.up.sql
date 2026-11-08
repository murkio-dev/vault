-- Eventos para outros serviços (ex.: o convite que o messaging envia por
-- e-mail), gravados na mesma transação da mudança. Quem os publica é um relay
-- externo, como no auth. O payload de um comando pode ter segredo (o token do
-- convite): depois de publicado, é trocado por {}.
CREATE TABLE IF NOT EXISTS outbox_events (
    id TEXT PRIMARY KEY,
    application_id TEXT REFERENCES applications(id),
    type TEXT NOT NULL,
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    exchange TEXT NOT NULL,
    envelope BOOLEAN NOT NULL DEFAULT TRUE,
    published_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending ON outbox_events(created_at) WHERE published_at IS NULL;
