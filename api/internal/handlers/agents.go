package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/compfly-ai/crosswind/api/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AgentHandlers handles agent-related HTTP requests
type AgentHandlers struct {
	services *services.Services
	logger   *zap.Logger
}

// NewAgentHandlers creates a new agent handlers instance
func NewAgentHandlers(svc *services.Services, logger *zap.Logger) *AgentHandlers {
	return &AgentHandlers{
		services: svc,
		logger:   logger,
	}
}

// Create handles POST /v1/agents
func (h *AgentHandlers) Create(c *gin.Context) {
	var req models.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
		return
	}

	agent, err := h.services.Agent.Create(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case services.ErrAgentAlreadyExists:
			respondWithError(c, http.StatusConflict, "AGENT_ALREADY_EXISTS", "Agent with this ID already exists", nil)
		case services.ErrInvalidProtocol:
			respondWithError(c, http.StatusBadRequest, "INVALID_PROTOCOL", "Unsupported agent protocol", nil)
		case services.ErrMissingBaseURL:
			respondWithError(c, http.StatusBadRequest, "MISSING_BASE_URL", "baseUrl is required for this protocol", nil)
		case services.ErrMissingEndpoint:
			respondWithError(c, http.StatusBadRequest, "MISSING_ENDPOINT", "endpoint is required for this protocol", nil)
		case services.ErrMissingAgentIdentifier:
			respondWithError(c, http.StatusBadRequest, "MISSING_AGENT_IDENTIFIER", "promptId, assistantId, or workflowId is required for this protocol", nil)
		case services.ErrMissingAgentID:
			respondWithError(c, http.StatusBadRequest, "MISSING_AGENT_ID", "agentId is required for Bedrock protocol", nil)
		case services.ErrMissingProjectID:
			respondWithError(c, http.StatusBadRequest, "MISSING_PROJECT_ID", "projectId is required for Vertex protocol", nil)
		case services.ErrMissingReasoningEngineID:
			respondWithError(c, http.StatusBadRequest, "MISSING_REASONING_ENGINE_ID", "reasoningEngineId is required for Vertex protocol", nil)
		case services.ErrMissingAgentCardURL:
			respondWithError(c, http.StatusBadRequest, "MISSING_AGENT_CARD_URL", "agentCardUrl is required for A2A protocol", nil)
		case services.ErrMissingMCPTransport:
			respondWithError(c, http.StatusBadRequest, "MISSING_MCP_TRANSPORT", "mcpTransport is required for MCP protocol", nil)
		default:
			h.logger.Error("failed to create agent", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create agent", nil)
		}
		return
	}

	c.JSON(http.StatusCreated, agent)
}

