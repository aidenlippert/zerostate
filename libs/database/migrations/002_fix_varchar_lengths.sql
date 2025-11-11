-- Migration 002: Fix VARCHAR length constraints
-- Version: 002
-- Description: Change VARCHAR constraints to TEXT for flexible fields

-- Fix agents table VARCHAR constraints
ALTER TABLE agents ALTER COLUMN pricing_model TYPE TEXT;
ALTER TABLE agents ALTER COLUMN did TYPE TEXT;
ALTER TABLE agents ALTER COLUMN status TYPE TEXT;
ALTER TABLE agents ALTER COLUMN region TYPE TEXT;

-- Record migration
INSERT INTO schema_migrations (version, description)
VALUES (2, 'Fix VARCHAR length constraints in agents table')
ON CONFLICT (version) DO NOTHING;
