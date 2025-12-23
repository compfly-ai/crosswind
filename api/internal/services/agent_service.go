package services

import (
	"context"
	"fmt"
	"time"

	"github.com/agent-eval/agent-eval/internal/config"
	"github.com/agent-eval/agent-eval/internal/models"
	"github.com/agent-eval/agent-eval/pkg/crypto"
	"github.com/agent-eval/agent-eval/pkg/repository"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// AgentService handles agent business logic
type AgentService struct {
	agentRepo   repository.AgentRepository
	evalRunRepo repository.EvalRunRepository
	config      *config.Config
	encryptor   *crypto.Encryptor
	apiAnalyzer *APIAnalyzer
	logger      *zap.Logger
}

// NewAgentService creates a new agent service.
// Returns error if critical dependencies (encryption, logging) cannot be initialized.
func NewAgentService(
	agentRepo repository.AgentRepository,
	evalRunRepo repository.EvalRunRepository,
	cfg *config.Config,
	apiAnalyzer *APIAnalyzer,
	logger *zap.Logger,
) (*AgentService, error) {
	// Validate encryption key is provided
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("encryption key is required for agent service")
	}

	enc, err := crypto.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryptor: %w", err)
	}

	// Use provided logger or create new one
	if logger == nil {
		var logErr error
		logger, logErr = zap.NewProduction()
		if logErr != nil {
			return nil, fmt.Errorf("failed to initialize logger: %w", logErr)
		}
	}

	return &AgentService{
		agentRepo:   agentRepo,
		evalRunRepo: evalRunRepo,
		config:      cfg,
		encryptor:   enc,
		apiAnalyzer: apiAnalyzer,
		logger:      logger,
	}, nil
}

// Create creates a new agent
func (s *AgentService) Create(ctx context.Context, req *models.CreateAgentRequest) (*models.Agent, error) {
	// Validate protocol
	if !isValidProtocol(req.EndpointConfig.Protocol) {
		return nil, ErrInvalidProtocol
	}

	// Validate protocol-specific required fields
	if err := validateProtocolRequiredFields(req.EndpointConfig); err != nil {
		return nil, err
	}

	// Encrypt sensitive fields
	encryptedCreds := ""
	if req.AuthConfig.Credentials != "" && s.encryptor != nil {
		var err error
		encryptedCreds, err = s.encryptor.Encrypt(req.AuthConfig.Credentials)
		if err != nil {
			return nil, err
		}
	}

	encryptedSystemPrompt := ""
	if req.SystemPrompt != "" && s.encryptor != nil {
		var err error
		encryptedSystemPrompt, err = s.encryptor.Encrypt(req.SystemPrompt)
		if err != nil {
			return nil, err
		}
	}

	// Set defaults
	sessionStrategy := req.SessionStrategy
	if sessionStrategy == "" {
		sessionStrategy = models.SessionStrategyAutoDetect
	}

	agent := &models.Agent{
		AgentID:        req.AgentID,
		SnapshotID:     uuid.New().String(),
		Name:           req.Name,
		Description:    req.Description,
		Goal:           req.Goal,
		Industry:       req.Industry,
		SystemPrompt:   encryptedSystemPrompt,
		EndpointConfig: req.EndpointConfig,
		AuthConfig: models.AuthConfig{
			Type:          req.AuthConfig.Type,
			Credentials:   encryptedCreds,
			HeaderName:    req.AuthConfig.HeaderName,
			HeaderPrefix:  req.AuthConfig.HeaderPrefix,
			AWSRegion:     req.AuthConfig.AWSRegion,
			AzureTenantID: req.AuthConfig.AzureTenantID,
		},
		RateLimits:           req.RateLimits,
		SessionStrategy:      sessionStrategy,
		DeclaredCapabilities: req.DeclaredCapabilities,
		Status:               models.AgentStatusActive,
	}

	if err := s.agentRepo.Create(ctx, agent); err != nil {
		return nil, err
	}

	// Trigger background API analysis for custom HTTP endpoints only
	// Platform protocols (openai, langgraph, bedrock, vertex) don't need analysis
	if s.apiAnalyzer != nil && req.EndpointConfig.Protocol == models.ProtocolCustom {
		go s.analyzeAgentInBackground(ctx, agent)
	}

	// Clear sensitive data before returning
	agent.AuthConfig.Credentials = ""
	agent.SystemPrompt = ""

	return agent, nil
}

