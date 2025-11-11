package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RunMigrations executes all pending database migrations
func (db *Database) RunMigrations(ctx context.Context) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "fix_agents_varchar_constraints",
			sql: `
				-- Fix VARCHAR length constraints in agents table
				ALTER TABLE agents ALTER COLUMN name TYPE TEXT;
				ALTER TABLE agents ALTER COLUMN description TYPE TEXT;
				ALTER TABLE agents ALTER COLUMN pricing_model TYPE TEXT;
				ALTER TABLE agents ALTER COLUMN did TYPE TEXT;
				ALTER TABLE agents ALTER COLUMN status TYPE TEXT;
				ALTER TABLE agents ALTER COLUMN region TYPE TEXT;

				-- Add missing columns
				ALTER TABLE agents ADD COLUMN IF NOT EXISTS wasm_hash TEXT DEFAULT '';
				ALTER TABLE agents ADD COLUMN IF NOT EXISTS s3_key TEXT DEFAULT '';

				-- Create indexes
				CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
				CREATE INDEX IF NOT EXISTS idx_agents_owner_id ON agents(owner_id);
			`,
		},
	}

	for _, migration := range migrations {
		fmt.Printf("Running migration: %s\n", migration.name)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if _, err := db.db.ExecContext(ctx, migration.sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.name, err)
		}

		fmt.Printf("✅ Migration %s completed successfully\n", migration.name)
	}

	return nil
}

// VerifySchema checks if the schema is correct
func (db *Database) VerifySchema(ctx context.Context) error {
	query := `
		SELECT
		  column_name,
		  data_type,
		  character_maximum_length
		FROM information_schema.columns
		WHERE table_name = 'agents'
		AND column_name IN ('name', 'description', 'pricing_model', 'did', 'status', 'region', 'wasm_hash', 's3_key')
		ORDER BY column_name
	`

	rows, err := db.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to verify schema: %w", err)
	}
	defer rows.Close()

	fmt.Println("\n📊 Schema verification:")
	fmt.Printf("%-20s %-15s %-10s\n", "Column", "Type", "Max Length")
	fmt.Println("--------------------------------------------------------")

	for rows.Next() {
		var columnName, dataType string
		var maxLength sql.NullInt64
		if err := rows.Scan(&columnName, &dataType, &maxLength); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		maxLenStr := "NULL"
		if maxLength.Valid {
			maxLenStr = fmt.Sprintf("%d", maxLength.Int64)
		}
		fmt.Printf("%-20s %-15s %-10s\n", columnName, dataType, maxLenStr)
	}

	return rows.Err()
}
