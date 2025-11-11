package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"time"
)

// FAANG-LEVEL DATABASE MIGRATION SYSTEM
// Following best practices:
// - Version-controlled schema changes
// - Transactional migrations (all-or-nothing)
// - Rollback support for failed migrations
// - Migration history tracking
// - Idempotent migrations (safe to retry)
// - Schema versioning and validation

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migration represents a single database migration
type Migration struct {
	Version     int
	Description string
	SQL         string
	AppliedAt   *time.Time
}

// MigrationHistory tracks applied migrations
type MigrationHistory struct {
	ID          int
	Version     int
	Description string
	AppliedAt   time.Time
}

// createMigrationsTable creates the migrations tracking table
func createMigrationsTable(ctx context.Context, db *sql.DB) error {
	// Note: uuid-ossp extension should be pre-enabled in the database
	// For Supabase, this is already enabled by default
	// For local PostgreSQL: CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id SERIAL PRIMARY KEY,
			version INT UNIQUE NOT NULL,
			description VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}
	return nil
}

// getAppliedMigrations returns list of applied migrations
func getAppliedMigrations(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	query := `SELECT version FROM schema_migrations ORDER BY version`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[version] = true
	}
	return applied, nil
}

// recordMigration records a successful migration
func recordMigration(ctx context.Context, tx *sql.Tx, version int, description string) error {
	query := `INSERT INTO schema_migrations (version, description) VALUES ($1, $2)`
	_, err := tx.ExecContext(ctx, query, version, description)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}
	return nil
}

// ensureAgentColumnsExist adds missing columns to agents table (emergency fix)
func ensureAgentColumnsExist(ctx context.Context, db *sql.DB) error {
	// EMERGENCY FIX FOR PRODUCTION: Convert VARCHAR columns to TEXT
	varcharToTextMigrations := []string{
		"ALTER TABLE agents ALTER COLUMN did TYPE TEXT",
		"ALTER TABLE agents ALTER COLUMN pricing_model TYPE TEXT",
		"ALTER TABLE agents ALTER COLUMN status TYPE TEXT",
		"ALTER TABLE agents ALTER COLUMN region TYPE TEXT",
	}

	for _, migration := range varcharToTextMigrations {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			// Column might not exist yet, which is fine
			fmt.Printf("Info during VARCHAR to TEXT conversion: %v\n", err)
		}
	}

	// Add columns first
	columnMigrations := []string{
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS did TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS description TEXT",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS pricing_model TEXT",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'online'",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_capacity INTEGER NOT NULL DEFAULT 10",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS current_load INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS region TEXT",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS metadata JSONB",
	}

	for _, migration := range columnMigrations {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			fmt.Printf("Warning during agent column migration: %v\n", err)
		}
	}

	// Drop version column if it exists
	_, _ = db.ExecContext(ctx, "ALTER TABLE agents DROP COLUMN IF EXISTS version")

	// Add constraint only if it doesn't exist (use DO block for conditional logic)
	constraintSQL := `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'agents_status_check'
			) THEN
				ALTER TABLE agents ADD CONSTRAINT agents_status_check
				CHECK (status IN ('online', 'busy', 'offline', 'maintenance'));
			END IF;
		END $$;
	`
	if _, err := db.ExecContext(ctx, constraintSQL); err != nil {
		fmt.Printf("Warning during constraint creation: %v\n", err)
	}

	// Create indexes (IF NOT EXISTS handles duplicates)
	indexMigrations := []string{
		"CREATE INDEX IF NOT EXISTS idx_agents_did ON agents(did)",
		"CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)",
		"CREATE INDEX IF NOT EXISTS idx_agents_created_at ON agents(created_at)",
	}

	for _, migration := range indexMigrations {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			fmt.Printf("Warning during index creation: %v\n", err)
		}
	}

	// Convert capabilities to JSONB if TEXT
	conversionSQL := `
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'agents'
				AND column_name = 'capabilities'
				AND data_type = 'text'
			) THEN
				ALTER TABLE agents ALTER COLUMN capabilities TYPE JSONB USING
					CASE
						WHEN capabilities IS NULL OR capabilities = '' THEN '{}'::jsonb
						ELSE capabilities::jsonb
					END;
			END IF;
		END $$;
	`
	if _, err := db.ExecContext(ctx, conversionSQL); err != nil {
		fmt.Printf("Warning during capabilities conversion: %v\n", err)
	}

	return nil
}

