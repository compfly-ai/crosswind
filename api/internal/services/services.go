package services

import (
	"context"
	"errors"

	"github.com/compfly-ai/crosswind/api/internal/config"
	"github.com/compfly-ai/crosswind/api/internal/queue"
	"github.com/compfly-ai/crosswind/api/internal/repository/clickhouse"
	"github.com/compfly-ai/crosswind/api/internal/repository/mongo"
	"go.uber.org/zap"
)

// Common errors
var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentAlreadyExists = errors.New("agent already exists")
	ErrSnapshotNotFound   = errors.New("agent snapshot not found")
	ErrInvalidProtocol    = errors.New("invalid protocol")
	ErrEvalRunNotFound    = errors.New("evaluation run not found")
	ErrEvalAlreadyRunning = errors.New("evaluation already running")
	ErrEvalNotCancellable = errors.New("evaluation run cannot be cancelled")
	ErrInvalidEvalMode    = errors.New("invalid evaluation mode")
	ErrResultsNotReady    = errors.New("results not ready")
	ErrDatasetNotFound    = errors.New("dataset not found")
	ErrOrgNotFound        = errors.New("organization not found")
	ErrGCSNotConfigured   = errors.New("GCS storage not configured")

	// Protocol-specific validation errors
	ErrMissingBaseURL           = errors.New("baseUrl is required for this protocol")
	ErrMissingEndpoint          = errors.New("endpoint is required for this protocol")
	ErrMissingAgentIdentifier   = errors.New("either promptId or assistantId is required for this protocol")
	ErrMissingAgentID           = errors.New("agentId is required for Bedrock protocol")
	ErrMissingProjectID         = errors.New("projectId is required for Vertex protocol")
	ErrMissingReasoningEngineID = errors.New("reasoningEngineId is required for Vertex protocol")
	ErrMissingAgentCardURL      = errors.New("agentCardUrl is required for A2A protocol")
	ErrMissingMCPTransport      = errors.New("mcpTransport is required for MCP protocol")
)

// Services holds all service instances
type Services struct {
	Agent       *AgentService
	Eval        *EvalService
	Dataset     *DatasetService
	Scenario    *ScenarioService
	Context     *ContextService
	Analytics   *AnalyticsService
	APIAnalyzer *APIAnalyzer
	repos       *mongo.Repositories
	queue       *queue.RedisQueue
	clickhouse  *clickhouse.Client
	config      *config.Config
}

// NewServices creates all service instances.
// Returns error if critical services cannot be initialized.
func NewServices(repos *mongo.Repositories, q *queue.RedisQueue, ch *clickhouse.Client, cfg *config.Config, logger *zap.Logger) (*Services, error) {
	svc := &Services{
		repos:      repos,
		queue:      q,
		clickhouse: ch,
		config:     cfg,
	}

	svc.APIAnalyzer = NewAPIAnalyzer(cfg.OpenAIKey)

	// Create agent service with proper error handling
	agentSvc, err := NewAgentService(repos.Agents, repos.EvalRuns, cfg, svc.APIAnalyzer, logger)
	if err != nil {
		return nil, err
	}
	svc.Agent = agentSvc

	svc.Eval = NewEvalService(repos.Agents, repos.EvalRuns, repos.Scenarios, repos.Results, q, ch, cfg, logger)
	svc.Dataset = NewDatasetService(repos.Datasets)
	svc.Scenario = NewScenarioService(repos.Agents, repos.Scenarios, repos.Contexts, cfg.OpenAIKey, logger)
	svc.Analytics = NewAnalyticsService(ch)
	// Context service initialized separately with storage client

	return svc, nil
}

// SetContextService sets the context service (called after GCS client initialization)
func (s *Services) SetContextService(ctxSvc *ContextService) {
	s.Context = ctxSvc
	// Also wire up to scenario service for document-based generation
	if s.Scenario != nil {
		s.Scenario.SetContextService(ctxSvc)
	}
}

// HealthCheck performs a health check on all dependencies
func (s *Services) HealthCheck(ctx context.Context) error {
	// Check MongoDB by attempting a simple operation using agents collection
	_, _, err := s.repos.Agents.List(ctx, "", 1, 0)
	if err != nil && err.Error() != "mongo: no documents in result" {
		// If it's not a "not found" error, it's a real error
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			// Ignore "not found" errors as expected for health check
		}
	}

	// Check Redis
	if err := s.queue.Ping(ctx); err != nil {
		return err
	}

	// Check ClickHouse (optional)
	if s.clickhouse != nil {
		if err := s.clickhouse.Ping(ctx); err != nil {
			return err
		}
	}

	return nil
}
