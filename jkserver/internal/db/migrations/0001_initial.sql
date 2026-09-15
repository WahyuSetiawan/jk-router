-- 0001_initial.sql
CREATE TABLE IF NOT EXISTS providers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS connections (
  id INTEGER PRIMARY KEY,
  provider_id TEXT NOT NULL,
  name TEXT,
  auth_type TEXT NOT NULL DEFAULT 'apikey',
  secret_enc BLOB,
  base_url TEXT,
  proxy_pool_id INTEGER,
  priority INTEGER NOT NULL DEFAULT 0,
  disabled INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS model_catalog (
  provider_id TEXT NOT NULL,
  model TEXT NOT NULL,
  capability TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT 'manual',
  PRIMARY KEY (provider_id, model)
);

CREATE TABLE IF NOT EXISTS combos (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  model_list TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS api_keys (
  id INTEGER PRIMARY KEY,
  key_hash TEXT NOT NULL,
  label TEXT,
  revoked INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proxy_pools (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  ptype TEXT NOT NULL DEFAULT 'http',
  proxy_url TEXT,
  no_proxy TEXT,
  is_active INTEGER NOT NULL DEFAULT 1,
  strict_proxy INTEGER NOT NULL DEFAULT 0,
  test_status TEXT NOT NULL DEFAULT 'untested',
  last_tested_at TEXT
);

CREATE TABLE IF NOT EXISTS settings_kv (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS usage_log (
  id INTEGER PRIMARY KEY,
  request_id TEXT,
  combo TEXT,
  model TEXT,
  provider TEXT,
  account_id INTEGER,
  fallback_from TEXT,
  state_at_start TEXT,
  tok_in INTEGER,
  tok_out INTEGER,
  latency_ms INTEGER,
  status TEXT,
  adapter_used INTEGER NOT NULL DEFAULT 0,
  ts TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY
);