// analyzeAgentInBackground runs API analysis asynchronously after agent creation.
// It preserves values from the parent context (like orgId for multi-tenancy) but
// uses a fresh timeout since the HTTP request context may be cancelled.
func (s *AgentService) analyzeAgentInBackground(parentCtx context.Context, agent *models.Agent) {
	// Create a new context that inherits values from parent (like orgId) but not cancellation.
	// This allows background work to continue after the HTTP request completes.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parentCtx), 2*time.Minute)
	defer cancel()

	s.logger.Info("starting background API analysis",
		zap.String("agentId", agent.AgentID))

	// Get agent with decrypted credentials for probing
	agentWithCreds, err := s.GetWithCredentials(ctx, agent.AgentID)
	if err != nil {
		s.logger.Error("failed to get agent for analysis",
			zap.String("agentId", agent.AgentID),
			zap.Error(err))
		return
	}

	result, err := s.apiAnalyzer.AnalyzeAgent(ctx, agentWithCreds)
	if err != nil {
		s.logger.Error("background API analysis failed",
			zap.String("agentId", agent.AgentID),
			zap.Error(err))
		return
	}

	if result.Successful && result.Schema != nil {
		if err := s.UpdateInferredSchema(ctx, agent.AgentID, result.Schema); err != nil {
			s.logger.Error("failed to save inferred schema",
				zap.String("agentId", agent.AgentID),
				zap.Error(err))
			return
		}
		s.logger.Info("background API analysis completed",
			zap.String("agentId", agent.AgentID),
			zap.String("method", result.Schema.InferenceMethod),
			zap.Float64("confidence", result.Schema.Confidence))
	} else {
		s.logger.Warn("background API analysis unsuccessful",
			zap.String("agentId", agent.AgentID),
			zap.String("error", result.Error))
	}
}

// Get retrieves an agent by agentID and snapshotID
func (s *AgentService) Get(ctx context.Context, agentID, snapshotID string) (*models.Agent, error) {
	agent, err := s.agentRepo.FindBySnapshot(ctx, agentID, snapshotID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}

	// Clear sensitive data
	agent.AuthConfig.Credentials = ""
	agent.SystemPrompt = ""

	return agent, nil
}

