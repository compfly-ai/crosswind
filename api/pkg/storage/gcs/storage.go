package gcs

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"go.uber.org/zap"
)

// GCSStorage implements FileStorage for Google Cloud Storage
type GCSStorage struct {
	client     *storage.Client
	bucketName string
	logger     *zap.Logger
}

// Config holds GCS configuration
type Config struct {
	BucketName string
	ProjectID  string // Optional - uses default credentials project if empty
}

// NewGCSStorage creates a new GCS storage instance
// Uses GOOGLE_APPLICATION_CREDENTIALS env var or Application Default Credentials
func NewGCSStorage(ctx context.Context, cfg Config, logger *zap.Logger) (*GCSStorage, error) {
	if cfg.BucketName == "" {
		return nil, fmt.Errorf("GCS bucket name is required")
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSStorage{
		client:     client,
		bucketName: cfg.BucketName,
		logger:     logger,
	}, nil
}

// Close closes the GCS client
func (s *GCSStorage) Close() error {
	return s.client.Close()
}

// Upload uploads from a reader to GCS
func (s *GCSStorage) Upload(ctx context.Context, path string, reader io.Reader, contentType string) error {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(path)

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := io.Copy(writer, reader); err != nil {
		writer.Close()
		return fmt.Errorf("failed to upload file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	if s.logger != nil {
		s.logger.Debug("uploaded file to GCS",
			zap.String("bucket", s.bucketName),
			zap.String("path", path),
			zap.String("contentType", contentType),
		)
	}

	return nil
}

// Download returns a reader for a file from GCS
func (s *GCSStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(path)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return reader, nil
}

// Delete removes a file from GCS
func (s *GCSStorage) Delete(ctx context.Context, path string) error {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(path)

	if err := obj.Delete(ctx); err != nil {
		if err == storage.ErrObjectNotExist {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	if s.logger != nil {
		s.logger.Debug("deleted file from GCS",
			zap.String("bucket", s.bucketName),
			zap.String("path", path),
		)
	}

	return nil
}

// Exists checks if a file exists in GCS
func (s *GCSStorage) Exists(ctx context.Context, path string) (bool, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(path)

	_, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetURL returns a signed URL for accessing the file
func (s *GCSStorage) GetURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	bucket := s.client.Bucket(s.bucketName)

	opts := &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	}

	url, err := bucket.SignedURL(path, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}
