-- Sprint 5 P2: Quota tracking per account
ALTER TABLE accounts ADD COLUMN quota_limit INTEGER DEFAULT 0;
ALTER TABLE accounts ADD COLUMN quota_window_seconds INTEGER DEFAULT 86400;
ALTER TABLE accounts ADD COLUMN quota_reset_at INTEGER DEFAULT 0;
