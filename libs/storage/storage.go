package storage

import (
	"context"
	"time"
)

// Storage defines the interface for binary storage backends
type Storage interface {
	// Upload saves a file to storage
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)

	// Download retrieves a file from storage
	Download(ctx context.Context, key string) ([]byte, error)

	// Delete removes a file from storage
	Delete(ctx context.Context, key string) error

	// GetSignedURL generates a URL for accessing the file
	GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)

	// Exists checks if a file exists in storage
	Exists(ctx context.Context, key string) (bool, error)

	// GetMetadata retrieves file metadata
	GetMetadata(ctx context.Context, key string) (map[string]string, error)
}