// List handles GET /v1/agents
func (h *AgentHandlers) List(c *gin.Context) {
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	response, err := h.services.Agent.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		h.logger.Error("failed to list agents", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list agents", nil)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Get handles GET /v1/agents/:agentId
func (h *AgentHandlers) Get(c *gin.Context) {
	agentID := c.Param("agentId")

	agent, err := h.services.Agent.Get(c.Request.Context(), agentID)
	if err != nil {
		if err == services.ErrAgentNotFound {
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
			return
		}
		h.logger.Error("failed to get agent", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get agent", nil)
		return
	}

	c.JSON(http.StatusOK, agent)
}

// Update handles PATCH /v1/agents/:agentId
func (h *AgentHandlers) Update(c *gin.Context) {
	agentID := c.Param("agentId")

	var req models.UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
		return
	}

	agent, err := h.services.Agent.Update(c.Request.Context(), agentID, &req)
	if err != nil {
		if err == services.ErrAgentNotFound {
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
			return
		}
		h.logger.Error("failed to update agent", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update agent", nil)
		return
	}

	c.JSON(http.StatusOK, agent)
}

// Delete handles DELETE /v1/agents/:agentId
func (h *AgentHandlers) Delete(c *gin.Context) {
	agentID := c.Param("agentId")

	err := h.services.Agent.Delete(c.Request.Context(), agentID)
	if err != nil {
		if err == services.ErrAgentNotFound {
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
			return
		}
		h.logger.Error("failed to delete agent", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete agent", nil)
		return
	}

	c.Status(http.StatusNoContent)
}

// AnalyzeAPIResponse is the sanitized response for API analysis (no internal details)
type AnalyzeAPIResponse struct {
	Schema     interface{} `json:"schema"`
	Successful bool        `json:"successful"`
	Message    string      `json:"message,omitempty"`
}

// A2AAgentCard represents the structure of an A2A agent card
// Based on A2A Protocol Specification v0.3
type A2AAgentCard struct {
	// Required fields
	ID              string              `json:"id"`                          // REQUIRED: Unique agent identifier
	Name            string              `json:"name"`                        // REQUIRED: Human-readable agent name
	ProtocolVersion string              `json:"protocolVersion"`             // REQUIRED: Latest supported A2A version (e.g., "0.3")
	Provider        A2AProvider         `json:"provider"`                    // REQUIRED: Publisher/maintainer information
	Capabilities    A2ACapabilities     `json:"capabilities"`                // REQUIRED: Feature support declaration
	Interfaces      []A2AInterface      `json:"interfaces"`                  // REQUIRED: Supported protocol bindings
	SecuritySchemes []A2ASecurityScheme `json:"securitySchemes,omitempty"`   // REQUIRED: Authentication methods
	Security        map[string][]string `json:"security,omitempty"`          // REQUIRED: Security requirements mapping

	// Optional fields
	Description               string                 `json:"description,omitempty"`               // OPTIONAL: Agent purpose/capabilities summary
	Version                   string                 `json:"version,omitempty"`                   // OPTIONAL: Agent version
	Skills                    []A2ASkill             `json:"skills,omitempty"`                    // OPTIONAL: Available agent skills/actions
	Extensions                []A2AExtension         `json:"extensions,omitempty"`                // OPTIONAL: Additional functionality
	SupportsExtendedAgentCard bool                   `json:"supportsExtendedAgentCard,omitempty"` // OPTIONAL: Extended card availability
	Metadata                  map[string]interface{} `json:"metadata,omitempty"`                  // OPTIONAL: Custom key-value attributes
}

// A2AProvider describes the entity publishing/maintaining the agent
type A2AProvider struct {
	ID   string `json:"id,omitempty"`  // OPTIONAL: Provider identifier
	Name string `json:"name"`          // REQUIRED: Provider display name
	URL  string `json:"url,omitempty"` // OPTIONAL: Provider website/contact URL
}

// A2ACapabilities declares which optional features the agent implements
type A2ACapabilities struct {
	Streaming         bool `json:"streaming,omitempty"`         // Real-time event delivery support
	PushNotifications bool `json:"pushNotifications,omitempty"` // Webhook-based async updates
}

// A2AInterface specifies a supported protocol binding and endpoint
type A2AInterface struct {
	Type string `json:"type"` // REQUIRED: Protocol identifier ("json-rpc", "http", "grpc")
	URL  string `json:"url"`  // REQUIRED: Service endpoint URI
}

// A2ASecurityScheme defines an authentication mechanism
type A2ASecurityScheme struct {
	Type        string `json:"type"`                  // REQUIRED: "apiKey", "http", "oauth2", "openIdConnect", "mutualTLS"
	Description string `json:"description,omitempty"` // OPTIONAL: Authentication approach overview
	// APIKey specific
	Name string `json:"name,omitempty"` // Header/query param name for apiKey
	In   string `json:"in,omitempty"`   // Location: "header", "query", "cookie"
	// HTTP specific
	Scheme string `json:"scheme,omitempty"` // e.g., "bearer", "basic"
}

// A2ASkill represents a specific capability or task the agent can perform
type A2ASkill struct {
	ID           string                 `json:"id"`                     // REQUIRED: Skill identifier
	Name         string                 `json:"name"`                   // REQUIRED: Human-readable skill name
	Description  string                 `json:"description,omitempty"` // OPTIONAL: Skill purpose/usage details
	InputSchema  map[string]interface{} `json:"inputSchema,omitempty"`  // OPTIONAL: Expected message content structure
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"` // OPTIONAL: Artifact/response structure
}

// A2AExtension represents additional functionality beyond core specification
type A2AExtension struct {
	URI         string `json:"uri"`                   // REQUIRED: Extension identifier URI
	Description string `json:"description,omitempty"` // OPTIONAL: Extension purpose
}

// A2AAgentPreviewResponse contains the agent card and pre-populated registration request
type A2AAgentPreviewResponse struct {
	// Raw agent card data (for display purposes)
	AgentCard *A2AAgentCard `json:"agentCard"`

	// Pre-populated CreateAgentRequest - frontend can use directly
	// Fields marked with comments need user input
	Registration *models.CreateAgentRequest `json:"registration"`
}

// AnalyzeAPI handles POST /v1/agents/:agentId/analyze
// Uses GPT to probe and analyze the agent's API structure
func (h *AgentHandlers) AnalyzeAPI(c *gin.Context) {
	agentID := c.Param("agentId")

	// Get the agent
	agent, err := h.services.Agent.Get(c.Request.Context(), agentID)
	if err != nil {
		if err == services.ErrAgentNotFound {
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
			return
		}
		h.logger.Error("failed to get agent", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get agent", nil)
		return
	}

	// Run the API analysis
	result, err := h.services.APIAnalyzer.AnalyzeAgent(c.Request.Context(), agent)
	if err != nil {
		h.logger.Error("failed to analyze agent API", zap.Error(err), zap.String("agentId", agentID))
		respondWithError(c, http.StatusInternalServerError, "ANALYSIS_FAILED", "Failed to analyze agent API", nil)
		return
	}

	// If successful, update the agent with the inferred schema
	if result.Successful && result.Schema != nil {
		if err := h.services.Agent.UpdateInferredSchema(c.Request.Context(), agentID, result.Schema); err != nil {
			h.logger.Warn("failed to save inferred schema", zap.Error(err))
			// Don't fail the request, just log it
		}
	}

	// Return sanitized response (no probe logs or internal errors)
	response := AnalyzeAPIResponse{
		Schema:     result.Schema,
		Successful: result.Successful,
	}
	if !result.Successful {
		response.Message = "Unable to infer API schema. Check agent endpoint configuration and credentials."
	}

	c.JSON(http.StatusOK, response)
}

// RetrieveAgentSpec handles GET /v1/agent-spec/:protocol/retrieve
// Fetches agent metadata from external endpoint and returns pre-populated registration fields
func (h *AgentHandlers) RetrieveAgentSpec(c *gin.Context) {
	protocol := c.Param("protocol")

	switch protocol {
	case models.ProtocolA2A:
		h.retrieveA2AAgentSpec(c)
	default:
		respondWithError(c, http.StatusBadRequest, "UNSUPPORTED_PROTOCOL", "Retrieve not supported for protocol: "+protocol, nil)
	}
}

// retrieveA2AAgentSpec fetches A2A agent card and returns pre-populated registration
func (h *AgentHandlers) retrieveA2AAgentSpec(c *gin.Context) {
	agentCardURL := c.Query("url")
	if agentCardURL == "" {
		respondWithError(c, http.StatusBadRequest, "MISSING_URL", "url query parameter is required for A2A protocol", nil)
		return
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Fetch the agent card
	resp, err := client.Get(agentCardURL)
	if err != nil {
		h.logger.Warn("failed to fetch agent card", zap.Error(err), zap.String("url", agentCardURL))
		respondWithError(c, http.StatusBadGateway, "FETCH_FAILED", "Failed to fetch agent card from URL", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Warn("agent card returned non-200 status", zap.Int("status", resp.StatusCode), zap.String("url", agentCardURL))
		respondWithError(c, http.StatusBadGateway, "INVALID_RESPONSE", "Agent card URL returned error status", resp.StatusCode)
		return
	}

	// Read and parse the agent card
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		respondWithError(c, http.StatusBadGateway, "READ_FAILED", "Failed to read agent card response", nil)
		return
	}

	var agentCard A2AAgentCard
	if err := json.Unmarshal(body, &agentCard); err != nil {
		h.logger.Warn("failed to parse agent card JSON", zap.Error(err), zap.String("url", agentCardURL))
		respondWithError(c, http.StatusBadGateway, "PARSE_FAILED", "Failed to parse agent card JSON", err.Error())
		return
	}

	// Extract tools from skills
	var tools []string
	for _, skill := range agentCard.Skills {
		if skill.ID != "" {
			tools = append(tools, skill.ID)
		} else if skill.Name != "" {
			tools = append(tools, skill.Name)
		}
	}

	// Build description (include provider if description is empty)
	description := agentCard.Description
	if description == "" && agentCard.Provider.Name != "" {
		description = "Agent provided by " + agentCard.Provider.Name
	}

	// Build auth config from securitySchemes
	authConfig := models.AuthConfigInput{
		Type: models.AuthTypeNone, // Default, user will update
	}
	if len(agentCard.SecuritySchemes) > 0 {
		scheme := agentCard.SecuritySchemes[0]
		switch scheme.Type {
		case "apiKey":
			authConfig.Type = models.AuthTypeAPIKey
			if scheme.Name != "" {
				authConfig.HeaderName = scheme.Name
			}
		case "http":
			if scheme.Scheme == "bearer" {
				authConfig.Type = models.AuthTypeBearer
			} else if scheme.Scheme == "basic" {
				authConfig.Type = models.AuthTypeBasic
			}
		case "oauth2":
			authConfig.Type = models.AuthTypeBearer
		}
		// Note: credentials left empty - user must provide
	}

	// Build CreateAgentRequest
	registration := &models.CreateAgentRequest{
		AgentID:     agentCard.ID,
		Name:        agentCard.Name,
		Description: description,
		Goal:        "", // User must provide
		Industry:    "", // User must provide
		EndpointConfig: models.EndpointConfig{
			Protocol:     models.ProtocolA2A,
			AgentCardURL: agentCardURL,
		},
		AuthConfig: authConfig,
	}

	// Add capabilities if we have tools
	if len(tools) > 0 {
		registration.DeclaredCapabilities = &models.AgentCapabilities{
			Tools:    tools,
			HasTools: true,
		}
	}

	// Return agent card and pre-populated registration
	c.JSON(http.StatusOK, A2AAgentPreviewResponse{
		AgentCard:    &agentCard,
		Registration: registration,
	})
}
