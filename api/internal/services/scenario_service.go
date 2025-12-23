package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agent-eval/agent-eval/internal/models"
	"github.com/agent-eval/agent-eval/pkg/repository"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Scenario generation constants
const (
	DefaultScenarioCount = 20 // Default number of scenarios if not specified
	MaxScenarioCount     = 30 // Maximum scenarios per generation request
	BatchSize            = 25 // Scenarios per LLM batch (for future parallel generation)
)

// Scenario service errors
var (
	ErrScenarioSetNotFound = errors.New("scenario set not found")
	ErrScenarioNotFound    = errors.New("scenario not found")
	ErrGenerationFailed    = errors.New("scenario generation failed")
)

// ScenarioService handles scenario business logic
type ScenarioService struct {
	agents     repository.AgentRepository
	scenarios  repository.ScenarioRepository
	contexts   repository.ContextRepository
	generator  *ScenarioGenerator
	contextSvc *ContextService
	logger     *zap.Logger
}

// NewScenarioService creates a new scenario service
func NewScenarioService(
	agents repository.AgentRepository,
	scenarios repository.ScenarioRepository,
	contexts repository.ContextRepository,
	openAIKey string,
	logger *zap.Logger,
) *ScenarioService {
	return &ScenarioService{
		agents:    agents,
		scenarios: scenarios,
		contexts:  contexts,
		generator: NewScenarioGenerator(openAIKey, logger),
		logger:    logger.Named("scenario-service"),
	}
}

// SetContextService sets the context service for document-based generation
func (s *ScenarioService) SetContextService(ctxSvc *ContextService) {
	s.contextSvc = ctxSvc
}

