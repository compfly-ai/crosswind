package storage

import (
	"context"
	"io"
	"time"
)

// FileStorage defines operations for file storage (contexts, uploads)
type FileStorage interface {
	// Upload uploads from a reader
	Upload(ctx context.Context, path string, reader io.Reader, contentType string) error

	// Download returns a reader for a file
	Download(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes a file
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists
	Exists(ctx context.Context, path string) (bool, error)

	// GetURL returns a URL for accessing the file
	// For local storage: returns /files/{path} (served by API)
	// For GCS: returns a signed URL
	GetURL(ctx context.Context, path string, expiry time.Duration) (string, error)
}

// FileInfo represents metadata about a stored file
type FileInfo struct {
	Path         string
	Size         int64
	ContentType  string
	LastModified time.Time
}

// Provider identifies the storage provider
type Provider string

const (
	ProviderLocal Provider = "local"
	ProviderGCS   Provider = "gcs"
)
