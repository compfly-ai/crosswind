// Package api provides the public API for the agent-eval platform.
//
// This package exposes:
// - Models: Data structures for agents, evals, scenarios, etc.
// - Services: Business logic components
// - Repositories: Data access interfaces
package api

import (
	"github.com/compfly-ai/crosswind/api/internal/config"
	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/compfly-ai/crosswind/api/internal/queue"
	"github.com/compfly-ai/crosswind/api/internal/repository/clickhouse"
	"github.com/compfly-ai/crosswind/api/internal/repository/mongo"
	"github.com/compfly-ai/crosswind/api/internal/services"
	"github.com/compfly-ai/crosswind/api/pkg/repository"
	"github.com/compfly-ai/crosswind/api/pkg/storage"
)

// ============================================================================
// Agent Models
// ============================================================================

type Agent = models.Agent
type CreateAgentRequest = models.CreateAgentRequest
type UpdateAgentRequest = models.UpdateAgentRequest
type AgentListResponse = models.AgentListResponse
type AgentSummary = models.AgentSummary
type InferredAPISchema = models.InferredAPISchema
type EndpointConfig = models.EndpointConfig
type AuthConfig = models.AuthConfig
type AuthConfigInput = models.AuthConfigInput
type AgentCapabilities = models.AgentCapabilities
type ToolDefinition = models.ToolDefinition
type RateLimits = models.RateLimits
type DiscoveredCapabilities = models.DiscoveredCapabilities

var DefaultRateLimits = models.DefaultRateLimits

// ============================================================================
// A2A Protocol Models (Agent-to-Agent spec v0.3)
// ============================================================================

type A2AAgentCard = models.A2AAgentCard
type A2AProvider = models.A2AProvider
type A2ACapabilities = models.A2ACapabilities
type A2AInterface = models.A2AInterface
type A2ASecurityScheme = models.A2ASecurityScheme
type A2ASkill = models.A2ASkill
type A2AExtension = models.A2AExtension

// ============================================================================
// MCP Protocol Models (Model Context Protocol)
// ============================================================================

type MCPToolInfo = services.MCPToolInfo
type MCPServerInfo = services.MCPServerInfo
type MCPDiscoveryResult = services.MCPDiscoveryResult

// FindMessageField identifies the primary text input field from an MCP tool's input schema.
var FindMessageField = services.FindMessageField

// ============================================================================
// Eval Models
// ============================================================================

type EvalRun = models.EvalRun
type EvalRunConfig = models.EvalRunConfig
type CreateEvalRunRequest = models.CreateEvalRunRequest
type CreateEvalRunResponse = models.CreateEvalRunResponse
type EvalRunListResponse = models.EvalRunListResponse
type EvalRunSummary = models.EvalRunSummary
type EvalProgress = models.EvalProgress
type DatasetUsed = models.DatasetUsed
type ScenarioSetUsed = models.ScenarioSetUsed
type SummaryScores = models.SummaryScores
type ThreatAnalysis = models.ThreatAnalysis
type TrustAnalysis = models.TrustAnalysis
type RefusalAnalysis = models.RefusalAnalysis
type Compliance = models.Compliance
type EvalRunConfigRequest = models.EvalRunConfigRequest
type Recommendation = models.Recommendation

// ============================================================================
// Result Models
// ============================================================================

type GetResultsResponse = models.GetResultsResponse
type EvalResultsSummary = models.EvalResultsSummary
type PromptResultDetail = models.PromptResultDetail
type CategoryStats = models.CategoryStats
type PerformanceMetrics = models.PerformanceMetrics

// ============================================================================
// Scenario Models
// ============================================================================

type ScenarioSet = models.ScenarioSet
type Scenario = models.Scenario
type ScenarioInput = models.ScenarioInput
type ScenarioUpdate = models.ScenarioUpdate
type GenerateScenariosRequest = models.GenerateScenariosRequest
type GenerateScenariosResponse = models.GenerateScenariosResponse
type ScenarioGenConfig = models.ScenarioGenConfig
type CategoryPlan = models.CategoryPlan
type UpdateScenariosRequest = models.UpdateScenariosRequest
type ImportScenariosRequest = models.ImportScenariosRequest
type ScenarioSetListResponse = models.ScenarioSetListResponse
type ScenarioSetSummary = models.ScenarioSetSummary
type ScenarioSummary = models.ScenarioSummary
type GenerationProgress = models.GenerationProgress
type GenerationPlan = models.GenerationPlan
type GenerationBatch = models.GenerationBatch

