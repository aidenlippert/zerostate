-- Migration: Fix VARCHAR constraints in agents table
-- Issue: Agent registration fails due to VARCHAR length limits
-- Solution: Change VARCHAR to TEXT for flexible-length fields

-- Fix pricing_model (was VARCHAR(50))
ALTER TABLE agents ALTER COLUMN pricing_model TYPE TEXT;

-- Fix did (was VARCHAR(255) - can be longer for some DID methods)
ALTER TABLE agents ALTER COLUMN did TYPE TEXT;

-- Fix status (was VARCHAR(50))
ALTER TABLE agents ALTER COLUMN status TYPE TEXT;

-- Fix region (was VARCHAR(100))
ALTER TABLE agents ALTER COLUMN region TYPE TEXT;

-- Verify changes
SELECT column_name, data_type, character_maximum_length
FROM information_schema.columns
WHERE table_name = 'agents'
AND column_name IN ('pricing_model', 'did', 'status', 'region')
ORDER BY column_name;
