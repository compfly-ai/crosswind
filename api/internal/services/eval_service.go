package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/compfly-ai/crosswind/api/internal/config"
	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/compfly-ai/crosswind/api/internal/queue"
	"github.com/compfly-ai/crosswind/api/internal/repository/clickhouse"
	"github.com/compfly-ai/crosswind/api/pkg/repository"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Eval mode configurations
var evalModeConfig = map[string]struct {
	estimatedPrompts int
}{
	models.EvalModeQuick:    {estimatedPrompts: 200},
	models.EvalModeStandard: {estimatedPrompts: 2000},
	models.EvalModeDeep:     {estimatedPrompts: 10000},
}

// EvalService handles evaluation business logic
type EvalService struct {
	agentRepo    repository.AgentRepository
	evalRunRepo  repository.EvalRunRepository
	scenarioRepo repository.ScenarioRepository
	resultsRepo  repository.ResultsRepository
	queue        *queue.RedisQueue
	clickhouse   *clickhouse.Client
	config       *config.Config
	logger       *zap.Logger
}

// NewEvalService creates a new eval service
func NewEvalService(
	agentRepo repository.AgentRepository,
	evalRunRepo repository.EvalRunRepository,
	scenarioRepo repository.ScenarioRepository,
	resultsRepo repository.ResultsRepository,
	q *queue.RedisQueue,
	ch *clickhouse.Client,
	cfg *config.Config,
	logger *zap.Logger,
) *EvalService {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &EvalService{
		agentRepo:    agentRepo,
		evalRunRepo:  evalRunRepo,
		scenarioRepo: scenarioRepo,
		resultsRepo:  resultsRepo,
		queue:        q,
		clickhouse:   ch,
		config:       cfg,
		logger:       logger,
	}
}

