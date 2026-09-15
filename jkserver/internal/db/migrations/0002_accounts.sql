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

CREATE TABLE IF NOT EXISTS combos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    model_ids   TEXT NOT NULL,
    strategy    TEXT NOT NULL DEFAULT 'fallback',
    created_at  INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at  INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

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

CREATE TABLE IF NOT EXISTS usage_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id     TEXT NOT NULL,
    combo          TEXT,
    model          TEXT,
    provider       TEXT,
    account_id     INTEGER,
    fallback_from  TEXT,
    state_at_start TEXT DEFAULT 'active',
    tok_in         INTEGER DEFAULT 0,
    tok_out        INTEGER DEFAULT 0,
    latency_ms     INTEGER,
    status         TEXT NOT NULL,
    adapter_used   TEXT DEFAULT '',
    ts             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S','now'))
);

CREATE INDEX IF NOT EXISTS idx_usage_log_request ON usage_log(request_id);
CREATE INDEX IF NOT EXISTS idx_usage_log_combo   ON usage_log(combo);
CREATE INDEX IF NOT EXISTS idx_usage_log_ts      ON usage_log(ts);
