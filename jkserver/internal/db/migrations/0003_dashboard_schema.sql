-- 0003_dashboard_schema.sql
-- Fix schema mismatch: V2 CREATE TABLE IF NOT EXISTS silently skipped
-- because V1 already created these tables. Use ALTER TABLE to add missing
-- columns, and drop obsolete ones.

-- combos: V1 has (id, name, model_list TEXT NOT NULL, created_at TEXT).
-- Dashboard needs: description, model_ids, strategy, updated_at.
-- SQLite doesn't support DROP COLUMN before 3.35, so we add what's needed
-- and update existing rows to copy model_list -> model_ids.
ALTER TABLE combos ADD COLUMN description TEXT DEFAULT '';
ALTER TABLE combos ADD COLUMN model_ids TEXT;
ALTER TABLE combos ADD COLUMN strategy TEXT DEFAULT 'fallback';
ALTER TABLE combos ADD COLUMN updated_at INTEGER DEFAULT (strftime('%s','now'));
-- Migrate: copy model_list into model_ids for existing rows
UPDATE combos SET model_ids = model_list WHERE model_ids IS NULL;

-- proxy_pools: V1 has no created_at. Add it.
ALTER TABLE proxy_pools ADD COLUMN created_at INTEGER DEFAULT (strftime('%s','now'));

-- usage_log: V1 has adapter_used INTEGER. Keep as-is (matches engine code).

-- accounts: V1 doesn't have this table. Safe to CREATE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS accounts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id   TEXT NOT NULL,
    label         TEXT NOT NULL DEFAULT '',
    auth_type     TEXT NOT NULL DEFAULT 'api_key',
    encrypted_key BLOB,
    proxy_pool_id INTEGER,
    priority      INTEGER NOT NULL DEFAULT 0,
    state         TEXT NOT NULL DEFAULT 'active',
    strike_count  INTEGER NOT NULL DEFAULT 0,
    cooled_until  INTEGER,
    created_at    INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at    INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

-- combo_accounts, pricing — safe to CREATE IF NOT EXISTS
CREATE TABLE IF NOT EXISTS combo_accounts (
    combo_id   INTEGER NOT NULL,
    account_id INTEGER NOT NULL,
    priority   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (combo_id, account_id),
    FOREIGN KEY (combo_id) REFERENCES combos(id) ON DELETE CASCADE,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS pricing (
    model              TEXT PRIMARY KEY,
    price_in_per_1k    REAL NOT NULL DEFAULT 0,
    price_out_per_1k   REAL NOT NULL DEFAULT 0,
    source             TEXT NOT NULL DEFAULT 'seed'
);

-- indexes
CREATE INDEX IF NOT EXISTS idx_usage_log_request ON usage_log(request_id);
CREATE INDEX IF NOT EXISTS idx_usage_log_combo   ON usage_log(combo);
CREATE INDEX IF NOT EXISTS idx_usage_log_ts      ON usage_log(ts);
