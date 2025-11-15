package database

import (
	"context"
	"fmt"
	"strings"
)

// InitializeSQLiteSchema creates all tables for SQLite (development mode)
// This is a simplified version of the PostgreSQL migrations for local testing
func (d *Database) InitializeSQLiteSchema(ctx context.Context) error {
	schema := `
		-- Users table
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			did TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT,
			is_active INTEGER DEFAULT 1,
			last_login_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			metadata TEXT DEFAULT '{}'
		);

		-- Agents table
		CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			did TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			capabilities TEXT NOT NULL DEFAULT '[]',
			pricing_model TEXT,
			status TEXT DEFAULT 'online',
			max_capacity INTEGER DEFAULT 100,
			current_load INTEGER DEFAULT 0,
			region TEXT DEFAULT 'global',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME,
			metadata TEXT DEFAULT '{}'
		);

		-- Tasks table
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			description TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			agent_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			result TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (agent_id) REFERENCES agents(id)
		);

		-- Payment channels table
		CREATE TABLE IF NOT EXISTS payment_channels (
			id TEXT PRIMARY KEY,
			payer_id TEXT NOT NULL,
			payee_id TEXT NOT NULL,
			balance REAL DEFAULT 0,
			status TEXT DEFAULT 'open',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (payer_id) REFERENCES users(id),
			FOREIGN KEY (payee_id) REFERENCES agents(id)
		);

		-- Reputation scores table
		CREATE TABLE IF NOT EXISTS reputation_scores (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			score REAL DEFAULT 0,
			total_tasks INTEGER DEFAULT 0,
			successful_tasks INTEGER DEFAULT 0,
			failed_tasks INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id)
		);

		-- Auctions table
		CREATE TABLE IF NOT EXISTS auctions (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			status TEXT DEFAULT 'open',
			min_bid REAL,
			max_bid REAL,
			winner_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME,
			FOREIGN KEY (task_id) REFERENCES tasks(id),
			FOREIGN KEY (winner_id) REFERENCES agents(id)
		);

		-- Bids table
		CREATE TABLE IF NOT EXISTS bids (
			id TEXT PRIMARY KEY,
			auction_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			amount REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (auction_id) REFERENCES auctions(id),
			FOREIGN KEY (agent_id) REFERENCES agents(id)
		);

		-- Schema migrations tracking
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER UNIQUE NOT NULL,
			description TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Agent keys table (Sprint 3 - blockchain key management)
		CREATE TABLE IF NOT EXISTS agent_keys (
			id TEXT PRIMARY KEY,
			agent_did TEXT NOT NULL,
			public_key BLOB NOT NULL,
			encrypted_private_key BLOB NOT NULL,
			key_type TEXT DEFAULT 'ed25519',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			rotated_at DATETIME,
			expires_at DATETIME,
			is_active INTEGER DEFAULT 1,
			FOREIGN KEY (agent_did) REFERENCES agents(did)
		);
		CREATE INDEX IF NOT EXISTS idx_agent_keys_agent_did ON agent_keys(agent_did);
		CREATE INDEX IF NOT EXISTS idx_agent_keys_active ON agent_keys(agent_did, is_active);

		-- Insert initial migration record
		INSERT OR IGNORE INTO schema_migrations (version, description)
		VALUES (1, 'Initial SQLite schema');
	`

	_, err := d.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite schema: %w", err)
	}

	return nil
}

// IsSQLite checks if the database is SQLite
func (d *Database) IsSQLite() bool {
	// Try a SQLite-specific query
	var result string
	err := d.db.QueryRow("SELECT sqlite_version()").Scan(&result)
	return err == nil && result != ""
}

// IsPostgreSQL checks if the database is PostgreSQL
func (d *Database) IsPostgreSQL() bool {
	// Try a PostgreSQL-specific query
	var result string
	err := d.db.QueryRow("SELECT version()").Scan(&result)
	if err != nil {
		return false
	}
	// PostgreSQL version string starts with "PostgreSQL"
	return len(result) > 10 && result[:10] == "PostgreSQL"
}

// ConvertPlaceholders converts PostgreSQL-style placeholders ($1, $2) to SQLite-style (?)
// Only needed for SQLite; PostgreSQL queries pass through unchanged
func (d *Database) ConvertPlaceholders(query string) string {
	if d.IsPostgreSQL() {
		return query
	}
	// For SQLite, replace $1, $2, etc. with ?
	// Start from higher numbers to avoid replacing $1 in $10, $11, etc.
	result := query
	for i := 99; i >= 1; i-- {
		result = strings.Replace(result, fmt.Sprintf("$%d", i), "?", -1)
	}
	return result
}
