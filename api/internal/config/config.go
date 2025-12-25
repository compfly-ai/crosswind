package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the API service
type Config struct {
	// Server
	Port        string
	Environment string

	// MongoDB
	MongoURI     string
	DatabaseName string

	// Redis
	RedisURL string

	// ClickHouse (Native protocol, port 9440)
	ClickHouseHost     string
	ClickHouseDatabase string
	ClickHouseUser     string
	ClickHousePassword string

	// Security
	EncryptionKey     string
	APIKey            string // Static API key for authentication
	DisableOrgAPIKeys bool   // When true, use single API key instead of per-org keys

	// Docs Auth (Basic Auth for /docs and /openapi.yaml)
	DocsUsername string
	DocsPassword string

	// LLM
	OpenAIKey string // For scenario generation

	// GCP Cloud Storage (uses GOOGLE_APPLICATION_CREDENTIALS env var)
	GCSBucketName string // Bucket for context documents

	// Rate Limiting Defaults
	DefaultRequestsPerMinute  int
	DefaultConcurrentSessions int
	DefaultTimeoutSeconds     int
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (check parent dirs for monorepo structure)
	// Errors are ignored as .env files are optional
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")

	// Build Redis URL from REDIS_URL or REDIS_HOST/REDIS_PASSWORD
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisHost := os.Getenv("REDIS_HOST")
		redisPassword := os.Getenv("REDIS_PASSWORD")
		if redisHost != "" {
			if redisPassword != "" {
				redisURL = fmt.Sprintf("redis://:%s@%s", redisPassword, redisHost)
			} else {
				redisURL = fmt.Sprintf("redis://%s", redisHost)
			}
		} else {
			redisURL = "redis://localhost:6379"
		}
	}

	cfg := &Config{
		Port:                      getEnv("SERVER_PORT", getEnv("PORT", "8080")),
		Environment:               getEnv("ENVIRONMENT", "development"),
		MongoURI:                  getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DatabaseName:              getEnv("DATABASE_NAME", "agent_eval"),
		RedisURL:                  redisURL,
		ClickHouseHost:            os.Getenv("CLICKHOUSE_HOST"), // e.g., host:9440
		ClickHouseDatabase:        getEnv("CLICKHOUSE_DATABASE", "agent_eval"),
		ClickHouseUser:            getEnv("CLICKHOUSE_USER", "default"),
		ClickHousePassword:        os.Getenv("CLICKHOUSE_PASSWORD"),
		EncryptionKey:             os.Getenv("ENCRYPTION_KEY"),
		APIKey:                    os.Getenv("API_KEY"),
		DisableOrgAPIKeys:         getEnvBool("DISABLE_ORG_API_KEYS", true),
		DocsUsername:              os.Getenv("DOCS_USERNAME"),
		DocsPassword:              os.Getenv("DOCS_PASSWORD"),
		OpenAIKey:                 os.Getenv("OPENAI_API_KEY"),
		GCSBucketName:             getEnv("GCS_BUCKET_NAME", "agent-eval-contexts"),
		DefaultRequestsPerMinute:  getEnvInt("DEFAULT_REQUESTS_PER_MINUTE", 30),
		DefaultConcurrentSessions: getEnvInt("DEFAULT_CONCURRENT_SESSIONS", 3),
		DefaultTimeoutSeconds:     getEnvInt("DEFAULT_TIMEOUT_SECONDS", 120),
	}

	// Validate required fields
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
