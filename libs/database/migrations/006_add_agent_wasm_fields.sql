-- Migration: 006_add_agent_wasm_fields
-- Created: 2025-11-12
-- Description: Add wasm_hash and s3_key fields to agents table for WASM binary storage

-- Add wasm_hash column for binary integrity verification
ALTER TABLE agents ADD COLUMN IF NOT EXISTS wasm_hash VARCHAR(64);

-- Add s3_key column for R2/S3 storage reference
ALTER TABLE agents ADD COLUMN IF NOT EXISTS s3_key TEXT;

-- Create index on wasm_hash for fast lookups
CREATE INDEX IF NOT EXISTS idx_agents_wasm_hash ON agents(wasm_hash);

-- Create index on s3_key for storage queries
CREATE INDEX IF NOT EXISTS idx_agents_s3_key ON agents(s3_key);
