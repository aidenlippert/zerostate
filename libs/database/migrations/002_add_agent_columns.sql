-- Add missing columns to agents table
-- Version: 002
-- Description: Add DID, description, status, and other missing columns to agents table

-- Add DID column (required for agent identification)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS did TEXT NOT NULL DEFAULT '';

-- Add description column
ALTER TABLE agents ADD COLUMN IF NOT EXISTS description TEXT;

-- Add pricing_model column (replaces simple price column)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS pricing_model TEXT;

-- Add status column with enum constraint
ALTER TABLE agents ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'online';
ALTER TABLE agents ADD CONSTRAINT agents_status_check CHECK (status IN ('online', 'busy', 'offline', 'maintenance'));

-- Add capacity columns
ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_capacity INTEGER NOT NULL DEFAULT 10;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS current_load INTEGER NOT NULL DEFAULT 0;

-- Add region column
ALTER TABLE agents ADD COLUMN IF NOT EXISTS region TEXT;

-- Add updated_at and last_seen_at timestamps
ALTER TABLE agents ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP;

-- Add metadata column for JSONB data
ALTER TABLE agents ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Drop version column if it exists (not used in current model)
ALTER TABLE agents DROP COLUMN IF EXISTS version;

-- Update capabilities column to JSONB if it's TEXT
ALTER TABLE agents ALTER COLUMN capabilities TYPE JSONB USING capabilities::jsonb;

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_agents_did ON agents(did);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_created_at ON agents(created_at);
