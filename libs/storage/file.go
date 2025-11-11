package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// FileStorage implements local filesystem storage for testing
type FileStorage struct {
	basePath string
	logger   *zap.Logger
}

// FileConfig configures file storage
type FileConfig struct {
	BasePath string // Base directory for file storage (default: ./data/agents)
}

// DefaultFileConfig returns default file storage configuration
func DefaultFileConfig() *FileConfig {
	return &FileConfig{
		BasePath: "./data/agents",
	}
}

// NewFileStorage creates a new file-based storage client
func NewFileStorage(ctx context.Context, cfg *FileConfig, logger *zap.Logger) (*FileStorage, error) {
	if cfg == nil {
		cfg = DefaultFileConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	logger.Info("file storage initialized",
		zap.String("base_path", cfg.BasePath),
	)

	return &FileStorage{
		basePath: cfg.BasePath,
		logger:   logger,
	}, nil
}

// Upload saves a file to the filesystem
func (f *FileStorage) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	filePath := filepath.Join(f.basePath, key)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	f.logger.Info("file uploaded to filesystem",
		zap.String("path", filePath),
		zap.Int("size", len(data)),
	)

	return filePath, nil
}

// Download retrieves a file from the filesystem
func (f *FileStorage) Download(ctx context.Context, key string) ([]byte, error) {
	filePath := filepath.Join(f.basePath, key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	f.logger.Info("file downloaded from filesystem",
		zap.String("path", filePath),
		zap.Int("size", len(data)),
	)

	return data, nil
}

// Delete removes a file from the filesystem
func (f *FileStorage) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(f.basePath, key)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	f.logger.Info("file deleted from filesystem",
		zap.String("path", filePath),
	)

	return nil
}

// GetSignedURL is not applicable for file storage but returns the file path
func (f *FileStorage) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	filePath := filepath.Join(f.basePath, key)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", key)
		}
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// Return file path as "URL"
	return "file://" + filePath, nil
}

// Exists checks if a file exists in the filesystem
func (f *FileStorage) Exists(ctx context.Context, key string) (bool, error) {
	filePath := filepath.Join(f.basePath, key)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	return true, nil
}

// GetMetadata retrieves file metadata
func (f *FileStorage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	filePath := filepath.Join(f.basePath, key)

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	metadata := map[string]string{
		"size":          fmt.Sprintf("%d", info.Size()),
		"last_modified": info.ModTime().Format(time.RFC3339),
		"path":          filePath,
	}

	return metadata, nil
}
