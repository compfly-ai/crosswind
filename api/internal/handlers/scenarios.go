package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/compfly-ai/crosswind/api/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ScenarioHandlers handles scenario-related HTTP requests
type ScenarioHandlers struct {
	services *services.Services
	logger   *zap.Logger
}

// NewScenarioHandlers creates a new scenario handlers instance
func NewScenarioHandlers(svc *services.Services, logger *zap.Logger) *ScenarioHandlers {
	return &ScenarioHandlers{
		services: svc,
		logger:   logger,
	}
}

// Generate handles POST /v1/agents/:agentId/scenarios/generate
func (h *ScenarioHandlers) Generate(c *gin.Context) {
	agentID := c.Param("agentId")

	var req models.GenerateScenariosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
		return
	}

	// Default evalType to red_team if not specified
	if req.EvalType == "" {
		req.EvalType = models.EvalTypeRedTeam
	}

	// Validate evalType
	if req.EvalType != models.EvalTypeRedTeam && req.EvalType != models.EvalTypeTrust {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"evalType must be 'red_team' or 'trust'",
			gin.H{"evalType": req.EvalType})
		return
	}

	// Validate tools requirement based on evalType
	// Red team requires tools (targeting specific systems), trust doesn't necessarily
	if req.EvalType == models.EvalTypeRedTeam && len(req.Tools) == 0 {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"At least one tool is required for red_team scenarios", nil)
		return
	}

	// Apply default focus areas if not provided
	if len(req.FocusAreas) == 0 {
		req.FocusAreas = models.GetDefaultFocusAreas(req.EvalType)
	}

	// Default includeMultiTurn to true for agentic evaluation
	// Multi-turn scenarios are critical for testing agent tool execution and context handling
	// Users can explicitly set to false if they only want single-turn scenarios
	if req.IncludeMultiTurn == nil {
		defaultTrue := true
		req.IncludeMultiTurn = &defaultTrue
	}

	// Validate focus areas match the evalType
	if !models.ValidateFocusAreas(req.EvalType, req.FocusAreas) {
		validAreas := models.GetDefaultFocusAreas(req.EvalType)
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"Invalid focus areas for evalType '"+req.EvalType+"'",
			gin.H{"validFocusAreas": validAreas})
		return
	}

	response, err := h.services.Scenario.Generate(c.Request.Context(), agentID, &req)
	if err != nil {
		switch err {
		case services.ErrAgentNotFound:
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
		default:
			h.logger.Error("failed to generate scenarios", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate scenarios", nil)
		}
		return
	}

	c.JSON(http.StatusAccepted, response)
}

