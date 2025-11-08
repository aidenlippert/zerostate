package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// DB wraps the database connection
type DB struct {
	conn       *sql.DB
	driverName string
}

// User represents a user in the database
type User struct {
	ID           string    `json:"id"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Don't expose password hash in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewDB creates a new database connection
// Supports both SQLite (local dev) and PostgreSQL (production/Supabase)
// Examples:
//   - SQLite: NewDB("./zerostate.db")
//   - PostgreSQL: NewDB("postgres://user:pass@host:5432/dbname")
func NewDB(connectionString string) (*DB, error) {
	var driverName string
	var conn *sql.DB
	var err error

	// Detect database type from connection string
	if strings.HasPrefix(connectionString, "postgres://") || strings.HasPrefix(connectionString, "postgresql://") {
		driverName = "postgres"
		conn, err = sql.Open("postgres", connectionString)
	} else {
		driverName = "sqlite3"
		conn, err = sql.Open("sqlite3", connectionString)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn:       conn,
		driverName: driverName,
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// initSchema creates the database tables
func (db *DB) initSchema() error {
	var schema string

	if db.driverName == "postgres" {
		schema = `
		CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			full_name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		`
	} else {
		schema = `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			full_name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		`
	}

	_, err := db.conn.Exec(schema)
	return err
}

// placeholder returns the correct parameter placeholder for the database driver
func (db *DB) placeholder(n int) string {
	if db.driverName == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// CreateUser creates a new user
func (db *DB) CreateUser(user *User) error {
	var query string
	if db.driverName == "postgres" {
		query = `
			INSERT INTO users (id, full_name, email, password_hash, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
	} else {
		query = `
			INSERT INTO users (id, full_name, email, password_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
	}

	_, err := db.conn.Exec(
		query,
		user.ID,
		user.FullName,
		user.Email,
		user.PasswordHash,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByEmail retrieves a user by email
func (db *DB) GetUserByEmail(email string) (*User, error) {
	var query string
	if db.driverName == "postgres" {
		query = `SELECT id, full_name, email, password_hash, created_at, updated_at FROM users WHERE email = $1`
	} else {
		query = `SELECT id, full_name, email, password_hash, created_at, updated_at FROM users WHERE email = ?`
	}

	user := &User{}
	err := db.conn.QueryRow(query, email).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(id string) (*User, error) {
	var query string
	if db.driverName == "postgres" {
		query = `SELECT id, full_name, email, password_hash, created_at, updated_at FROM users WHERE id = $1`
	} else {
		query = `SELECT id, full_name, email, password_hash, created_at, updated_at FROM users WHERE id = ?`
	}

	user := &User{}
	err := db.conn.QueryRow(query, id).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// UpdateUser updates a user
func (db *DB) UpdateUser(user *User) error {
	var query string
	if db.driverName == "postgres" {
		query = `UPDATE users SET full_name = $1, email = $2, password_hash = $3, updated_at = $4 WHERE id = $5`
	} else {
		query = `UPDATE users SET full_name = ?, email = ?, password_hash = ?, updated_at = ? WHERE id = ?`
	}

	user.UpdatedAt = time.Now()

	_, err := db.conn.Exec(
		query,
		user.FullName,
		user.Email,
		user.PasswordHash,
		user.UpdatedAt,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeleteUser deletes a user
func (db *DB) DeleteUser(id string) error {
	var query string
	if db.driverName == "postgres" {
		query = `DELETE FROM users WHERE id = $1`
	} else {
		query = `DELETE FROM users WHERE id = ?`
	}

	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
