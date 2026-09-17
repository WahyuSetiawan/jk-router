-- 0007_media.sql
-- Media providers (TTS, STT, Image). Separate from chat accounts — different
-- API contracts, no combo/failover support needed yet.

CREATE TABLE IF NOT EXISTS media_accounts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id   TEXT NOT NULL,
    label         TEXT NOT NULL DEFAULT '',
    auth_type     TEXT NOT NULL DEFAULT 'api_key',
    encrypted_key BLOB,
    priority      INTEGER NOT NULL DEFAULT 0,
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at    INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);