// Generate creates a new scenario set and starts generation
func (s *ScenarioService) Generate(ctx context.Context, agentID string, req *models.GenerateScenariosRequest) (*models.GenerateScenariosResponse, error) {
	// Check if agent exists
	agent, err := s.agents.FindByID(ctx, agentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	// Set default count and enforce limits
	count := req.Count
	if count == 0 {
		count = DefaultScenarioCount
	}
	if count > MaxScenarioCount {
		count = MaxScenarioCount
	}

	// Fetch context content if context IDs provided
	var contextContent string
	if len(req.ContextIDs) > 0 {
		if s.contextSvc == nil {
			s.logger.Warn("context service not configured, proceeding without context",
				zap.Strings("contextIds", req.ContextIDs),
			)
		} else {
			contextContent, err = s.fetchContextContent(ctx, agent, req.ContextIDs)
			if err != nil {
				s.logger.Warn("failed to fetch context content, proceeding without context",
					zap.Error(err),
					zap.Strings("contextIds", req.ContextIDs),
				)
				// Don't fail - proceed without context
			} else {
				s.logger.Info("fetched context content for scenario generation",
					zap.Strings("contextIds", req.ContextIDs),
					zap.Int("contentLength", len(contextContent)),
				)
			}
		}
	}

	// Generate set ID
	setID := generateScenarioSetID()

	// Resolve includeMultiTurn (default true, already set in handler)
	includeMultiTurn := true
	if req.IncludeMultiTurn != nil {
		includeMultiTurn = *req.IncludeMultiTurn
	}

	// Create scenario set record with evalType and progress tracking
	// Start with "pending" status so SSE can show the planning transition
	scenarioSet := &models.ScenarioSet{
		SetID:   setID,
		AgentID: agentID,
		Status:  models.ScenarioStatusPending,
		Config: models.ScenarioGenConfig{
			EvalType:           req.EvalType,
			Tools:              req.Tools,
			FocusAreas:         req.FocusAreas,
			CustomInstructions: req.CustomInstructions,
			ContextIDs:         req.ContextIDs,
			ContextContent:     contextContent, // Not persisted, used for generation
			Industry:           agent.Industry,
			Count:              count,
			IncludeMultiTurn:   includeMultiTurn,
		},
		Scenarios: []models.Scenario{},
		Summary:   models.ScenarioSummary{},
		Progress: &models.GenerationProgress{
			Total:       count,
			Generated:   0,
			Stage:       "pending",
			Message:     "Waiting to start...",
			LastUpdated: time.Now(),
		},
	}

	if err := s.scenarios.Create(ctx, scenarioSet); err != nil {
		return nil, err
	}

	// Start generation in background
	go s.generateAsync(context.Background(), setID, agent, &scenarioSet.Config)

	// Estimate time based on count (add time if context is large)
	estimatedSeconds := 10 + (count / 5)
	if len(contextContent) > 10000 {
		estimatedSeconds += 10 // Extra time for context processing
	}

	return &models.GenerateScenariosResponse{
		ScenarioSetID:    setID,
		Status:           models.ScenarioStatusGenerating,
		EstimatedSeconds: estimatedSeconds,
	}, nil
}

// fetchContextContent retrieves and filters context document content
// This fetches extracted text from contexts that have been processed by the worker.
// For contexts that haven't been processed yet, it falls back to metadata.
func (s *ScenarioService) fetchContextContent(ctx context.Context, agent *models.Agent, contextIDs []string) (string, error) {
	contexts, err := s.contexts.FindByIDs(ctx, contextIDs)
	if err != nil {
		return "", fmt.Errorf("failed to fetch contexts: %w", err)
	}

	if len(contexts) == 0 {
		return "", fmt.Errorf("no ready contexts found for IDs: %v", contextIDs)
	}

	var contentParts []string
	for _, ctxDoc := range contexts {
		var contextContent strings.Builder
		contextContent.WriteString(fmt.Sprintf("=== Context: %s ===\n", ctxDoc.Name))
		if ctxDoc.Description != "" {
			contextContent.WriteString(fmt.Sprintf("Description: %s\n\n", ctxDoc.Description))
		}

		hasExtractedContent := false

		for _, f := range ctxDoc.Files {
			if f.Status != models.FileStatusReady {
				continue
			}

			// Check if we have extracted text content
			if f.ExtractedText != "" {
				// File has extracted text - use it
				contextContent.WriteString(fmt.Sprintf("--- File: %s ---\n", f.Name))
				contextContent.WriteString(f.ExtractedText)
				contextContent.WriteString("\n\n")
				hasExtractedContent = true
			} else if f.ExtractedChars > 0 {
				// File was processed but text not loaded - provide metadata
				// The worker should have stored extracted text in GCS
				contextContent.WriteString(fmt.Sprintf("--- File: %s (processed, %d chars) ---\n", f.Name, f.ExtractedChars))
				contextContent.WriteString("[Text extraction completed but content not loaded. Worker should populate ExtractedText field.]\n\n")
			}
		}

		// If no extracted content available, provide metadata as fallback
		if !hasExtractedContent {
			var fileInfo []string
			for _, f := range ctxDoc.Files {
				if f.Status == models.FileStatusReady {
					info := fmt.Sprintf("- %s (%s", f.Name, f.ContentType)
					if f.PageCount > 0 {
						info += fmt.Sprintf(", %d pages", f.PageCount)
					}
					if f.RowCount > 0 {
						info += fmt.Sprintf(", %d rows", f.RowCount)
					}
					info += ")"
					fileInfo = append(fileInfo, info)
				}
			}
			if len(fileInfo) > 0 {
				contextContent.WriteString("Files available (awaiting text extraction):\n")
				contextContent.WriteString(joinStrings(fileInfo, "\n"))
				contextContent.WriteString("\n\nNote: Text extraction pending. Scenario generation will use available metadata.\n")
			}
		}

		contentParts = append(contentParts, contextContent.String())
	}

	if len(contentParts) == 0 {
		return "", nil
	}

	return joinStrings(contentParts, "\n\n"), nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// generateAsync runs scenario generation in the background with planning
func (s *ScenarioService) generateAsync(ctx context.Context, setID string, agent *models.Agent, config *models.ScenarioGenConfig) {
	logger := s.logger.With(
		zap.String("setId", setID),
		zap.String("agentId", agent.AgentID),
		zap.String("evalType", config.EvalType),
		zap.Int("count", config.Count),
	)

	logger.Info("starting scenario generation with planning")

	// Update status to generating and start planning phase
	s.scenarios.UpdateStatus(ctx, setID, models.ScenarioStatusGenerating)
	s.scenarios.UpdateStage(ctx, setID, models.StagePlanning, "Analyzing agent capabilities and creating generation plan...")

	plan, err := s.generator.PlanGeneration(ctx, agent, config)
	if err != nil {
		logger.Warn("planning failed, falling back to direct generation", zap.Error(err))
		// Fall back to direct generation without planning
		s.generateWithoutPlan(ctx, setID, agent, config, logger)
		return
	}

	logger.Info("generation plan created",
		zap.Int("requested", plan.RequestedCount),
		zap.Int("recommended", plan.RecommendedCount),
		zap.Int("batches", len(plan.Batches)),
	)

	// Store plan in progress
	if err := s.scenarios.UpdatePlan(ctx, setID, plan); err != nil {
		logger.Warn("failed to store plan", zap.Error(err))
	}

	// Step 2: Execute batches
	s.scenarios.UpdateStage(ctx, setID, models.StageGenerating, "Generating scenarios...")

	var allScenarios []models.Scenario
	totalGenerated := 0

	for i := range plan.Batches {
		batch := &plan.Batches[i]
		batch.Status = models.BatchStatusGenerating

		// Update batch status in DB
		if err := s.scenarios.UpdateBatches(ctx, setID, plan.Batches); err != nil {
			logger.Warn("failed to update batch status", zap.Error(err))
		}

		logger.Info("generating batch",
			zap.String("batchId", batch.BatchID),
			zap.String("category", batch.Category),
			zap.Int("count", batch.Count),
		)

		// Generate this batch
		scenarios, err := s.generator.GenerateBatch(ctx, agent, config, batch)
		if err != nil {
			logger.Error("batch generation failed",
				zap.String("batchId", batch.BatchID),
				zap.Error(err),
			)
			batch.Status = models.BatchStatusFailed
			// Continue with other batches
			continue
		}

		batch.Status = models.BatchStatusComplete
		batch.Generated = len(scenarios)
		allScenarios = append(allScenarios, scenarios...)
		totalGenerated += len(scenarios)

		// Update progress after each batch
		if err := s.scenarios.UpdateProgress(ctx, setID, totalGenerated, plan.RecommendedCount); err != nil {
			logger.Warn("failed to update progress", zap.Error(err))
		}
		if err := s.scenarios.UpdateBatches(ctx, setID, plan.Batches); err != nil {
			logger.Warn("failed to update batch status", zap.Error(err))
		}

		// Save scenarios incrementally so SSE stream can show progress
		if err := s.scenarios.AddScenarios(ctx, setID, scenarios); err != nil {
			logger.Warn("failed to save batch scenarios incrementally", zap.Error(err))
		}

		// Update stage message to show progress
		s.scenarios.UpdateStage(ctx, setID, models.StageGenerating,
			fmt.Sprintf("Generated %d of %d scenarios...", totalGenerated, plan.RecommendedCount))

		logger.Info("batch complete",
			zap.String("batchId", batch.BatchID),
			zap.Int("generated", len(scenarios)),
			zap.Int("totalSoFar", totalGenerated),
		)
	}

	if len(allScenarios) == 0 {
		logger.Error("all batches failed, no scenarios generated")
		s.scenarios.UpdateStage(ctx, setID, models.StageFailed, "All generation batches failed")
		s.scenarios.UpdateStatusWithError(ctx, setID, models.ScenarioStatusFailed, "all generation batches failed")
		return
	}

	logger.Info("all batches complete", zap.Int("totalScenarios", len(allScenarios)))

	// Step 3: Finalize - scenarios already saved incrementally, just update summary and status
	s.scenarios.UpdateStage(ctx, setID, models.StageComplete, "Finalizing...")

	summary := calculateSummary(allScenarios)

	// Update summary and mark as ready (scenarios already saved incrementally)
	if err := s.scenarios.UpdateSummary(ctx, setID, summary); err != nil {
		logger.Warn("failed to update summary", zap.Error(err))
	}
	if err := s.scenarios.UpdateStatus(ctx, setID, models.ScenarioStatusReady); err != nil {
		logger.Error("failed to update status to ready", zap.Error(err))
		s.scenarios.UpdateStage(ctx, setID, models.StageFailed, "Failed to finalize")
		return
	}

	logger.Info("generation complete",
		zap.Int("total", summary.Total),
		zap.Int("requested", plan.RequestedCount),
		zap.Int("recommended", plan.RecommendedCount),
	)
}

// generateWithoutPlan is the fallback when planning fails
func (s *ScenarioService) generateWithoutPlan(ctx context.Context, setID string, agent *models.Agent, config *models.ScenarioGenConfig, logger *zap.Logger) {
	logger.Info("running direct generation without plan")

	// Update stage: preparing context
	if len(config.ContextIDs) > 0 {
		s.scenarios.UpdateStage(ctx, setID, models.StagePreparingContext, "Fetching context documents...")
	} else {
		s.scenarios.UpdateStage(ctx, setID, models.StageGenerating, "Preparing generation request...")
	}

	time.Sleep(100 * time.Millisecond)
	s.scenarios.UpdateStage(ctx, setID, models.StageGenerating, "Calling LLM to generate scenarios...")

	progressCallback := func(generated int) {
		if err := s.scenarios.UpdateProgress(ctx, setID, generated, config.Count); err != nil {
			logger.Warn("failed to update progress", zap.Error(err))
		}
	}

	scenarios, err := s.generator.GenerateScenariosWithProgress(ctx, agent, config, progressCallback)
	if err != nil {
		logger.Error("scenario generation failed", zap.Error(err))
		s.scenarios.UpdateStage(ctx, setID, models.StageFailed, "Generation failed")
		errorMsg := err.Error()
		if len(errorMsg) > 500 {
			errorMsg = errorMsg[:500] + "..."
		}
		s.scenarios.UpdateStatusWithError(ctx, setID, models.ScenarioStatusFailed, errorMsg)
		return
	}

	logger.Info("generated scenarios successfully", zap.Int("count", len(scenarios)))
	s.scenarios.UpdateStage(ctx, setID, models.StageComplete, "Saving scenarios...")

	summary := calculateSummary(scenarios)

	if err := s.scenarios.UpdateScenarios(ctx, setID, scenarios, summary); err != nil {
		logger.Error("failed to save scenarios", zap.Error(err))
		s.scenarios.UpdateStage(ctx, setID, models.StageFailed, "Failed to save scenarios")
		s.scenarios.UpdateStatusWithError(ctx, setID, models.ScenarioStatusFailed, "failed to save scenarios: "+err.Error())
		return
	}

	logger.Info("scenarios saved successfully", zap.Int("total", summary.Total))
}

// Get retrieves a scenario set
func (s *ScenarioService) Get(ctx context.Context, setID string) (*models.ScenarioSet, error) {
	set, err := s.scenarios.FindBySetID(ctx, setID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrScenarioSetNotFound
		}
		return nil, err
	}
	return set, nil
}

// List lists scenario sets for an agent
func (s *ScenarioService) List(ctx context.Context, agentID string, limit, offset int) (*models.ScenarioSetListResponse, error) {
	// Check if agent exists
	_, err := s.agents.FindByID(ctx, agentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	sets, total, err := s.scenarios.ListByAgent(ctx, agentID, limit, offset)
	if err != nil {
		return nil, err
	}

	summaries := make([]models.ScenarioSetSummary, len(sets))
	for i, set := range sets {
		summaries[i] = models.ScenarioSetSummary{
			SetID:     set.SetID,
			Status:    set.Status,
			Summary:   set.Summary,
			CreatedAt: set.CreatedAt,
		}
	}

	return &models.ScenarioSetListResponse{
		ScenarioSets: summaries,
		Total:        total,
	}, nil
}

// Update updates a scenario set (enable/disable scenarios, add more, edit existing)
func (s *ScenarioService) Update(ctx context.Context, setID string, req *models.UpdateScenariosRequest) (*models.ScenarioSet, error) {
	// Get existing set
	set, err := s.scenarios.FindBySetID(ctx, setID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrScenarioSetNotFound
		}
		return nil, err
	}

	// Track the next available ID for new scenarios
	nextID := len(set.Scenarios) + 1

	// Enable scenarios
	for _, id := range req.Enable {
		if err := s.scenarios.UpdateScenarioEnabled(ctx, setID, id, true); err != nil {
			return nil, err
		}
	}

	// Disable scenarios
	for _, id := range req.Disable {
		if err := s.scenarios.UpdateScenarioEnabled(ctx, setID, id, false); err != nil {
			return nil, err
		}
	}

	// Edit existing scenarios
	for _, edit := range req.EditScenarios {
		if err := s.updateScenario(ctx, setID, &edit); err != nil {
			if err == mongodriver.ErrNoDocuments {
				return nil, fmt.Errorf("scenario not found: %s", edit.ID)
			}
			return nil, fmt.Errorf("failed to update scenario %s: %w", edit.ID, err)
		}
	}

	// Remove scenarios
	for _, id := range req.RemoveScenarios {
		if err := s.scenarios.RemoveScenario(ctx, setID, id); err != nil {
			return nil, err
		}
	}

	// Add raw scenarios directly (without LLM)
	if len(req.AddRawScenarios) > 0 {
		newScenarios := make([]models.Scenario, len(req.AddRawScenarios))
		for i, input := range req.AddRawScenarios {
			newScenarios[i] = models.Scenario{
				ID:                fmt.Sprintf("scn_%d", nextID+i),
				Category:          input.Category,
				Subcategory:       input.Subcategory,
				Tool:              input.Tool,
				Severity:          input.Severity,
				Prompt:            input.Prompt,
				ScenarioType:      input.ScenarioType,
				ExpectedBehavior:  input.ExpectedBehavior,
				Tags:              input.Tags,
				MultiTurn:         input.MultiTurn,
				Turns:             input.Turns,
				Enabled:           true, // New scenarios are enabled by default
				Rationale:         input.Rationale,
				GroundTruth:       input.GroundTruth,
				FailureIndicators: input.FailureIndicators,
			}
		}
		nextID += len(req.AddRawScenarios)

		if err := s.scenarios.AddScenarios(ctx, setID, newScenarios); err != nil {
			return nil, fmt.Errorf("failed to add raw scenarios: %w", err)
		}
	}

	// Add new scenarios from natural language prompts (LLM converts)
	if len(req.AddScenarios) > 0 {
		agent, err := s.agents.FindByID(ctx, set.AgentID)
		if err != nil {
			return nil, err
		}

		newScenarios, err := s.generator.GenerateAdditionalScenarios(ctx, agent, req.AddScenarios, set.Scenarios)
		if err != nil {
			return nil, fmt.Errorf("failed to generate additional scenarios: %w", err)
		}

		// Assign IDs to new scenarios
		for i := range newScenarios {
			newScenarios[i].ID = fmt.Sprintf("scn_%d", nextID+i)
		}

		if err := s.scenarios.AddScenarios(ctx, setID, newScenarios); err != nil {
			return nil, err
		}
	}

	// Get updated set and recalculate summary
	updatedSet, err := s.scenarios.FindBySetID(ctx, setID)
	if err != nil {
		return nil, err
	}

	// Recalculate and update summary
	summary := calculateSummary(updatedSet.Scenarios)
	if err := s.scenarios.UpdateSummary(ctx, setID, summary); err != nil {
		s.logger.Warn("failed to update summary", zap.Error(err), zap.String("setId", setID))
	}

	// Fetch final state
	return s.scenarios.FindBySetID(ctx, setID)
}

// updateScenario applies partial updates to a specific scenario
func (s *ScenarioService) updateScenario(ctx context.Context, setID string, update *models.ScenarioUpdate) error {
	updateFields := make(map[string]interface{})

	if update.Category != nil {
		updateFields["category"] = *update.Category
	}
	if update.Subcategory != nil {
		updateFields["subcategory"] = *update.Subcategory
	}
	if update.Tool != nil {
		updateFields["tool"] = *update.Tool
	}
	if update.Severity != nil {
		updateFields["severity"] = *update.Severity
	}
	if update.Prompt != nil {
		updateFields["prompt"] = *update.Prompt
	}
	if update.ScenarioType != nil {
		updateFields["scenarioType"] = *update.ScenarioType
	}
	if update.ExpectedBehavior != nil {
		updateFields["expectedBehavior"] = *update.ExpectedBehavior
	}
	if update.Tags != nil {
		updateFields["tags"] = update.Tags
	}
	if update.MultiTurn != nil {
		updateFields["multiTurn"] = *update.MultiTurn
	}
	if update.Turns != nil {
		updateFields["turns"] = update.Turns
	}
	if update.Enabled != nil {
		updateFields["enabled"] = *update.Enabled
	}
	if update.Rationale != nil {
		updateFields["rationale"] = *update.Rationale
	}
	if update.GroundTruth != nil {
		updateFields["groundTruth"] = update.GroundTruth
	}
	if update.FailureIndicators != nil {
		updateFields["failureIndicators"] = update.FailureIndicators
	}

	if len(updateFields) == 0 {
		return nil // Nothing to update
	}

	return s.scenarios.UpdateScenario(ctx, setID, update.ID, updateFields)
}

// Delete deletes a scenario set
func (s *ScenarioService) Delete(ctx context.Context, setID string) error {
	// Verify ownership
	_, err := s.scenarios.FindBySetID(ctx, setID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return ErrScenarioSetNotFound
		}
		return err
	}

	return s.scenarios.Delete(ctx, setID)
}

// calculateSummary calculates summary statistics for scenarios
func calculateSummary(scenarios []models.Scenario) models.ScenarioSummary {
	summary := models.ScenarioSummary{
		Total:      len(scenarios),
		Enabled:    0,
		ByTool:     make(map[string]int),
		BySeverity: make(map[string]int),
		ByCategory: make(map[string]int),
		MultiTurn:  0,
	}

	for _, s := range scenarios {
		if s.Enabled {
			summary.Enabled++
		}
		if s.Tool != "" {
			summary.ByTool[s.Tool]++
		}
		if s.Severity != "" {
			summary.BySeverity[s.Severity]++
		}
		if s.Category != "" {
			summary.ByCategory[s.Category]++
		}
		if s.MultiTurn {
			summary.MultiTurn++
		}
	}

	return summary
}

func generateScenarioSetID() string {
	now := time.Now()
	// Generate 6 random bytes (12 hex chars) for uniqueness
	randBytes := make([]byte, 6)
	rand.Read(randBytes)
	return fmt.Sprintf("scn_set_%s_%s", now.Format("20060102"), hex.EncodeToString(randBytes))
}
