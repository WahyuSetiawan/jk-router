-- 0009_provider_base_url.sql
-- Allow dashboard to override base_url per provider (e.g. point OpenAI at a proxy).
ALTER TABLE providers ADD COLUMN base_url TEXT DEFAULT '';