// ============================================================================
// Context Models
// ============================================================================

type Context = models.Context
type ContextFile = models.ContextFile
type ContextListResponse = models.ContextListResponse
type ContextSummary = models.ContextSummary

// ============================================================================
// Dataset Models
// ============================================================================

type Dataset = models.Dataset
type DatasetPrompt = models.DatasetPrompt
type DatasetListResponse = models.DatasetListResponse

// ============================================================================
// Status/Type Constants
// ============================================================================

const (
	// Agent statuses
	AgentStatusActive   = models.AgentStatusActive
	AgentStatusInactive = models.AgentStatusInactive
	AgentStatusDeleted  = models.AgentStatusDeleted

	// Protocol types
	ProtocolOpenAI           = models.ProtocolOpenAI
	ProtocolAzureOpenAI      = models.ProtocolAzureOpenAI
	ProtocolLangGraph        = models.ProtocolLangGraph
	ProtocolBedrock          = models.ProtocolBedrock
	ProtocolBedrockAgentCore = models.ProtocolBedrockAgentCore
	ProtocolVertex           = models.ProtocolVertex
	ProtocolCustom           = models.ProtocolCustom
	ProtocolCustomWS         = models.ProtocolCustomWS
	ProtocolA2A              = models.ProtocolA2A
	ProtocolMCP              = models.ProtocolMCP

	// Session strategies
	SessionStrategyAutoDetect    = models.SessionStrategyAutoDetect
	SessionStrategyAgentManaged  = models.SessionStrategyAgentManaged
	SessionStrategyClientHistory = models.SessionStrategyClientHistory

	// Eval statuses
	EvalStatusPending   = models.EvalStatusPending
	EvalStatusRunning   = models.EvalStatusRunning
	EvalStatusCompleted = models.EvalStatusCompleted
	EvalStatusFailed    = models.EvalStatusFailed
	EvalStatusCancelled = models.EvalStatusCancelled

	// Eval types
	EvalTypeRedTeam = models.EvalTypeRedTeam
	EvalTypeTrust   = models.EvalTypeTrust

	// Eval modes
	EvalModeQuick    = models.EvalModeQuick
	EvalModeStandard = models.EvalModeStandard
	EvalModeDeep     = models.EvalModeDeep

	// Scenario statuses
	ScenarioStatusPending    = models.ScenarioStatusPending
	ScenarioStatusGenerating = models.ScenarioStatusGenerating
	ScenarioStatusReady      = models.ScenarioStatusReady
	ScenarioStatusFailed     = models.ScenarioStatusFailed

	// Scenario generation stages
	StagePlanning         = models.StagePlanning
	StagePreparingContext = models.StagePreparingContext
	StageProcessingDocs   = models.StageProcessingDocs
	StageGenerating       = models.StageGenerating
	StageComplete         = models.StageComplete
	StageFailed           = models.StageFailed

	// Context statuses
	ContextStatusProcessing = models.ContextStatusProcessing
	ContextStatusReady      = models.ContextStatusReady
	ContextStatusFailed     = models.ContextStatusFailed
)

// ============================================================================
// Repositories
// ============================================================================

type Repositories = mongo.Repositories

var NewRepositories = mongo.NewRepositories

// ============================================================================
// Repository Interfaces
// ============================================================================

// Re-export repository interfaces from pkg/repository
type AgentRepository = repository.AgentRepository
type EvalRunRepository = repository.EvalRunRepository
type ScenarioRepository = repository.ScenarioRepository
type ContextRepository = repository.ContextRepository
type DatasetRepository = repository.DatasetRepository
type ResultsRepository = repository.ResultsRepository

// Re-export the Repositories struct for convenience
type RepositoryInterfaces = repository.Repositories

// ============================================================================
// Services
// ============================================================================

type AgentService = services.AgentService
type EvalService = services.EvalService
type ScenarioService = services.ScenarioService
type ScenarioGenerator = services.ScenarioGenerator
type ContextService = services.ContextService
type DatasetService = services.DatasetService
type AnalyticsService = services.AnalyticsService
type APIAnalyzer = services.APIAnalyzer