// List lists all agents
func (s *AgentService) List(ctx context.Context, status string, limit, offset int) (*models.AgentListResponse, error) {
	agents, total, err := s.agentRepo.List(ctx, status, limit, offset)
	if err != nil {
		return nil, err
	}

	// Collect agent IDs for batch query
	agentIDs := make([]string, len(agents))
	for i, agent := range agents {
		agentIDs[i] = agent.AgentID
	}

	// Batch fetch latest runs for all agents in a single query (N+1 fix)
	latestRuns, err := s.evalRunRepo.GetLatestRunsByAgentIDs(ctx, agentIDs)
	if err != nil {
		s.logger.Warn("failed to fetch latest runs for agents", zap.Error(err))
		// Continue without latest run info rather than failing
		latestRuns = make(map[string]*models.EvalRun)
	}

	summaries := make([]models.AgentSummary, len(agents))
	for i, agent := range agents {
		var lastEvalRun *time.Time
		if run, ok := latestRuns[agent.AgentID]; ok {
			lastEvalRun = &run.CreatedAt
		}

		summaries[i] = models.AgentSummary{
			AgentID:     agent.AgentID,
			SnapshotID:  agent.SnapshotID,
			Name:        agent.Name,
			Industry:    agent.Industry,
			Status:      agent.Status,
			LastEvalRun: lastEvalRun,
			CreatedAt:   agent.CreatedAt,
		}
	}

	return &models.AgentListResponse{
		Agents: summaries,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// Update updates an agent
func (s *AgentService) Update(ctx context.Context, agentID string, req *models.UpdateAgentRequest) (*models.Agent, error) {
	// Check if agent exists
	existingAgent, err := s.agentRepo.FindByID(ctx, agentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	// Track if endpoint config changed (triggers re-analysis)
	endpointConfigChanged := false

	// Build update document
	update := bson.M{}

	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.Goal != nil {
		update["goal"] = *req.Goal
	}
	if req.Industry != nil {
		update["industry"] = *req.Industry
	}
	if req.SystemPrompt != nil && s.encryptor != nil {
		encrypted, err := s.encryptor.Encrypt(*req.SystemPrompt)
		if err != nil {
			return nil, err
		}
		update["systemPrompt"] = encrypted
	}
	if req.EndpointConfig != nil {
		update["endpointConfig"] = *req.EndpointConfig
		// Check if endpoint config actually changed
		if req.EndpointConfig.Endpoint != existingAgent.EndpointConfig.Endpoint ||
			req.EndpointConfig.BaseURL != existingAgent.EndpointConfig.BaseURL ||
			req.EndpointConfig.SessionEndpoint != existingAgent.EndpointConfig.SessionEndpoint ||
			req.EndpointConfig.SpecURL != existingAgent.EndpointConfig.SpecURL ||
			req.EndpointConfig.Protocol != existingAgent.EndpointConfig.Protocol {
			endpointConfigChanged = true
			// Clear existing inferred schema since endpoint changed
			update["inferredSchema"] = nil
		}
	}
	if req.AuthConfig != nil {
		encryptedCreds := ""
		if req.AuthConfig.Credentials != "" && s.encryptor != nil {
			var encErr error
			encryptedCreds, encErr = s.encryptor.Encrypt(req.AuthConfig.Credentials)
			if encErr != nil {
				return nil, encErr
			}
		}
		// Convert AuthConfigInput to AuthConfig for storage
		update["authConfig"] = models.AuthConfig{
			Type:          req.AuthConfig.Type,
			Credentials:   encryptedCreds,
			HeaderName:    req.AuthConfig.HeaderName,
			HeaderPrefix:  req.AuthConfig.HeaderPrefix,
			AWSRegion:     req.AuthConfig.AWSRegion,
			AzureTenantID: req.AuthConfig.AzureTenantID,
		}
		// Auth config change also triggers re-analysis
		endpointConfigChanged = true
		update["inferredSchema"] = nil
	}
	if req.RateLimits != nil {
		update["rateLimits"] = *req.RateLimits
	}
	if req.SessionStrategy != nil {
		update["sessionStrategy"] = *req.SessionStrategy
	}
	if req.DeclaredCapabilities != nil {
		update["declaredCapabilities"] = *req.DeclaredCapabilities
	}
	if req.Status != nil {
		update["status"] = *req.Status
	}

	if err := s.agentRepo.Update(ctx, agentID, update); err != nil {
		return nil, err
	}

	// Trigger background API analysis if endpoint config changed (only for custom HTTP)
	if endpointConfigChanged && s.apiAnalyzer != nil {
		// Get the updated agent for analysis
		updatedAgent, err := s.agentRepo.FindByID(ctx, agentID)
		if err == nil && updatedAgent.EndpointConfig.Protocol == models.ProtocolCustom {
			s.logger.Info("endpoint config changed, triggering API re-analysis",
				zap.String("agentId", agentID))
			go s.analyzeAgentInBackground(ctx, updatedAgent)
		}
	}

	return s.Get(ctx, agentID, existingAgent.SnapshotID)
}

// Delete soft-deletes an agent
func (s *AgentService) Delete(ctx context.Context, agentID string) error {
	// Check if agent exists
	_, err := s.agentRepo.FindByID(ctx, agentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return ErrAgentNotFound
		}
		return err
	}

	return s.agentRepo.Delete(ctx, agentID)
}

// GetWithCredentials retrieves an agent with decrypted credentials (for workers)
func (s *AgentService) GetWithCredentials(ctx context.Context, agentID string) (*models.Agent, error) {
	agent, err := s.agentRepo.FindByID(ctx, agentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	// Decrypt credentials
	if agent.AuthConfig.Credentials != "" && s.encryptor != nil {
		decrypted, err := s.encryptor.Decrypt(agent.AuthConfig.Credentials)
		if err != nil {
			return nil, err
		}
		agent.AuthConfig.Credentials = decrypted
	}

	// Decrypt system prompt
	if agent.SystemPrompt != "" && s.encryptor != nil {
		decrypted, err := s.encryptor.Decrypt(agent.SystemPrompt)
		if err != nil {
			return nil, err
		}
		agent.SystemPrompt = decrypted
	}

	return agent, nil
}

// UpdateInferredSchema updates the agent's inferred API schema
func (s *AgentService) UpdateInferredSchema(ctx context.Context, agentID string, schema *models.InferredAPISchema) error {
	return s.agentRepo.Update(ctx, agentID, bson.M{
		"inferredSchema": schema,
	})
}

func isValidProtocol(protocol string) bool {
	switch protocol {
	// Platform protocols (use native SDKs)
	case models.ProtocolOpenAI, models.ProtocolAzureOpenAI,
		models.ProtocolLangGraph, models.ProtocolBedrock, models.ProtocolVertex:
		return true
	// Generic protocols (custom HTTP adapters)
	case models.ProtocolCustom, models.ProtocolCustomWS:
		return true
	// Future protocols (V2)
	case models.ProtocolA2A, models.ProtocolMCP:
		return true
	default:
		return false
	}
}

// validateProtocolRequiredFields validates that required fields are present for each protocol
func validateProtocolRequiredFields(config models.EndpointConfig) error {
	switch config.Protocol {
	case models.ProtocolOpenAI:
		// OpenAI requires either promptId (Responses API), assistantId (Assistants API), or workflowId (Agent Builder)
		if config.PromptID == "" && config.AssistantID == "" && config.WorkflowID == "" {
			return ErrMissingAgentIdentifier
		}
		return nil

	case models.ProtocolAzureOpenAI:
		// Azure OpenAI requires baseUrl and either promptId, assistantId, or workflowId
		if config.BaseURL == "" {
			return ErrMissingBaseURL
		}
		if config.PromptID == "" && config.AssistantID == "" && config.WorkflowID == "" {
			return ErrMissingAgentIdentifier
		}
		return nil

	case models.ProtocolLangGraph:
		// LangGraph requires baseUrl (deployment URL)
		if config.BaseURL == "" {
			return ErrMissingBaseURL
		}
		return nil

	case models.ProtocolBedrock:
		// Bedrock requires agentId
		if config.AgentID == "" {
			return ErrMissingAgentID
		}
		return nil

	case models.ProtocolVertex:
		// Vertex requires projectId and reasoningEngineId
		if config.ProjectID == "" {
			return ErrMissingProjectID
		}
		if config.ReasoningEngineID == "" {
			return ErrMissingReasoningEngineID
		}
		return nil

	case models.ProtocolCustom:
		// Custom HTTP requires endpoint (full conversation URL)
		if config.Endpoint == "" {
			return ErrMissingEndpoint
		}
		return nil

	case models.ProtocolCustomWS:
		// Custom WebSocket requires endpoint
		if config.Endpoint == "" {
			return ErrMissingEndpoint
		}
		return nil

	case models.ProtocolA2A:
		// A2A requires agentCardUrl
		if config.AgentCardURL == "" {
			return ErrMissingAgentCardURL
		}
		return nil

	case models.ProtocolMCP:
		// MCP requires endpoint and mcpTransport
		if config.Endpoint == "" {
			return ErrMissingEndpoint
		}
		if config.MCPTransport == "" {
			return ErrMissingMCPTransport
		}
		return nil

	default:
		return nil
	}
}