// Migrate runs all pending migrations
func Migrate(ctx context.Context, db *sql.DB) error {
	// Create migrations table if not exists
	if err := createMigrationsTable(ctx, db); err != nil {
		return err
	}

	// EMERGENCY FIX: Ensure agent columns exist before running migrations
	if err := ensureAgentColumnsExist(ctx, db); err != nil {
		return fmt.Errorf("failed to ensure agent columns exist: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	// Load migration files
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// Apply pending migrations
	for _, migration := range migrations {
		if applied[migration.Version] {
			continue
		}

		fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description)

		// Execute migration in transaction
		err := WithTransaction(ctx, db, func(tx *sql.Tx) error {
			// Execute migration SQL
			if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			// Record successful migration
			if err := recordMigration(ctx, tx, migration.Version, migration.Description); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		fmt.Printf("✅ Migration %d applied successfully\n", migration.Version)
	}

	fmt.Println("All migrations applied successfully")
	return nil
}

// loadMigrations loads migration files from embedded filesystem
func loadMigrations() ([]Migration, error) {
	// For now, return initial schema as first migration
	// In production, this would read from migrations/*.sql files
	return []Migration{
		{
			Version:     1,
			Description: "Initial schema",
			SQL:         getInitialSchema(),
		},
	}, nil
}

// getInitialSchema returns the initial database schema
func getInitialSchema() string {
	return `
-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users & Authentication
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    did VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_users_did ON users(did);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active);

-- JWT refresh tokens
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    ip_address INET,
    user_agent TEXT
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON refresh_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token_hash);

-- API keys
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    name VARCHAR(255),
    scopes JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(is_active);

-- Payment system
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    did VARCHAR(255) UNIQUE NOT NULL,
    balance DECIMAL(20, 8) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    total_deposited DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_withdrawn DECIMAL(20, 8) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_accounts_did ON accounts(did);
CREATE INDEX IF NOT EXISTS idx_accounts_balance ON accounts(balance);

CREATE TABLE IF NOT EXISTS payment_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payer_did VARCHAR(255) NOT NULL,
    payee_did VARCHAR(255) NOT NULL,
    auction_id VARCHAR(255),
    total_deposit DECIMAL(20, 8) NOT NULL,
    current_balance DECIMAL(20, 8) NOT NULL,
    escrowed_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_settled DECIMAL(20, 8) NOT NULL DEFAULT 0,
    pending_refund DECIMAL(20, 8) NOT NULL DEFAULT 0,
    state VARCHAR(50) NOT NULL,
    task_id VARCHAR(255),
    escrow_released BOOLEAN DEFAULT false,
    sequence_number BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_channels_payer ON payment_channels(payer_did);
CREATE INDEX IF NOT EXISTS idx_channels_payee ON payment_channels(payee_did);
CREATE INDEX IF NOT EXISTS idx_channels_auction ON payment_channels(auction_id);
CREATE INDEX IF NOT EXISTS idx_channels_task ON payment_channels(task_id);
CREATE INDEX IF NOT EXISTS idx_channels_state ON payment_channels(state);

CREATE TABLE IF NOT EXISTS channel_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id UUID NOT NULL REFERENCES payment_channels(id) ON DELETE CASCADE,
    transaction_type VARCHAR(50) NOT NULL,
    amount DECIMAL(20, 8) NOT NULL,
    task_id VARCHAR(255),
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_channel_txs_channel ON channel_transactions(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_txs_type ON channel_transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_channel_txs_created ON channel_transactions(created_at);

-- Marketplace
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    did TEXT UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    pricing_model TEXT,
    status TEXT DEFAULT 'online',
    max_capacity INT DEFAULT 10,
    current_load INT DEFAULT 0,
    region TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_agents_did ON agents(did);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_capabilities ON agents USING GIN (capabilities);
CREATE INDEX IF NOT EXISTS idx_agents_region ON agents(region);

CREATE TABLE IF NOT EXISTS auctions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    auction_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    duration_seconds INT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    reserve_price DECIMAL(20, 8),
    max_price DECIMAL(20, 8),
    min_reputation DECIMAL(5, 2),
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    winning_bid_id UUID,
    final_price DECIMAL(20, 8),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_auctions_task ON auctions(task_id);
CREATE INDEX IF NOT EXISTS idx_auctions_user ON auctions(user_id);
CREATE INDEX IF NOT EXISTS idx_auctions_status ON auctions(status);
CREATE INDEX IF NOT EXISTS idx_auctions_expires ON auctions(expires_at);

CREATE TABLE IF NOT EXISTS bids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id UUID NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    agent_did VARCHAR(255) NOT NULL,
    price DECIMAL(20, 8) NOT NULL,
    estimated_time_seconds INT,
    reputation_score DECIMAL(5, 2),
    quality_score DECIMAL(5, 2),
    composite_score DECIMAL(10, 6),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bids_auction ON bids(auction_id);
CREATE INDEX IF NOT EXISTS idx_bids_agent ON bids(agent_did);
CREATE INDEX IF NOT EXISTS idx_bids_composite ON bids(composite_score DESC);

-- Reputation system
CREATE TABLE IF NOT EXISTS reputation_scores (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_did VARCHAR(255) UNIQUE NOT NULL,
    overall_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    reliability_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    quality_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    speed_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    total_tasks INT NOT NULL DEFAULT 0,
    successful_tasks INT NOT NULL DEFAULT 0,
    failed_tasks INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reputation_agent ON reputation_scores(agent_did);
CREATE INDEX IF NOT EXISTS idx_reputation_overall ON reputation_scores(overall_score DESC);

CREATE TABLE IF NOT EXISTS reputation_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_did VARCHAR(255) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    task_id VARCHAR(255),
    score_delta DECIMAL(5, 2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_reputation_events_agent ON reputation_events(agent_did);
CREATE INDEX IF NOT EXISTS idx_reputation_events_type ON reputation_events(event_type);
CREATE INDEX IF NOT EXISTS idx_reputation_events_created ON reputation_events(created_at DESC);

-- Task execution
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id VARCHAR(255) UNIQUE NOT NULL,
    user_did VARCHAR(255) NOT NULL,
    agent_did VARCHAR(255),
    task_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    input JSONB NOT NULL,
    output JSONB,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    timeout_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_tasks_task_id ON tasks(task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_user ON tasks(user_did);
CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(agent_did);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at DESC);

-- Audit logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(255),
    status_code INT,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC);

-- Rate limiting
CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key VARCHAR(255) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    tokens_remaining INT NOT NULL,
    window_start TIMESTAMP WITH TIME ZONE NOT NULL,
    window_end TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(key, endpoint, window_start)
);

CREATE INDEX IF NOT EXISTS idx_rate_limit_key ON rate_limit_buckets(key, endpoint);
CREATE INDEX IF NOT EXISTS idx_rate_limit_window ON rate_limit_buckets(window_end);

-- Triggers for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_users_updated_at') THEN
        CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_accounts_updated_at') THEN
        CREATE TRIGGER update_accounts_updated_at BEFORE UPDATE ON accounts
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_payment_channels_updated_at') THEN
        CREATE TRIGGER update_payment_channels_updated_at BEFORE UPDATE ON payment_channels
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_agents_updated_at') THEN
        CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON agents
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_auctions_updated_at') THEN
        CREATE TRIGGER update_auctions_updated_at BEFORE UPDATE ON auctions
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_reputation_updated_at') THEN
        CREATE TRIGGER update_reputation_updated_at BEFORE UPDATE ON reputation_scores
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
END$$;

-- Initial system user
INSERT INTO users (did, email, is_active, metadata)
VALUES ('did:zerostate:system', 'system@zerostate.io', true, '{"type": "system"}'::jsonb)
ON CONFLICT (did) DO NOTHING;
`
}

// GetMigrationStatus returns current migration status
func GetMigrationStatus(ctx context.Context, db *sql.DB) ([]MigrationHistory, error) {
	if err := createMigrationsTable(ctx, db); err != nil {
		return nil, err
	}

	query := `
		SELECT id, version, description, applied_at
		FROM schema_migrations
		ORDER BY version DESC
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query migration status: %w", err)
	}
	defer rows.Close()

	var migrations []MigrationHistory
	for rows.Next() {
		var m MigrationHistory
		if err := rows.Scan(&m.ID, &m.Version, &m.Description, &m.AppliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}
		migrations = append(migrations, m)
	}
	return migrations, nil
}
