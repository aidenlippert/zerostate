package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// FAANG-LEVEL DATABASE CONNECTION MANAGEMENT
// Following best practices:
// - Connection pooling with configurable limits
// - Health checks and connection validation
// - Automatic reconnection on failure
// - Proper cleanup and resource management
// - Timeout configuration for operations
// - SSL/TLS support for production

// Config holds database configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string // disable, require, verify-ca, verify-full
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
}

// DefaultConfig returns default database configuration
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            5432,
		User:            "zerostate",
		Password:        "",
		Database:        "zerostate",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	}
}

// ProductionConfig returns production database configuration
func ProductionConfig() *Config {
	cfg := DefaultConfig()
	cfg.SSLMode = "verify-full" // CRITICAL: Require SSL in production
	cfg.MaxOpenConns = 100      // Higher concurrency for production
	cfg.MaxIdleConns = 20
	return cfg
}

// Connect establishes a database connection with proper configuration
func Connect(cfg *Config) (*sql.DB, error) {
	// Build connection string
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
		int(cfg.ConnectTimeout.Seconds()),
	)

	// Open database connection
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// HealthCheck performs a database health check
func HealthCheck(ctx context.Context, db *sql.DB) error {
	// Execute simple query to verify connection
	var result int
	err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	if result != 1 {
		return fmt.Errorf("unexpected health check result: %d", result)
	}
	return nil
}

// GetStats returns database connection pool statistics
func GetStats(db *sql.DB) sql.DBStats {
	return db.Stats()
}

// Close gracefully closes the database connection
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}

// WithTransaction executes a function within a database transaction
func WithTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute function
	err = fn(tx)
	if err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %w (rollback error: %v)", err, rbErr)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
