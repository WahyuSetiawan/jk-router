-- Add expires_at for OAuth token tracking.
ALTER TABLE accounts ADD COLUMN expires_at INTEGER;
