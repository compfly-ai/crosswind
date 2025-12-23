package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"
)

// Client wraps the ClickHouse connection
type Client struct {
	conn     driver.Conn
	database string
	logger   *zap.Logger
}

// Config holds ClickHouse connection configuration
type Config struct {
	Host     string // host:port format (e.g., "host.clickhouse.cloud:9440")
	Database string
	User     string
	Password string
}

// NewClient creates a new ClickHouse client using native protocol
func NewClient(cfg *Config, logger *zap.Logger) (*Client, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("ClickHouse host is required")
	}

	// Build connection options using native protocol
	opts := &clickhouse.Options{
		Addr:     []string{cfg.Host},
		Protocol: clickhouse.Native, // Native protocol for performance
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:      10 * time.Second,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		// Enable TLS for ClickHouse Cloud (native port 9440 uses TLS)
		TLS: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	logger.Info("connected to ClickHouse",
		zap.String("host", cfg.Host),
		zap.String("database", cfg.Database))

	return &Client{
		conn:     conn,
		database: cfg.Database,
		logger:   logger,
	}, nil
}

// Close closes the ClickHouse connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// Ping checks the connection health
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// EvalDetails returns the EvalDetails repository
func (c *Client) EvalDetails() *EvalDetailsRepository {
	return &EvalDetailsRepository{
		conn:     c.conn,
		database: c.database,
		logger:   c.logger,
	}
}

// EvalSessions returns the EvalSessions repository
func (c *Client) EvalSessions() *EvalSessionsRepository {
	return &EvalSessionsRepository{
		conn:     c.conn,
		database: c.database,
		logger:   c.logger,
	}
}