var NewAgentService = services.NewAgentService
var NewEvalService = services.NewEvalService
var NewScenarioService = services.NewScenarioService
var NewScenarioGenerator = services.NewScenarioGenerator
var NewContextService = services.NewContextService
var NewDatasetService = services.NewDatasetService
var NewAnalyticsService = services.NewAnalyticsService
var NewAPIAnalyzer = services.NewAPIAnalyzer

// ============================================================================
// Service Errors
// ============================================================================

var (
	// Agent errors
	ErrAgentNotFound            = services.ErrAgentNotFound
	ErrAgentAlreadyExists       = services.ErrAgentAlreadyExists
	ErrInvalidProtocol          = services.ErrInvalidProtocol
	ErrMissingBaseURL           = services.ErrMissingBaseURL
	ErrMissingEndpoint          = services.ErrMissingEndpoint
	ErrMissingAgentIdentifier   = services.ErrMissingAgentIdentifier
	ErrMissingAgentID           = services.ErrMissingAgentID
	ErrMissingAgentRuntimeArn   = services.ErrMissingAgentRuntimeArn
	ErrMissingProjectID         = services.ErrMissingProjectID
	ErrMissingReasoningEngineID = services.ErrMissingReasoningEngineID
	ErrMissingAgentCardURL      = services.ErrMissingAgentCardURL
	ErrMissingMCPTransport      = services.ErrMissingMCPTransport

	// Eval errors
	ErrEvalRunNotFound    = services.ErrEvalRunNotFound
	ErrEvalAlreadyRunning = services.ErrEvalAlreadyRunning
	ErrEvalNotCancellable = services.ErrEvalNotCancellable
	ErrInvalidEvalMode    = services.ErrInvalidEvalMode
	ErrResultsNotReady    = services.ErrResultsNotReady

	// Scenario errors
	ErrScenarioSetNotFound = services.ErrScenarioSetNotFound

	// Context errors
	ErrContextNotFound     = services.ErrContextNotFound
	ErrNoFilesProvided     = services.ErrNoFilesProvided
	ErrUnsupportedFileType = services.ErrUnsupportedFileType
	ErrFileTooLarge        = services.ErrFileTooLarge
)

// ============================================================================
// Configuration
// ============================================================================

type Config = config.Config

var LoadConfig = config.Load

// ============================================================================
// Queue
// ============================================================================

type RedisQueue = queue.RedisQueue
type EvalJob = queue.EvalJob

var NewRedisQueue = queue.NewRedisQueue

// Queue errors
var ErrJobAlreadyEnqueued = queue.ErrJobAlreadyEnqueued

// ============================================================================
// Storage
// ============================================================================

// FileStorage is the storage interface for file operations
type FileStorage = storage.FileStorage
type FileInfo = storage.FileInfo
type FileMetadata = storage.FileMetadata

// Storage providers
type StorageProvider = storage.Provider

const (
	StorageProviderLocal = storage.ProviderLocal
	StorageProviderGCS   = storage.ProviderGCS
)

// Storage configuration and factory
type StorageConfig = storage.Config

var NewFileStorage = storage.NewFileStorage
var NewFileStorageWithContext = storage.NewFileStorageWithContext
var LoadStorageConfig = storage.LoadConfig

// GCSClient provides Google Cloud Storage integration
type GCSClient = storage.GCSClient

var NewGCSClient = storage.NewGCSClient
var BuildContextPath = storage.BuildContextPath
var BuildFilePath = storage.BuildFilePath
var BuildProcessedPath = storage.BuildProcessedPath

// ============================================================================
// Model Helper Functions
// ============================================================================

var GetDefaultFocusAreas = models.GetDefaultFocusAreas
var ValidateFocusAreas = models.ValidateFocusAreas

// ============================================================================
// ClickHouse (Analytics)
// ============================================================================

type ClickHouseClient = clickhouse.Client
type ClickHouseConfig = clickhouse.Config
type EvalDetail = clickhouse.EvalDetail
type EvalSession = clickhouse.EvalSession

var NewClickHouseClient = clickhouse.NewClient
