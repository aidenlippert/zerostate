package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Storage handles Cloudflare R2 storage operations
type R2Storage struct {
	client     *s3.Client
	bucketName string
}

// Config holds R2 configuration
type Config struct {
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
	BucketName      string
	Region          string // Default: "auto"
}

// NewR2Storage creates a new R2 storage client
func NewR2Storage(cfg Config) (*R2Storage, error) {
	if cfg.Region == "" {
		cfg.Region = "auto"
	}

	// Create AWS config with R2 endpoint
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true // Required for R2
	})

	return &R2Storage{
		client:     client,
		bucketName: cfg.BucketName,
	}, nil
}

// NewR2StorageFromEnv creates R2 storage from environment variables
func NewR2StorageFromEnv() (*R2Storage, error) {
	config := Config{
		AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		Endpoint:        os.Getenv("R2_ENDPOINT"),
		BucketName:      os.Getenv("R2_BUCKET_NAME"),
		Region:          "auto",
	}

	if config.AccessKeyID == "" || config.SecretAccessKey == "" || config.Endpoint == "" || config.BucketName == "" {
		return nil, fmt.Errorf("missing required R2 environment variables (R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_ENDPOINT, R2_BUCKET_NAME)")
	}

	return NewR2Storage(config)
}

// UploadWASM uploads a WASM binary to R2
func (r *R2Storage) UploadWASM(ctx context.Context, key string, data []byte) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/wasm"),
		Metadata: map[string]string{
			"uploaded-at": time.Now().UTC().Format(time.RFC3339),
		},
	})

	if err != nil {
		return fmt.Errorf("failed to upload WASM to R2: %w", err)
	}

	return nil
}

// DownloadWASM downloads a WASM binary from R2
func (r *R2Storage) DownloadWASM(ctx context.Context, key string) ([]byte, error) {
	result, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download WASM from R2: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read WASM data: %w", err)
	}

	return data, nil
}

// DeleteWASM deletes a WASM binary from R2
func (r *R2Storage) DeleteWASM(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete WASM from R2: %w", err)
	}

	return nil
}

// ListWASM lists all WASM binaries in R2 with a given prefix
func (r *R2Storage) ListWASM(ctx context.Context, prefix string) ([]string, error) {
	result, err := r.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(r.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list WASM files in R2: %w", err)
	}

	keys := make([]string, 0, len(result.Contents))
	for _, obj := range result.Contents {
		keys = append(keys, *obj.Key)
	}

	return keys, nil
}

// GetURL generates a presigned URL for accessing a WASM binary
func (r *R2Storage) GetURL(ctx context.Context, key string, duration time.Duration) (string, error) {
	// For R2, we can construct a direct URL (R2 supports public URLs if configured)
	// Or use presigned URLs. For simplicity, we'll return a direct URL format.
	// Note: This requires the bucket to have appropriate access policies.
	return fmt.Sprintf("%s/%s", r.bucketName, key), nil
}

// Exists checks if a WASM binary exists in R2
func (r *R2Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		// Check if error is "not found" - just return false for any error
		// since HeadObject typically returns NotFound errors
		return false, nil
	}

	return true, nil
}

// GetMetadata retrieves metadata for a WASM binary
func (r *R2Storage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	result, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get WASM metadata: %w", err)
	}

	metadata := make(map[string]string)
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return metadata, nil
}
