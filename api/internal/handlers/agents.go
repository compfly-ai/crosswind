package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

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
	// Read body for protocol detection
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body", nil)
		return
	}

	// Check if this is an A2A agent (lenient parsing for protocol detection)
	var protocolCheck struct {
		EndpointConfig struct {
			Protocol string `json:"protocol"`
		} `json:"endpointConfig"`
	}
	_ = json.Unmarshal(body, &protocolCheck)

	var req models.CreateAgentRequest
	if protocolCheck.EndpointConfig.Protocol == models.ProtocolA2A ||
		protocolCheck.EndpointConfig.Protocol == models.ProtocolMCP {
		// For A2A/MCP, use lenient parsing (no required field validation)
		// Service will auto-populate from agent card / MCP tool discovery
		if err := json.Unmarshal(body, &req); err != nil {
			respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
			return
		}
		// Validate only agentId is required for A2A/MCP
		if req.AgentID == "" {
			respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "agentId is required", nil)
			return
		}
	} else {
		// For other protocols, use strict gin binding with required field validation
		if err := json.Unmarshal(body, &req); err != nil {
			respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
			return
		}
		// Manual validation for non-A2A protocols
		if req.AgentID == "" || req.Name == "" || req.Description == "" || req.Goal == "" || req.Industry == "" {
			respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "agentId, name, description, goal, and industry are required", nil)
			return
		}
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
		case services.ErrMissingAgentRuntimeArn:
			respondWithError(c, http.StatusBadRequest, "MISSING_AGENT_RUNTIME_ARN", "agentRuntimeArn is required for BedrockAgentCore protocol", nil)
		case services.ErrMissingProjectID:
			respondWithError(c, http.StatusBadRequest, "MISSING_PROJECT_ID", "projectId is required for Vertex protocol", nil)
		case services.ErrMissingReasoningEngineID:
			respondWithError(c, http.StatusBadRequest, "MISSING_REASONING_ENGINE_ID", "reasoningEngineId is required for Vertex protocol", nil)
		case services.ErrMissingAgentCardURL:
			respondWithError(c, http.StatusBadRequest, "MISSING_AGENT_CARD_URL", "agentCardUrl is required for A2A protocol", nil)
		case services.ErrMissingMCPTransport:
			respondWithError(c, http.StatusBadRequest, "MISSING_MCP_TRANSPORT", "mcpTransport is required for MCP protocol", nil)
		case services.ErrMissingMCPToolName:
			respondWithError(c, http.StatusBadRequest, "MISSING_MCP_TOOL_NAME", "mcpToolName is required for MCP protocol", nil)
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
