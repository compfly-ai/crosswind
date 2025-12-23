// Package repository defines the repository interfaces for Agent-Eval.
package repository

import (
	"context"

	"github.com/agent-eval/agent-eval/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

// AgentRepository defines operations for agent persistence.
type AgentRepository interface {
	// Create creates a new agent.
	Create(ctx context.Context, agent *models.Agent) error

	// FindByID finds an agent by agent ID.
	FindByID(ctx context.Context, agentID string) (*models.Agent, error)

	// FindBySnapshot finds an agent by agent ID and snapshot ID.
	FindBySnapshot(ctx context.Context, agentID, snapshotID string) (*models.Agent, error)

	// List lists agents with optional status filter and pagination.
	List(ctx context.Context, status string, limit, offset int) ([]models.Agent, int64, error)

	// Update updates an agent with the given fields.
	Update(ctx context.Context, agentID string, update bson.M) error

	// Delete soft-deletes an agent.
	Delete(ctx context.Context, agentID string) error

	// Exists checks if an active (non-deleted) agent exists.
	Exists(ctx context.Context, agentID string) (bool, error)

	// HardDelete permanently removes a deleted agent.
	HardDelete(ctx context.Context, agentID string) error
}

// EvalRunRepository defines operations for evaluation run persistence.
type EvalRunRepository interface {
	// Create creates a new evaluation run.
	Create(ctx context.Context, run *models.EvalRun) error

	// FindByRunID finds an evaluation run by run ID.
	FindByRunID(ctx context.Context, runID string) (*models.EvalRun, error)

	// ListByAgent lists evaluation runs for an agent with optional status filter.
	ListByAgent(ctx context.Context, agentID string, status string, limit, offset int) ([]models.EvalRun, int64, error)

	// Update updates an evaluation run.
	Update(ctx context.Context, runID string, update bson.M) error

	// UpdateProgress updates the progress of an evaluation run.
	UpdateProgress(ctx context.Context, runID string, progress models.EvalProgress) error

	// UpdateStatus updates the status of an evaluation run.
	UpdateStatus(ctx context.Context, runID, status string) error

	// HasActiveRun checks if an agent has an active (pending or running) evaluation.
	HasActiveRun(ctx context.Context, agentID string) (bool, error)

	// GetLatestRun gets the most recent evaluation run for an agent.
	GetLatestRun(ctx context.Context, agentID string) (*models.EvalRun, error)

	// GetLatestRunsByAgentIDs gets the most recent evaluation run for multiple agents in a single query.
	// Returns a map of agentID -> EvalRun. Agents with no runs are not included in the map.
	GetLatestRunsByAgentIDs(ctx context.Context, agentIDs []string) (map[string]*models.EvalRun, error)
}

// ScenarioRepository defines operations for scenario set persistence.
type ScenarioRepository interface {
	// Create creates a new scenario set.
	Create(ctx context.Context, set *models.ScenarioSet) error

	// FindBySetID finds a scenario set by set ID.
	FindBySetID(ctx context.Context, setID string) (*models.ScenarioSet, error)

	// ListByAgent lists scenario sets for an agent.
	ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]models.ScenarioSet, int64, error)

	// Update updates a scenario set.
	Update(ctx context.Context, setID string, update bson.M) error

	// UpdateStatus updates the status of a scenario set.
	UpdateStatus(ctx context.Context, setID, status string) error

	// UpdateStatusWithError updates the status and stores an error message.
	UpdateStatusWithError(ctx context.Context, setID, status, errorMsg string) error

	// UpdateProgress updates generation progress for live tracking.
	UpdateProgress(ctx context.Context, setID string, generated, total int) error

	// UpdateStage updates the generation stage and message.
	UpdateStage(ctx context.Context, setID, stage, message string) error

	// AppendScenario appends a single scenario to the set.
	AppendScenario(ctx context.Context, setID string, scenario models.Scenario) error

	// UpdateScenarios updates the scenarios and summary.
	UpdateScenarios(ctx context.Context, setID string, scenarios []models.Scenario, summary models.ScenarioSummary) error

	// UpdateScenarioEnabled updates the enabled status of a specific scenario.
	UpdateScenarioEnabled(ctx context.Context, setID, scenarioID string, enabled bool) error

	// AddScenarios adds new scenarios to an existing set.
	AddScenarios(ctx context.Context, setID string, scenarios []models.Scenario) error

	// RemoveScenario removes a scenario from a set.
	RemoveScenario(ctx context.Context, setID, scenarioID string) error

	// UpdateScenario updates specific fields of a scenario within a set.
	UpdateScenario(ctx context.Context, setID, scenarioID string, update bson.M) error

	// UpdateSummary updates only the summary of a scenario set.
	UpdateSummary(ctx context.Context, setID string, summary models.ScenarioSummary) error

	// UpdatePlan stores the generation plan in progress.
	UpdatePlan(ctx context.Context, setID string, plan *models.GenerationPlan) error

	// UpdateBatches updates the batch status in the plan.
	UpdateBatches(ctx context.Context, setID string, batches []models.GenerationBatch) error

	// Delete deletes a scenario set.
	Delete(ctx context.Context, setID string) error
}

