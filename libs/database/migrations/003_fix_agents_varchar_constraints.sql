-- Migration: Fix VARCHAR length constraints in agents table
-- Issue: pricing_model VARCHAR(50) is too small for JSON pricing data
-- Date: 2025-11-10

-- Change pricing_model from VARCHAR(50) to TEXT to accommodate full JSON pricing objects
ALTER TABLE agents ALTER COLUMN pricing_model TYPE TEXT;

-- Also change did, status, and region to TEXT for future flexibility
ALTER TABLE agents ALTER COLUMN did TYPE TEXT;
ALTER TABLE agents ALTER COLUMN status TYPE TEXT;
ALTER TABLE agents ALTER COLUMN region TYPE TEXT;
