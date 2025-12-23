package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/compfly-ai/crosswind/api/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// EvalHandlers handles evaluation-related HTTP requests
type EvalHandlers struct {
	services *services.Services
	logger   *zap.Logger
}

// NewEvalHandlers creates a new eval handlers instance
func NewEvalHandlers(svc *services.Services, logger *zap.Logger) *EvalHandlers {
	return &EvalHandlers{
		services: svc,
		logger:   logger,
	}
}

// Create handles POST /v1/agents/:agentId/evals
func (h *EvalHandlers) Create(c *gin.Context) {
	agentID := c.Param("agentId")

	var req models.CreateEvalRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
		return
	}

	response, err := h.services.Eval.Create(c.Request.Context(), agentID, &req)
	if err != nil {
		switch err {
		case services.ErrAgentNotFound:
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
		case services.ErrEvalAlreadyRunning:
			respondWithError(c, http.StatusConflict, "EVAL_ALREADY_RUNNING", "Agent already has an active evaluation run", nil)
		case services.ErrInvalidEvalMode:
			respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evaluation mode", nil)
		default:
			h.logger.Error("failed to create eval run", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create evaluation run", nil)
		}
		return
	}

	c.JSON(http.StatusAccepted, response)
}

// ListByAgent handles GET /v1/agents/:agentId/evals
func (h *EvalHandlers) ListByAgent(c *gin.Context) {
	agentID := c.Param("agentId")

	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	response, err := h.services.Eval.ListByAgent(c.Request.Context(), agentID, status, limit, offset)
	if err != nil {
		if err == services.ErrAgentNotFound {
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
			return
		}
		h.logger.Error("failed to list eval runs", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list evaluation runs", nil)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Get handles GET /v1/evals/:runId
func (h *EvalHandlers) Get(c *gin.Context) {
	runID := c.Param("runId")

	run, err := h.services.Eval.Get(c.Request.Context(), runID)
	if err != nil {
		if err == services.ErrEvalRunNotFound {
			respondWithError(c, http.StatusNotFound, "EVAL_RUN_NOT_FOUND", "Evaluation run not found", gin.H{"runId": runID})
			return
		}
		h.logger.Error("failed to get eval run", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get evaluation run", nil)
		return
	}

	c.JSON(http.StatusOK, run)
}

// GetResults handles GET /v1/evals/:runId/results
func (h *EvalHandlers) GetResults(c *gin.Context) {
	runID := c.Param("runId")

	results, err := h.services.Eval.GetResults(c.Request.Context(), runID)
	if err != nil {
		if err == services.ErrEvalRunNotFound {
			respondWithError(c, http.StatusNotFound, "EVAL_RUN_NOT_FOUND", "Evaluation run not found", gin.H{"runId": runID})
			return
		}
		if err == services.ErrResultsNotReady {
			respondWithError(c, http.StatusNotFound, "RESULTS_NOT_READY", "Evaluation results are not ready yet", gin.H{"runId": runID})
			return
		}
		h.logger.Error("failed to get eval results", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get evaluation results", nil)
		return
	}

	c.JSON(http.StatusOK, results)
}

// Cancel handles POST /v1/evals/:runId/cancel
func (h *EvalHandlers) Cancel(c *gin.Context) {
	runID := c.Param("runId")

	run, err := h.services.Eval.Cancel(c.Request.Context(), runID)
	if err != nil {
		if err == services.ErrEvalRunNotFound {
			respondWithError(c, http.StatusNotFound, "EVAL_RUN_NOT_FOUND", "Evaluation run not found", gin.H{"runId": runID})
			return
		}
		if err == services.ErrEvalNotCancellable {
			respondWithError(c, http.StatusConflict, "EVAL_NOT_CANCELLABLE", "Evaluation run cannot be cancelled in its current state", nil)
			return
		}
		h.logger.Error("failed to cancel eval run", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to cancel evaluation run", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runId":  run.RunID,
		"status": run.Status,
	})
}

// Rerun handles POST /v1/evals/:runId/rerun
// Creates a new evaluation run based on a previous run's configuration
func (h *EvalHandlers) Rerun(c *gin.Context) {
	runID := c.Param("runId")

	h.logger.Info("rerun evaluation request",
		zap.String("runId", runID))

	response, err := h.services.Eval.Rerun(c.Request.Context(), runID)
	if err != nil {
		switch err {
		case services.ErrEvalRunNotFound:
			respondWithError(c, http.StatusNotFound, "EVAL_RUN_NOT_FOUND", "Evaluation run not found", gin.H{"runId": runID})
		case services.ErrAgentNotFound:
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent no longer exists", gin.H{"runId": runID})
		case services.ErrEvalAlreadyRunning:
			respondWithError(c, http.StatusConflict, "EVAL_ALREADY_RUNNING", "Agent already has an active evaluation run", nil)
		default:
			h.logger.Error("failed to rerun evaluation",
				zap.String("runId", runID),
				zap.Error(err))
			// Return user-facing errors as bad request, others as internal error
			errMsg := err.Error()
			if strings.Contains(errMsg, "deleted") || strings.Contains(errMsg, "cannot be rerun") {
				respondWithError(c, http.StatusBadRequest, "RERUN_FAILED", errMsg, nil)
			} else {
				respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to rerun evaluation", nil)
			}
		}
		return
	}

	h.logger.Info("evaluation rerun created",
		zap.String("originalRunId", runID),
		zap.String("newRunId", response.RunID))

	c.JSON(http.StatusAccepted, response)
}