// ContextRepository defines operations for context document persistence.
type ContextRepository interface {
	// Create creates a new context.
	Create(ctx context.Context, ctxDoc *models.Context) error

	// FindByID finds a context by context ID.
	FindByID(ctx context.Context, contextID string) (*models.Context, error)

	// FindByIDs finds multiple contexts by their IDs.
	FindByIDs(ctx context.Context, contextIDs []string) ([]models.Context, error)

	// List lists contexts with pagination.
	List(ctx context.Context, limit, offset int) ([]models.Context, int64, error)

	// Update updates a context.
	Update(ctx context.Context, contextID string, update bson.M) error

	// UpdateStatus updates a context status and optionally error message.
	UpdateStatus(ctx context.Context, contextID, status, errorMsg string) error

	// UpdateFileStatus updates a specific file's status within a context.
	UpdateFileStatus(ctx context.Context, contextID, fileName, status string, metadata map[string]interface{}) error

	// UpdateSummary updates the context summary.
	UpdateSummary(ctx context.Context, contextID string, summary *models.ContextSummary) error

	// AddFiles appends new files to an existing context.
	AddFiles(ctx context.Context, contextID string, files []models.ContextFile) error

	// Delete permanently removes a context.
	Delete(ctx context.Context, contextID string) error

	// Exists checks if a context exists.
	Exists(ctx context.Context, contextID string) (bool, error)
}

// DatasetRepository defines operations for dataset persistence.
type DatasetRepository interface {
	// ListDatasets lists available datasets.
	ListDatasets(ctx context.Context, category string, isActive bool) ([]models.Dataset, int64, error)

	// FindDataset finds a dataset by ID.
	FindDataset(ctx context.Context, datasetID string) (*models.Dataset, error)

	// GetPrompts gets prompts for a dataset with pagination.
	GetPrompts(ctx context.Context, datasetID, version string, limit, offset int) ([]models.DatasetPrompt, int64, error)

	// GetDistinctCategories returns all unique categories from active shared datasets.
	GetDistinctCategories(ctx context.Context, isActive bool) ([]string, error)

	// GetDistinctEvalTypes returns all unique eval types from active shared datasets.
	GetDistinctEvalTypes(ctx context.Context, isActive bool) ([]string, error)
}

// ResultsRepository defines operations for evaluation results persistence.
type ResultsRepository interface {
	// Create creates a new results summary.
	Create(ctx context.Context, summary *models.EvalResultsSummary) error

	// FindByRunID finds results summary by run ID.
	FindByRunID(ctx context.Context, runID string) (*models.EvalResultsSummary, error)

	// Update updates results summary.
	Update(ctx context.Context, runID string, update bson.M) error

	// AppendFailure appends a failure to the results.
	AppendFailure(ctx context.Context, runID string, failure models.PromptResultDetail) error

	// AppendSamplePass appends a sample pass to the results.
	AppendSamplePass(ctx context.Context, runID, category string, pass models.PromptResultDetail, maxSamplesPerCategory int) error
}

// Repositories groups all repository interfaces for dependency injection.
type Repositories struct {
	Agents    AgentRepository
	EvalRuns  EvalRunRepository
	Scenarios ScenarioRepository
	Contexts  ContextRepository
	Datasets  DatasetRepository
	Results   ResultsRepository
}
