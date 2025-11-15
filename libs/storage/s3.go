package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"
)

// S3Storage implements cloud storage using AWS S3
type S3Storage struct {
	client    *s3.Client
	bucket    string
	region    string
	endpoint  string
	logger    *zap.Logger
	urlExpiry time.Duration
}

// S3Config configures S3 storage
type S3Config struct {
	Bucket          string        // S3 bucket name
	Region          string        // AWS region (e.g., "us-east-1")
	AccessKeyID     string        // AWS access key ID
	SecretAccessKey string        // AWS secret access key
	Endpoint        string        // Custom endpoint (for LocalStack/MinIO)
	URLExpiry       time.Duration // Signed URL expiration (default: 1 hour)
}

// DefaultS3Config returns default S3 configuration
func DefaultS3Config() *S3Config {
	return &S3Config{
		Bucket:    "zerostate-agents",
		Region:    "us-east-1",
		URLExpiry: 1 * time.Hour,
	}
}

// NewS3Storage creates a new S3 storage client
func NewS3Storage(ctx context.Context, cfg *S3Config, logger *zap.Logger) (*S3Storage, error) {
	if cfg == nil {
		cfg = DefaultS3Config()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Load AWS configuration
	var awsCfg aws.Config
	var err error

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use explicit credentials
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				"",
			)),
		)
	} else {
		// Use default credential chain (IAM role, env vars, etc.)
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for LocalStack/MinIO
		}
	})

	// Test connection by checking if bucket exists
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		logger.Warn("bucket does not exist or is not accessible",
			zap.String("bucket", cfg.Bucket),
			zap.Error(err),
		)
	}

	logger.Info("S3 storage initialized",
		zap.String("bucket", cfg.Bucket),
		zap.String("region", cfg.Region),
	)

	return &S3Storage{
		client:    s3Client,
		bucket:    cfg.Bucket,
		region:    cfg.Region,
		endpoint:  cfg.Endpoint,
		logger:    logger,
		urlExpiry: cfg.URLExpiry,
	}, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	// Upload to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		ACL:         types.ObjectCannedACLPrivate, // Private by default
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	s.logger.Info("file uploaded to S3",
		zap.String("bucket", s.bucket),
		zap.String("key", key),
		zap.Int("size", len(data)),
	)

	// Generate permanent URL
	var url string
	if s.endpoint != "" {
		// For custom endpoints (R2, MinIO, LocalStack), use path-style URL
		url = fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	} else {
		// For AWS S3, use virtual-hosted-style URL
		url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	}

	return url, nil
}

// Download retrieves a file from S3
func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	// Get object from S3
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	// Read all data
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object: %w", err)
	}

	s.logger.Info("file downloaded from S3",
		zap.String("bucket", s.bucket),
		zap.String("key", key),
		zap.Int("size", len(data)),
	)

	return data, nil
}

// Delete removes a file from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	s.logger.Info("file deleted from S3",
		zap.String("bucket", s.bucket),
		zap.String("key", key),
	)

	return nil
}

// GetSignedURL generates a pre-signed URL for temporary access
func (s *S3Storage) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if expiry == 0 {
		expiry = s.urlExpiry
	}

	// Create presign client
	presignClient := s3.NewPresignClient(s.client)

	// Generate presigned GET URL
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	s.logger.Info("signed URL generated",
		zap.String("key", key),
		zap.Duration("expiry", expiry),
	)

	return presignResult.URL, nil
}

// Exists checks if a file exists in S3
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if error is "not found"
		return false, nil
	}
	return true, nil
}

// ListVersions returns all versions of a file
func (s *S3Storage) ListVersions(ctx context.Context, key string) ([]string, error) {
	result, err := s.client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	versions := make([]string, 0, len(result.Versions))
	for _, version := range result.Versions {
		if version.VersionId != nil {
			versions = append(versions, *version.VersionId)
		}
	}

	return versions, nil
}

// GetMetadata retrieves file metadata without downloading
func (s *S3Storage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	metadata := make(map[string]string)
	if result.ContentType != nil {
		metadata["content_type"] = *result.ContentType
	}
	if result.ContentLength != nil {
		metadata["size"] = fmt.Sprintf("%d", *result.ContentLength)
	}
	if result.LastModified != nil {
		metadata["last_modified"] = result.LastModified.Format(time.RFC3339)
	}
	if result.ETag != nil {
		metadata["etag"] = *result.ETag
	}

	return metadata, nil
}
