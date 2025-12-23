package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/compfly-ai/crosswind/pkg/storage/gcs"
	"github.com/compfly-ai/crosswind/pkg/storage/local"
	"go.uber.org/zap"
)

// Config holds storage configuration
type Config struct {
	// Provider: "local" (default) or "gcs"
	Provider Provider

	// Local storage settings
	LocalPath string

	// GCS settings (optional)
	GCSBucket    string
	GCSProjectID string

	// Logger for storage operations (optional)
	Logger *zap.Logger
}

// StorageConfig is used by the main package to configure storage
type StorageConfig struct {
	Provider       string
	LocalBasePath  string
	GCSBucket      string
	GCSCredentials string
}

// LoadConfig loads storage configuration from environment variables
func LoadConfig() *Config {
	provider := Provider(os.Getenv("STORAGE_PROVIDER"))
	if provider == "" {
		provider = ProviderLocal
	}

	return &Config{
		Provider:     provider,
		LocalPath:    os.Getenv("AGENT_EVAL_DATA_DIR"),
		GCSBucket:    os.Getenv("GCS_BUCKET"),
		GCSProjectID: os.Getenv("GCS_PROJECT_ID"),
	}
}

// NewFileStorage creates a FileStorage based on configuration
func NewFileStorage(cfg *Config) (FileStorage, error) {
	return NewFileStorageWithContext(context.Background(), cfg)
}

// NewFileStorageWithContext creates a FileStorage with a context (needed for GCS)
func NewFileStorageWithContext(ctx context.Context, cfg *Config) (FileStorage, error) {
	switch cfg.Provider {
	case ProviderLocal, "":
		return local.NewLocalStorage(cfg.LocalPath), nil

	case ProviderGCS:
		if cfg.GCSBucket == "" {
			return nil, fmt.Errorf("GCS_BUCKET is required when STORAGE_PROVIDER=gcs")
		}
		gcsCfg := gcs.Config{
			BucketName: cfg.GCSBucket,
			ProjectID:  cfg.GCSProjectID,
		}
		return gcs.NewGCSStorage(ctx, gcsCfg, cfg.Logger)

	default:
		return nil, fmt.Errorf("unknown storage provider: %s", cfg.Provider)
	}
}

// NewStorage creates a FileStorage from StorageConfig (used by main)
func NewStorage(cfg StorageConfig) (FileStorage, error) {
	return NewStorageWithContext(context.Background(), cfg)
}

// NewStorageWithContext creates a FileStorage from StorageConfig with a context
func NewStorageWithContext(ctx context.Context, cfg StorageConfig) (FileStorage, error) {
	switch cfg.Provider {
	case "local", "":
		return local.NewLocalStorage(cfg.LocalBasePath), nil

	case "gcs":
		if cfg.GCSBucket == "" {
			return nil, fmt.Errorf("GCS_BUCKET is required when STORAGE_PROVIDER=gcs")
		}
		gcsCfg := gcs.Config{
			BucketName: cfg.GCSBucket,
		}
		return gcs.NewGCSStorage(ctx, gcsCfg, nil)

	default:
		return nil, fmt.Errorf("unknown storage provider: %s", cfg.Provider)
	}
}
