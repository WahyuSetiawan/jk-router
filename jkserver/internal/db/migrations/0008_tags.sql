-- 0008_tags.sql
-- Tags support for accounts (connections)
ALTER TABLE accounts ADD COLUMN tags TEXT NOT NULL DEFAULT '';
