-- Add cost_usd column to usage_log for pricing tracking (P2)
ALTER TABLE usage_log ADD COLUMN cost_usd REAL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_usage_log_cost ON usage_log(cost_usd);
