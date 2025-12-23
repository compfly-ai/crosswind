package handlers

import (
	"net/http"

	"github.com/agent-eval/agent-eval/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers holds all HTTP handlers
type Handlers struct {
	Agents    *AgentHandlers
	Evals     *EvalHandlers
	Datasets  *DatasetHandlers
	Scenarios *ScenarioHandlers
	Contexts  *ContextHandlers
	logger    *zap.Logger
	services  *services.Services
}

// NewHandlers creates a new handlers instance
func NewHandlers(svc *services.Services, logger *zap.Logger) *Handlers {
	return &Handlers{
		Agents:    NewAgentHandlers(svc, logger),
		Evals:     NewEvalHandlers(svc, logger),
		Datasets:  NewDatasetHandlers(svc, logger),
		Scenarios: NewScenarioHandlers(svc, logger),
		Contexts:  NewContextHandlers(svc, logger),
		logger:    logger,
		services:  svc,
	}
}

// HealthCheck handles health check requests
func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// ReadinessCheck handles readiness check requests
func (h *Handlers) ReadinessCheck(c *gin.Context) {
	// Check database connectivity
	if err := h.services.HealthCheck(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// respondWithError sends an error response
func respondWithError(c *gin.Context, status int, code, message string, details interface{}) {
	c.JSON(status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