// Get handles GET /v1/agents/:agentId/scenarios/:scenarioSetId
func (h *ScenarioHandlers) Get(c *gin.Context) {
	setID := c.Param("scenarioSetId")

	set, err := h.services.Scenario.Get(c.Request.Context(), setID)
	if err != nil {
		switch err {
		case services.ErrScenarioSetNotFound:
			respondWithError(c, http.StatusNotFound, "SCENARIO_SET_NOT_FOUND", "Scenario set not found", gin.H{"scenarioSetId": setID})
		default:
			h.logger.Error("failed to get scenario set", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get scenario set", nil)
		}
		return
	}

	c.JSON(http.StatusOK, set)
}

// List handles GET /v1/agents/:agentId/scenarios
func (h *ScenarioHandlers) List(c *gin.Context) {
	agentID := c.Param("agentId")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	response, err := h.services.Scenario.List(c.Request.Context(), agentID, limit, offset)
	if err != nil {
		switch err {
		case services.ErrAgentNotFound:
			respondWithError(c, http.StatusNotFound, "AGENT_NOT_FOUND", "Agent not found", gin.H{"agentId": agentID})
		default:
			h.logger.Error("failed to list scenario sets", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list scenario sets", nil)
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

// Update handles PATCH /v1/agents/:agentId/scenarios/:scenarioSetId
func (h *ScenarioHandlers) Update(c *gin.Context) {
	setID := c.Param("scenarioSetId")

	var req models.UpdateScenariosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
		return
	}

	set, err := h.services.Scenario.Update(c.Request.Context(), setID, &req)
	if err != nil {
		switch err {
		case services.ErrScenarioSetNotFound:
			respondWithError(c, http.StatusNotFound, "SCENARIO_SET_NOT_FOUND", "Scenario set not found", gin.H{"scenarioSetId": setID})
		default:
			h.logger.Error("failed to update scenario set", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update scenario set", nil)
		}
		return
	}

	c.JSON(http.StatusOK, set)
}

// Delete handles DELETE /v1/agents/:agentId/scenarios/:scenarioSetId
func (h *ScenarioHandlers) Delete(c *gin.Context) {
	setID := c.Param("scenarioSetId")

	err := h.services.Scenario.Delete(c.Request.Context(), setID)
	if err != nil {
		switch err {
		case services.ErrScenarioSetNotFound:
			respondWithError(c, http.StatusNotFound, "SCENARIO_SET_NOT_FOUND", "Scenario set not found", gin.H{"scenarioSetId": setID})
		default:
			h.logger.Error("failed to delete scenario set", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete scenario set", nil)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// SSEEvent represents a Server-Sent Event for scenario streaming
type SSEEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// SSEProgressData provides user-friendly progress updates
// This is the public-facing structure - keeps internal details hidden
// Designed to be lightweight for frequent progress updates
type SSEProgressData struct {
	SetID   string `json:"setId"`
	Status  string `json:"status"`
	Stage   string `json:"stage"`   // planning, generating, complete, failed
	Message string `json:"message"` // User-friendly message
	Progress *struct {
		Generated int `json:"generated"`
		Total     int `json:"total"`
	} `json:"progress,omitempty"`
	// Batches shows simplified batch status (only sent during generating stage)
	Batches       []SSEBatchStatus `json:"batches,omitempty"`
	ScenarioCount int              `json:"scenarioCount"`
	Error         string           `json:"error,omitempty"`
}

// SSEBatchStatus is a simplified batch status for progress updates
type SSEBatchStatus struct {
	Category  string `json:"category"`
	Status    string `json:"status"` // pending, generating, complete
	Count     int    `json:"count"`
	Generated int    `json:"generated"`
}

// SSEPlanData is sent once when the plan is ready (on init or first progress after planning)
type SSEPlanData struct {
	SetID            string `json:"setId"`
	Status           string `json:"status"`
	Stage            string `json:"stage"`
	Message          string `json:"message"`
	RequestedCount   int    `json:"requestedCount"`
	RecommendedCount int    `json:"recommendedCount"`
	Rationale        string `json:"rationale"`
	Categories       []struct {
		Category    string `json:"category"`
		Recommended int    `json:"recommended"`
		Priority    int    `json:"priority"`
	} `json:"categories"`
	Warnings []string `json:"warnings,omitempty"`
}

// StreamProgress handles GET /v1/agents/:agentId/scenarios/:scenarioSetId/stream
// This provides Server-Sent Events (SSE) for live progress updates during generation
func (h *ScenarioHandlers) StreamProgress(c *gin.Context) {
	setID := c.Param("scenarioSetId")

	// Use background context for DB queries (independent of request lifecycle)
	dbCtx := context.Background()

	// Verify the scenario set exists before starting the stream
	initialSet, err := h.services.Scenario.Get(dbCtx, setID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scenario set not found"})
		return
	}

	// Set SSE headers - must be set before any writes
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	// Get flusher for manual control
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.logger.Error("streaming not supported")
		return
	}

	// Track state for detecting changes
	lastScenarioCount := len(initialSet.Scenarios)
	lastStatus := initialSet.Status
	lastGenerated := 0
	lastStage := ""
	planSent := false // Track if we've sent the plan summary

	if initialSet.Progress != nil {
		lastGenerated = initialSet.Progress.Generated
		lastStage = initialSet.Progress.Stage
		// Check if plan is already available
		planSent = initialSet.Progress.Plan != nil
	}

	// Send initial event immediately
	h.writeSSEEvent(c.Writer, "init", h.buildSSEProgressData(initialSet))
	flusher.Flush()

	// If plan is already available on init, send it as a separate event
	if planSent && initialSet.Progress.Plan != nil {
		h.writeSSEEvent(c.Writer, "plan", h.buildSSEPlanData(initialSet))
		flusher.Flush()
	}

	h.logger.Info("SSE stream started",
		zap.String("setId", setID),
		zap.String("stage", lastStage),
		zap.String("status", lastStatus))

	// Check if already complete
	if initialSet.Status == models.ScenarioStatusReady {
		h.writeSSEEvent(c.Writer, "complete", h.buildSSEProgressData(initialSet))
		flusher.Flush()
		return
	} else if initialSet.Status == models.ScenarioStatusFailed {
		h.writeSSEEvent(c.Writer, "error", h.buildSSEProgressData(initialSet))
		flusher.Flush()
		return
	}

	// Create ticker for polling
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Poll for updates until complete or client disconnects
	for {
		select {
		case <-c.Request.Context().Done():
			h.logger.Debug("SSE client disconnected", zap.String("setId", setID))
			return

		case <-ticker.C:
			set, err := h.services.Scenario.Get(dbCtx, setID)
			if err != nil {
				h.logger.Error("SSE failed to fetch progress", zap.String("setId", setID), zap.Error(err))
				h.writeSSEEvent(c.Writer, "error", gin.H{"error": "Failed to fetch progress"})
				flusher.Flush()
				return
			}

			// Get current state
			currentStage := ""
			currentGenerated := 0
			hasPlan := false
			if set.Progress != nil {
				currentStage = set.Progress.Stage
				currentGenerated = set.Progress.Generated
				hasPlan = set.Progress.Plan != nil
			}
			currentCount := len(set.Scenarios)

			// Send plan once when it first becomes available
			if hasPlan && !planSent {
				h.writeSSEEvent(c.Writer, "plan", h.buildSSEPlanData(set))
				flusher.Flush()
				planSent = true
				h.logger.Info("SSE plan sent", zap.String("setId", setID))
			}

			// Detect changes
			stageChanged := currentStage != lastStage
			statusChanged := set.Status != lastStatus
			countChanged := currentCount != lastScenarioCount
			generatedChanged := currentGenerated != lastGenerated

			if stageChanged || statusChanged || countChanged || generatedChanged {
				h.writeSSEEvent(c.Writer, "progress", h.buildSSEProgressData(set))
				flusher.Flush()

				h.logger.Info("SSE progress sent",
					zap.String("setId", setID),
					zap.String("stage", currentStage),
					zap.String("status", set.Status),
					zap.Int("scenarios", currentCount),
					zap.Int("generated", currentGenerated))

				lastScenarioCount = currentCount
				lastStage = currentStage
				lastGenerated = currentGenerated
				lastStatus = set.Status
			}

			// Check for terminal status
			if set.Status == models.ScenarioStatusReady {
				h.writeSSEEvent(c.Writer, "complete", h.buildSSEProgressData(set))
				flusher.Flush()
				h.logger.Info("SSE stream complete", zap.String("setId", setID))
				return
			} else if set.Status == models.ScenarioStatusFailed {
				h.writeSSEEvent(c.Writer, "error", h.buildSSEProgressData(set))
				flusher.Flush()
				h.logger.Info("SSE stream failed", zap.String("setId", setID))
				return
			}
		}
	}
}

// writeSSEEvent writes a Server-Sent Event to the writer
func (h *ScenarioHandlers) writeSSEEvent(w io.Writer, event string, data interface{}) {
	// Format: event: <name>\ndata: <json>\n\n
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("failed to marshal SSE data", zap.Error(err))
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonBytes))
}

// buildSSEProgressData creates a lightweight SSE progress data structure
// This is designed for frequent updates - no large plan details, just essential progress info
func (h *ScenarioHandlers) buildSSEProgressData(set *models.ScenarioSet) SSEProgressData {
	data := SSEProgressData{
		SetID:         set.SetID,
		Status:        set.Status,
		ScenarioCount: len(set.Scenarios),
	}

	if set.Progress != nil {
		data.Stage = set.Progress.Stage
		data.Message = set.Progress.Message
		data.Progress = &struct {
			Generated int `json:"generated"`
			Total     int `json:"total"`
		}{
			Generated: set.Progress.Generated,
			Total:     set.Progress.Total,
		}

		// Include simplified batch status during generating stage
		if set.Progress.Plan != nil && len(set.Progress.Plan.Batches) > 0 {
			data.Batches = make([]SSEBatchStatus, len(set.Progress.Plan.Batches))
			for i, b := range set.Progress.Plan.Batches {
				data.Batches[i] = SSEBatchStatus{
					Category:  b.Category,
					Status:    b.Status,
					Count:     b.Count,
					Generated: b.Generated,
				}
			}
		}
	} else {
		switch set.Status {
		case models.ScenarioStatusPending:
			data.Stage = "pending"
			data.Message = "Waiting to start..."
		case models.ScenarioStatusGenerating:
			data.Stage = models.StageGenerating
			data.Message = "Generating scenarios..."
		case models.ScenarioStatusReady:
			data.Stage = models.StageComplete
			data.Message = fmt.Sprintf("Generated %d scenarios", len(set.Scenarios))
		case models.ScenarioStatusFailed:
			data.Stage = models.StageFailed
			data.Message = "Generation failed"
		}
	}

	if set.Error != "" {
		data.Error = h.sanitizeErrorMessage(set.Error)
	}

	return data
}

// buildSSEPlanData creates the plan summary data (sent once when plan is ready)
func (h *ScenarioHandlers) buildSSEPlanData(set *models.ScenarioSet) SSEPlanData {
	data := SSEPlanData{
		SetID:   set.SetID,
		Status:  set.Status,
		Stage:   set.Progress.Stage,
		Message: set.Progress.Message,
	}

	if set.Progress.Plan != nil {
		plan := set.Progress.Plan
		data.RequestedCount = plan.RequestedCount
		data.RecommendedCount = plan.RecommendedCount
		data.Rationale = plan.Rationale
		data.Warnings = plan.Warnings

		// Extract just category, count, and priority from breakdown
		for _, cat := range plan.CategoryBreakdown {
			data.Categories = append(data.Categories, struct {
				Category    string `json:"category"`
				Recommended int    `json:"recommended"`
				Priority    int    `json:"priority"`
			}{
				Category:    cat.Category,
				Recommended: cat.Recommended,
				Priority:    cat.Priority,
			})
		}
	}

	return data
}

// sanitizeErrorMessage converts internal error messages to user-friendly ones
func (h *ScenarioHandlers) sanitizeErrorMessage(err string) string {
	// Map known internal errors to user-friendly messages
	switch {
	case containsIgnoreCase(err, "context deadline exceeded"):
		return "Request timed out. Please try again."
	case containsIgnoreCase(err, "OpenAI API"):
		return "AI service temporarily unavailable. Please try again."
	case containsIgnoreCase(err, "rate limit"):
		return "Too many requests. Please wait a moment and try again."
	case containsIgnoreCase(err, "context not found"):
		return "One or more uploaded documents could not be found."
	case containsIgnoreCase(err, "unsupported"):
		return "One or more documents are in an unsupported format."
	case containsIgnoreCase(err, "extraction failed"):
		return "Failed to process uploaded documents."
	case containsIgnoreCase(err, "connection"):
		return "Service temporarily unavailable. Please try again."
	default:
		// For unknown errors, return a generic message
		// Don't expose internal details
		return "An error occurred during generation. Please try again."
	}
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
