package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// GCSClient is a wrapper that provides GCS-compatible API using the FileStorage interface.
// This allows context_service.go to work with any storage backend.
type GCSClient struct {
	storage FileStorage
}

// NewGCSClient creates a new GCSClient wrapper around any FileStorage
func NewGCSClient(storage FileStorage) *GCSClient {
	return &GCSClient{storage: storage}
}

// UploadFile uploads a file
func (g *GCSClient) UploadFile(ctx context.Context, objectName string, reader io.Reader, contentType string) error {
	return g.storage.Upload(ctx, objectName, reader, contentType)
}

// DownloadFile downloads a file
func (g *GCSClient) DownloadFile(ctx context.Context, objectName string) (io.ReadCloser, error) {
	return g.storage.Download(ctx, objectName)
}

// DeletePrefix deletes all files with a given prefix
// Note: This is a simplified implementation for local storage
func (g *GCSClient) DeletePrefix(ctx context.Context, prefix string) error {
	// For local storage, we just delete the directory
	// This is a simplified version - a full implementation would iterate over files
	return g.storage.Delete(ctx, prefix)
}

// FileExists checks if a file exists
func (g *GCSClient) FileExists(ctx context.Context, objectName string) (bool, error) {
	return g.storage.Exists(ctx, objectName)
}

// GenerateSignedURL generates a URL for file access
func (g *GCSClient) GenerateSignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	return g.storage.GetURL(ctx, objectName, expiry)
}

// FileMetadata represents metadata for a stored file
type FileMetadata struct {
	Name        string    `json:"name"`
	FullPath    string    `json:"fullPath,omitempty"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// BuildContextPath builds the storage path for a context
func BuildContextPath(contextID string) string {
	return fmt.Sprintf("contexts/%s", contextID)
}

// BuildFilePath builds the full storage path for a file within a context
func BuildFilePath(contextID, fileName string) string {
	return fmt.Sprintf("contexts/%s/%s", contextID, fileName)
}

// BuildProcessedPath builds the path for processed/extracted content
func BuildProcessedPath(contextID string) string {
	return fmt.Sprintf("contexts/%s/processed/extracted.json", contextID)
}
