package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/compfly-ai/crosswind/api/docs"
	"github.com/compfly-ai/crosswind/api/internal/config"
	"github.com/compfly-ai/crosswind/api/internal/handlers"
	"github.com/compfly-ai/crosswind/api/internal/middleware"
	"github.com/compfly-ai/crosswind/api/internal/queue"
	"github.com/compfly-ai/crosswind/api/internal/repository/clickhouse"
	"github.com/compfly-ai/crosswind/api/internal/repository/mongo"
	"github.com/compfly-ai/crosswind/api/internal/services"
	"github.com/compfly-ai/crosswind/api/pkg/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Agent Eval API - Swagger UI</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = () => {
            SwaggerUIBundle({
                url: "/openapi.yaml",
                dom_id: '#swagger-ui',
                presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
                layout: "BaseLayout"
            });
        };
    </script>
</body>
</html>`

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Debug: log docs auth config
	if cfg.DocsPassword != "" {
		logger.Info("docs auth enabled",
			zap.String("username", cfg.DocsUsername),
			zap.Int("passwordLength", len(cfg.DocsPassword)),
		)
	} else {
		logger.Info("docs auth disabled (no DOCS_PASSWORD set)")
	}

	// Initialize MongoDB connection
	mongoClient, err := mongo.NewClient(cfg.MongoURI)
	if err != nil {
		logger.Fatal("failed to connect to MongoDB", zap.Error(err))
	}
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	// Initialize Redis queue
	redisQueue, err := queue.NewRedisQueue(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redisQueue.Close()

	// Initialize ClickHouse connection (optional - for analytics)
	var chClient *clickhouse.Client
	if cfg.ClickHouseHost != "" {
		chClient, err = clickhouse.NewClient(&clickhouse.Config{
			Host:     cfg.ClickHouseHost,
			Database: cfg.ClickHouseDatabase,
			User:     cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		}, logger)
		if err != nil {
			logger.Warn("failed to connect to ClickHouse, analytics disabled", zap.Error(err))
		} else {
			defer chClient.Close()
		}
	}

	// Initialize repositories
	repos := mongo.NewRepositories(mongoClient, cfg.DatabaseName)

	// Initialize services
	svc, err := services.NewServices(repos, redisQueue, chClient, cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize services", zap.Error(err))
	}

	// Initialize file storage for context documents
	// Uses local storage by default (STORAGE_PROVIDER=local), or GCS if configured
	storageCfg := storage.LoadConfig()
	fileStorage, err := storage.NewFileStorage(storageCfg)
	if err != nil {
		logger.Warn("failed to initialize file storage, context uploads disabled", zap.Error(err))
	} else {
		gcsClient := storage.NewGCSClient(fileStorage)
		ctxSvc := services.NewContextService(repos, gcsClient, cfg.Environment, logger)
		svc.SetContextService(ctxSvc)
		logger.Info("file storage initialized", zap.String("provider", string(storageCfg.Provider)))
	}

	// Initialize handlers
	h := handlers.NewHandlers(svc, logger)

	// Set up Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.CORS())

	// Health check endpoints
	router.GET("/health", h.HealthCheck)
	router.GET("/ready", h.ReadinessCheck)

	// Documentation endpoints (protected by Basic Auth)
	docsAuth := middleware.BasicAuth(cfg.DocsUsername, cfg.DocsPassword)
	router.GET("/openapi.yaml", docsAuth, func(c *gin.Context) {
		c.Data(http.StatusOK, "application/x-yaml", docs.OpenAPISpec)
	})
	router.GET("/docs", docsAuth, func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, swaggerUIHTML)
	})

	// Auth configuration
	authCfg := &middleware.AuthConfig{
		APIKey: cfg.APIKey,
	}

	// API v1 routes
	v1 := router.Group("/v1")
	v1.Use(middleware.Auth(authCfg))
	{
		// Agents
		agents := v1.Group("/agents")
		{
			agents.POST("", h.Agents.Create)
			agents.GET("", h.Agents.List)
			agents.GET("/:agentId", h.Agents.Get)
			agents.PATCH("/:agentId", h.Agents.Update)
			agents.DELETE("/:agentId", h.Agents.Delete)
			agents.POST("/:agentId/analyze", h.Agents.AnalyzeAPI)

			// Agent evals
			agents.POST("/:agentId/evals", h.Evals.Create)
			agents.GET("/:agentId/evals", h.Evals.ListByAgent)

			// Agent scenarios (custom generated)
			agents.POST("/:agentId/scenarios/generate", h.Scenarios.Generate)
			agents.GET("/:agentId/scenarios", h.Scenarios.List)
			agents.GET("/:agentId/scenarios/:scenarioSetId", h.Scenarios.Get)
			agents.GET("/:agentId/scenarios/:scenarioSetId/stream", h.Scenarios.StreamProgress) // SSE for live updates
			agents.PATCH("/:agentId/scenarios/:scenarioSetId", h.Scenarios.Update)
			agents.DELETE("/:agentId/scenarios/:scenarioSetId", h.Scenarios.Delete)
		}

		// Evaluation runs
		evals := v1.Group("/evals")
		{
			evals.GET("/:runId", h.Evals.Get)
			evals.GET("/:runId/results", h.Evals.GetResults)
			evals.POST("/:runId/cancel", h.Evals.Cancel)
			evals.POST("/:runId/rerun", h.Evals.Rerun)
		}

		// Datasets
		datasets := v1.Group("/datasets")
		{
			datasets.GET("", h.Datasets.List)
			datasets.GET("/:datasetId", h.Datasets.Get)
		}

		// Contexts (document uploads for scenario generation)
		contexts := v1.Group("/contexts")
		{
			contexts.POST("", h.Contexts.Create)
			contexts.GET("", h.Contexts.List)
			contexts.GET("/:contextId", h.Contexts.Get)
			contexts.DELETE("/:contextId", h.Contexts.Delete)
			contexts.POST("/:contextId/files", h.Contexts.AddFiles)
			contexts.GET("/:contextId/files/:fileName", h.Contexts.GetFile)
		}
	}

	// Create HTTP server
	// Note: WriteTimeout is set to 0 (disabled) to support SSE streaming endpoints
	// SSE connections need to stay open for extended periods during scenario generation
	srv := &http.Server{
		Addr:        ":" + cfg.Port,
		Handler:     router,
		ReadTimeout: 30 * time.Second,
		// WriteTimeout disabled (0) for SSE support - individual handlers manage their own timeouts
		IdleTimeout: 120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting server", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}