// Create creates a new evaluation run
func (s *EvalService) Create(ctx context.Context, agentID string, req *models.CreateEvalRunRequest) (*models.CreateEvalRunResponse, error) {
	// Validate eval mode
	modeConfig, ok := evalModeConfig[req.Mode]
	if !ok {
		return nil, ErrInvalidEvalMode
	}

	// Get agent by ID
	agent, err := s.agentRepo.FindByID(ctx, agentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	// Check if agent is active (not deleted)
	if agent.Status == models.AgentStatusDeleted {
		return nil, ErrAgentNotFound
	}

	// Check if there's already an active run for this agent
	hasActive, err := s.evalRunRepo.HasActiveRun(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if hasActive {
		return nil, ErrEvalAlreadyRunning
	}

	// Determine rate limits (agent > defaults)
	rateLimits := models.DefaultRateLimits()
	if agent.RateLimits != nil {
		if agent.RateLimits.RequestsPerMinute > 0 {
			rateLimits.RequestsPerMinute = agent.RateLimits.RequestsPerMinute
		}
		if agent.RateLimits.ConcurrentSessions > 0 {
			rateLimits.ConcurrentSessions = agent.RateLimits.ConcurrentSessions
		}
		if agent.RateLimits.MaxTimeoutSeconds > 0 {
			rateLimits.MaxTimeoutSeconds = agent.RateLimits.MaxTimeoutSeconds
		}
	}

	// Override with request config if provided
	if req.Config != nil && req.Config.RequestsPerMinute != nil {
		rateLimits.RequestsPerMinute = *req.Config.RequestsPerMinute
	}

	// Validate and load scenario sets if provided
	var scenarioSetsUsed []models.ScenarioSetUsed
	var scenarioSetIDs []string
	scenarioPromptCount := 0

	if req.Config != nil && len(req.Config.ScenarioSetIDs) > 0 {
		scenarioSetIDs = req.Config.ScenarioSetIDs
		for _, setID := range scenarioSetIDs {
			set, err := s.scenarioRepo.FindBySetID(ctx, setID)
			if err != nil {
				if err == mongodriver.ErrNoDocuments {
					return nil, fmt.Errorf("scenario set not found: %s", setID)
				}
				return nil, err
			}

			// Ensure scenario set is ready
			if set.Status != models.ScenarioStatusReady {
				return nil, fmt.Errorf("scenario set not ready: %s (status: %s)", setID, set.Status)
			}

			// Ensure evalType matches
			if set.Config.EvalType != req.EvalType {
				return nil, fmt.Errorf("scenario set evalType mismatch: %s has evalType '%s', but eval request is '%s'",
					setID, set.Config.EvalType, req.EvalType)
			}

			// Count enabled scenarios
			enabledCount := 0
			for _, scenario := range set.Scenarios {
				if scenario.Enabled {
					enabledCount++
				}
			}
			scenarioPromptCount += enabledCount

			scenarioSetsUsed = append(scenarioSetsUsed, models.ScenarioSetUsed{
				SetID:         setID,
				ScenarioCount: enabledCount,
				EvalType:      set.Config.EvalType,
				FocusAreas:    set.Config.FocusAreas,
			})
		}
	}

	// Calculate total prompts (from mode estimate + scenario sets)
	totalPrompts := modeConfig.estimatedPrompts
	if scenarioPromptCount > 0 {
		// If scenario sets provided, add their count
		// If ONLY scenario sets (no datasets), use just the scenario count
		if req.Config != nil && len(req.Config.IncludeDatasets) == 0 && len(req.Config.ScenarioSetIDs) > 0 {
			totalPrompts = scenarioPromptCount
		} else {
			totalPrompts += scenarioPromptCount
		}
	}

	// Generate run ID
	runID := generateRunID()

	// Create eval run
	evalRun := &models.EvalRun{
		RunID:   runID,
		AgentID: agentID,
		Mode:    req.Mode,
		EvalType: req.EvalType,
		Status:   models.EvalStatusPending,
		Config: models.EvalRunConfig{
			RequestsPerMinute:    rateLimits.RequestsPerMinute,
			ConcurrentSessions:   rateLimits.ConcurrentSessions,
			TimeoutSeconds:       rateLimits.MaxTimeoutSeconds,
			ResetSessionOnError:  true,
			MaxConsecutiveErrors: 5,
		},
		ScenarioSetsUsed: scenarioSetsUsed,
		Progress: models.EvalProgress{
			TotalPrompts:     totalPrompts,
			CompletedPrompts: 0,
			LastUpdated:      time.Now(),
		},
	}

	if err := s.evalRunRepo.Create(ctx, evalRun); err != nil {
		return nil, err
	}

	// Determine if built-in datasets should be included
	// Default is false - if user provides scenarioSetIds, just use those
	includeBuiltInDatasets := false
	if req.Config != nil && req.Config.IncludeBuiltInDatasets != nil {
		includeBuiltInDatasets = *req.Config.IncludeBuiltInDatasets
	}

	// Enqueue the job
	job := queue.EvalJob{
		RunID:                  runID,
		AgentID:                agentID,
		Mode:                   req.Mode,
		EvalType:               req.EvalType,
		ScenarioSetIDs:         scenarioSetIDs,
		IncludeBuiltInDatasets: includeBuiltInDatasets,
	}

	if err := s.queue.EnqueueEvalJob(ctx, job); err != nil {
		// Update status to failed if we can't enqueue
		s.evalRunRepo.UpdateStatus(ctx, runID, models.EvalStatusFailed)
		return nil, err
	}

	return &models.CreateEvalRunResponse{
		RunID:            runID,
		AgentID:          agentID,
		Mode:             req.Mode,
		EvalType:         req.EvalType,
		Status:           models.EvalStatusPending,
		EstimatedPrompts: totalPrompts,
		CreatedAt:        evalRun.CreatedAt,
	}, nil
}

// Get retrieves an evaluation run
func (s *EvalService) Get(ctx context.Context, runID string) (*models.EvalRun, error) {
	run, err := s.evalRunRepo.FindByRunID(ctx, runID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrEvalRunNotFound
		}
		return nil, err
	}
	return run, nil
}

// ListByAgent lists evaluation runs for an agent
func (s *EvalService) ListByAgent(ctx context.Context, agentID, status string, limit, offset int) (*models.EvalRunListResponse, error) {
	// Check if agent exists
	_, err := s.agentRepo.FindByID(ctx, agentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	runs, total, err := s.evalRunRepo.ListByAgent(ctx, agentID, status, limit, offset)
	if err != nil {
		return nil, err
	}

	summaries := make([]models.EvalRunSummary, len(runs))
	for i, run := range runs {
		summaries[i] = models.EvalRunSummary{
			RunID:         run.RunID,
			Mode:          run.Mode,
			EvalType:      run.EvalType,
			Status:        run.Status,
			SummaryScores: run.SummaryScores,
			StartedAt:     run.StartedAt,
			CompletedAt:   run.CompletedAt,
		}
	}

	return &models.EvalRunListResponse{
		Runs:   summaries,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// GetResults retrieves evaluation results
func (s *EvalService) GetResults(ctx context.Context, runID string) (*models.GetResultsResponse, error) {
	// Get the eval run first
	run, err := s.evalRunRepo.FindByRunID(ctx, runID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrEvalRunNotFound
		}
		return nil, err
	}

	// If still running, return partial results from run
	if run.Status == models.EvalStatusPending || run.Status == models.EvalStatusRunning {
		return &models.GetResultsResponse{
			RunID:    runID,
			Status:   run.Status,
			EvalType: run.EvalType,
		}, nil
	}

	// Get full results
	results, err := s.resultsRepo.FindByRunID(ctx, runID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrResultsNotReady
		}
		return nil, err
	}

	// Apply visibility filtering to results
	// Results from restricted datasets will have content redacted or excluded
	filteredFailures := models.FilterResultsByVisibility(results.Failures)
	filteredPasses := models.FilterResultsByVisibility(results.SamplePasses)

	return &models.GetResultsResponse{
		RunID:                runID,
		Status:               run.Status,
		EvalType:             run.EvalType,
		SummaryScores:        run.SummaryScores,
		RegulatoryCompliance: run.RegulatoryCompliance,
		ThreatAnalysis:       run.ThreatAnalysis,
		TrustAnalysis:        run.TrustAnalysis,
		Recommendations:      run.Recommendations,
		Failures:             filteredFailures,
		SamplePasses:         filteredPasses,
		CategoryBreakdown:    results.CategoryBreakdown,
		PerformanceMetrics:   &results.PerformanceMetrics,
	}, nil
}

// Cancel cancels an evaluation run
func (s *EvalService) Cancel(ctx context.Context, runID string) (*models.EvalRun, error) {
	run, err := s.evalRunRepo.FindByRunID(ctx, runID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrEvalRunNotFound
		}
		return nil, err
	}

	// Can only cancel pending or running evals
	if run.Status != models.EvalStatusPending && run.Status != models.EvalStatusRunning {
		return nil, ErrEvalNotCancellable
	}

	// Update status
	if err := s.evalRunRepo.Update(ctx, runID, bson.M{
		"status":      models.EvalStatusCancelled,
		"completedAt": time.Now(),
	}); err != nil {
		return nil, err
	}

	run.Status = models.EvalStatusCancelled
	return run, nil
}

// Rerun creates a new evaluation run based on a previous run
func (s *EvalService) Rerun(ctx context.Context, runID string) (*models.CreateEvalRunResponse, error) {
	s.logger.Info("starting evaluation rerun",
		zap.String("originalRunId", runID))

	// Get the original eval run
	originalRun, err := s.evalRunRepo.FindByRunID(ctx, runID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			s.logger.Warn("original eval run not found",
				zap.String("runId", runID))
			return nil, ErrEvalRunNotFound
		}
		s.logger.Error("failed to find original eval run",
			zap.String("runId", runID),
			zap.Error(err))
		return nil, err
	}

	// Load the agent by ID
	agent, err := s.agentRepo.FindByID(ctx, originalRun.AgentID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			s.logger.Error("agent not found",
				zap.String("agentId", originalRun.AgentID))
			return nil, ErrAgentNotFound
		}
		s.logger.Error("failed to find agent",
			zap.String("agentId", originalRun.AgentID),
			zap.Error(err))
		return nil, err
	}

	// Check if agent is active (not deleted)
	if agent.Status == models.AgentStatusDeleted {
		s.logger.Warn("cannot rerun eval - agent is deleted",
			zap.String("agentId", agent.AgentID))
		return nil, fmt.Errorf("agent has been deleted")
	}

	// Check if there's already an active run for this agent
	hasActive, err := s.evalRunRepo.HasActiveRun(ctx, agent.AgentID)
	if err != nil {
		s.logger.Error("failed to check for active runs",
			zap.String("agentId", agent.AgentID),
			zap.Error(err))
		return nil, err
	}
	if hasActive {
		s.logger.Info("cannot rerun - agent already has active evaluation",
			zap.String("agentId", agent.AgentID))
		return nil, ErrEvalAlreadyRunning
	}

	// Validate scenario sets if they were used in the original run
	var scenarioSetIDs []string
	var validScenarioSetsUsed []models.ScenarioSetUsed
	for _, setUsed := range originalRun.ScenarioSetsUsed {
		set, err := s.scenarioRepo.FindBySetID(ctx, setUsed.SetID)
		if err != nil {
			if err == mongodriver.ErrNoDocuments {
				s.logger.Warn("scenario set from original run no longer exists - skipping",
					zap.String("setId", setUsed.SetID))
				continue // Skip missing scenario sets
			}
			s.logger.Error("failed to validate scenario set",
				zap.String("setId", setUsed.SetID),
				zap.Error(err))
			return nil, err
		}
		if set.Status != models.ScenarioStatusReady {
			s.logger.Warn("scenario set not ready - skipping",
				zap.String("setId", setUsed.SetID),
				zap.String("status", set.Status))
			continue
		}
		scenarioSetIDs = append(scenarioSetIDs, setUsed.SetID)
		validScenarioSetsUsed = append(validScenarioSetsUsed, setUsed)
	}

	// Generate new run ID
	newRunID := generateRunID()

	// Create new eval run with same configuration
	evalRun := &models.EvalRun{
		RunID:            newRunID,
		AgentID:          agent.AgentID,
		Mode:             originalRun.Mode,
		EvalType:         originalRun.EvalType,
		Status:           models.EvalStatusPending,
		Config:           originalRun.Config,
		ScenarioSetsUsed: validScenarioSetsUsed,
		Progress: models.EvalProgress{
			TotalPrompts:     originalRun.Progress.TotalPrompts,
			CompletedPrompts: 0,
			LastUpdated:      time.Now(),
		},
	}

	if err := s.evalRunRepo.Create(ctx, evalRun); err != nil {
		s.logger.Error("failed to create rerun eval record",
			zap.String("newRunId", newRunID),
			zap.Error(err))
		return nil, err
	}

	// Enqueue the job
	job := queue.EvalJob{
		RunID:          newRunID,
		AgentID:        agent.AgentID,
		Mode:           originalRun.Mode,
		EvalType:       originalRun.EvalType,
		ScenarioSetIDs: scenarioSetIDs,
	}

	if err := s.queue.EnqueueEvalJob(ctx, job); err != nil {
		s.logger.Error("failed to enqueue rerun job",
			zap.String("newRunId", newRunID),
			zap.Error(err))
		// Update status to failed if we can't enqueue
		s.evalRunRepo.UpdateStatus(ctx, newRunID, models.EvalStatusFailed)
		return nil, err
	}

	s.logger.Info("evaluation rerun created successfully",
		zap.String("newRunId", newRunID),
		zap.String("originalRunId", runID),
		zap.String("agentId", agent.AgentID),
		zap.String("mode", originalRun.Mode),
		zap.String("evalType", originalRun.EvalType))

	return &models.CreateEvalRunResponse{
		RunID:            newRunID,
		AgentID:          agent.AgentID,
		Mode:             originalRun.Mode,
		EvalType:         originalRun.EvalType,
		Status:           models.EvalStatusPending,
		EstimatedPrompts: originalRun.Progress.TotalPrompts,
		CreatedAt:        evalRun.CreatedAt,
	}, nil
}

func generateRunID() string {
	now := time.Now()
	// Generate 6 random bytes (12 hex chars) for uniqueness
	randBytes := make([]byte, 6)
	rand.Read(randBytes)
	return fmt.Sprintf("run_%s_%s", now.Format("20060102"), hex.EncodeToString(randBytes))
}
