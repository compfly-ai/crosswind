package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage implements FileStorage for local filesystem
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new LocalStorage instance
// basePath defaults to AGENT_EVAL_DATA_DIR env var, then "./data"
func NewLocalStorage(basePath string) *LocalStorage {
	if basePath == "" {
		basePath = os.Getenv("AGENT_EVAL_DATA_DIR")
	}
	if basePath == "" {
		basePath = "./data"
	}

	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		// Log warning but continue - will fail on first write
		fmt.Printf("Warning: could not create data directory %s: %v\n", basePath, err)
	}

	return &LocalStorage{basePath: basePath}
}

// Upload uploads from a reader
func (s *LocalStorage) Upload(ctx context.Context, path string, reader io.Reader, contentType string) error {
	fullPath := s.fullPath(path)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Download returns a reader for a file
func (s *LocalStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := s.fullPath(path)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes a file
func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := s.fullPath(path)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, not an error
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists checks if a file exists
func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := s.fullPath(path)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}

// GetURL returns a URL for accessing the file
// For local storage, returns an API path that will be served by the API server
func (s *LocalStorage) GetURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	// Local storage doesn't support signed URLs
	// Return a path that the API server will serve
	return fmt.Sprintf("/v1/files/%s", path), nil
}

// fullPath returns the full filesystem path for a storage path
func (s *LocalStorage) fullPath(path string) string {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)
	// Remove any leading slashes or dots
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, ".")
	return filepath.Join(s.basePath, cleanPath)
}
