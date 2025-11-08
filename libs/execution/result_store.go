package execution

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// PostgresResultStore implements ResultStore using PostgreSQL
type PostgresResultStore struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewPostgresResultStore creates a new PostgreSQL result store
func NewPostgresResultStore(db *sql.DB, logger *zap.Logger) *PostgresResultStore {
	return &PostgresResultStore{
		db:     db,
		logger: logger,
	}
}

// StoreResult stores a task execution result in the database
func (s *PostgresResultStore) StoreResult(ctx context.Context, result *TaskResult) error {
	query := `
		INSERT INTO task_results (
			task_id,
			agent_id,
			exit_code,
			stdout,
			stderr,
			duration_ms,
			error,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (task_id) DO UPDATE SET
			exit_code = EXCLUDED.exit_code,
			stdout = EXCLUDED.stdout,
			stderr = EXCLUDED.stderr,
			duration_ms = EXCLUDED.duration_ms,
			error = EXCLUDED.error,
			created_at = EXCLUDED.created_at
	`

	durationMs := result.Duration.Milliseconds()

	_, err := s.db.ExecContext(ctx, query,
		result.TaskID,
		result.AgentID,
		result.ExitCode,
		result.Stdout,
		result.Stderr,
		durationMs,
		result.Error,
		result.CreatedAt,
	)

	if err != nil {
		s.logger.Error("failed to store task result",
			zap.String("task_id", result.TaskID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to store result: %w", err)
	}

	s.logger.Info("stored task result",
		zap.String("task_id", result.TaskID),
		zap.Int("exit_code", result.ExitCode),
		zap.Int64("duration_ms", durationMs),
	)

	return nil
}

// GetResult retrieves a task execution result from the database
func (s *PostgresResultStore) GetResult(ctx context.Context, taskID string) (*TaskResult, error) {
	query := `
		SELECT
			task_id,
			agent_id,
			exit_code,
			stdout,
			stderr,
			duration_ms,
			error,
			created_at
		FROM task_results
		WHERE task_id = $1
	`

	var result TaskResult
	var durationMs int64
	var errorStr sql.NullString

	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&result.TaskID,
		&result.AgentID,
		&result.ExitCode,
		&result.Stdout,
		&result.Stderr,
		&durationMs,
		&errorStr,
		&result.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("result not found for task %s", taskID)
	}
	if err != nil {
		s.logger.Error("failed to get task result",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	result.Duration = time.Duration(durationMs) * time.Millisecond
	if errorStr.Valid {
		result.Error = errorStr.String
	}

	return &result, nil
}

// ListResults retrieves task results with optional filtering
func (s *PostgresResultStore) ListResults(ctx context.Context, agentID string, limit string, offset string) ([]*TaskResult, error) {
	var query string
	var args []interface{}

	if agentID != "" {
		query = `
			SELECT
				task_id,
				agent_id,
				exit_code,
				stdout,
				stderr,
				duration_ms,
				error,
				created_at
			FROM task_results
			WHERE agent_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{agentID, limit, offset}
	} else {
		query = `
			SELECT
				task_id,
				agent_id,
				exit_code,
				stdout,
				stderr,
				duration_ms,
				error,
				created_at
			FROM task_results
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = []interface{}{limit, offset}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to list task results", zap.Error(err))
		return nil, fmt.Errorf("failed to list results: %w", err)
	}
	defer rows.Close()

	var results []*TaskResult
	for rows.Next() {
		var result TaskResult
		var durationMs int64
		var errorStr sql.NullString

		err := rows.Scan(
			&result.TaskID,
			&result.AgentID,
			&result.ExitCode,
			&result.Stdout,
			&result.Stderr,
			&durationMs,
			&errorStr,
			&result.CreatedAt,
		)
		if err != nil {
			s.logger.Error("failed to scan result row", zap.Error(err))
			continue
		}

		result.Duration = time.Duration(durationMs) * time.Millisecond
		result.DurationMs = durationMs
		if errorStr.Valid {
			result.Error = errorStr.String
		}

		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("error iterating results", zap.Error(err))
		return nil, fmt.Errorf("failed to iterate results: %w", err)
	}

	return results, nil
}

// InitResultsTable creates the task_results table if it doesn't exist
func (s *PostgresResultStore) InitResultsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS task_results (
			task_id VARCHAR(255) PRIMARY KEY,
			agent_id VARCHAR(255) NOT NULL,
			exit_code INTEGER NOT NULL,
			stdout BYTEA,
			stderr BYTEA,
			duration_ms BIGINT NOT NULL,
			error TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			INDEX idx_task_results_agent_id (agent_id),
			INDEX idx_task_results_created_at (created_at)
		)
	`

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		s.logger.Error("failed to create task_results table", zap.Error(err))
		return fmt.Errorf("failed to init table: %w", err)
	}

	s.logger.Info("task_results table initialized")
	return nil
}
