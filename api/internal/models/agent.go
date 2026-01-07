package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Agent represents an AI agent registered for evaluation
type Agent struct {
	ID                     primitive.ObjectID      `bson:"_id,omitempty" json:"-"`
	AgentID                string                  `bson:"agentId" json:"agentId"`
	Name                   string                  `bson:"name" json:"name"`
	Description            string                  `bson:"description" json:"description"`
	Goal                   string                  `bson:"goal" json:"goal"`
	Industry               string                  `bson:"industry" json:"industry"`
	SystemPrompt           string                  `bson:"systemPrompt,omitempty" json:"systemPrompt,omitempty"`
	EndpointConfig         EndpointConfig          `bson:"endpointConfig" json:"endpointConfig"`
	AuthConfig             AuthConfig              `bson:"authConfig" json:"authConfig"`
	InferredSchema         *InferredAPISchema      `bson:"inferredSchema,omitempty" json:"inferredSchema,omitempty"`
	RateLimits             *RateLimits             `bson:"rateLimits,omitempty" json:"rateLimits,omitempty"`
	SessionStrategy        string                  `bson:"sessionStrategy" json:"sessionStrategy"`
	DeclaredCapabilities   *AgentCapabilities      `bson:"declaredCapabilities,omitempty" json:"declaredCapabilities,omitempty"`
	DiscoveredCapabilities *DiscoveredCapabilities `bson:"discoveredCapabilities,omitempty" json:"discoveredCapabilities,omitempty"`
	Status                 string                  `bson:"status" json:"status"`
	CreatedAt              time.Time               `bson:"createdAt" json:"createdAt"`
	UpdatedAt              time.Time               `bson:"updatedAt" json:"updatedAt"`
}

// EndpointConfig holds the agent's API endpoint configuration
type EndpointConfig struct {
	// Protocol determines which adapter and validation rules to use
	// Platform protocols: openai, azure_openai, langgraph, bedrock, vertex
	// Generic protocols: custom (HTTP), custom_ws (WebSocket)
	// Future protocols: a2a, mcp
	Protocol string `bson:"protocol" json:"protocol"`

	// === Platform protocol fields ===

	// OpenAI/Azure OpenAI: prompt ID for Responses API (preferred)
	PromptID string `bson:"promptId,omitempty" json:"promptId,omitempty"`
	// OpenAI/Azure OpenAI: assistant ID for Assistants API (legacy, deprecated Aug 2026)
	// LangGraph: also uses assistantId for deployed assistants
	AssistantID string `bson:"assistantId,omitempty" json:"assistantId,omitempty"`
	// OpenAI Agent Builder: workflow ID for ChatKit-based agents
	WorkflowID string `bson:"workflowId,omitempty" json:"workflowId,omitempty"`

	// Bedrock: agent ID and optional alias
	AgentID      string `bson:"agentId,omitempty" json:"agentId,omitempty"`
	AgentAliasID string `bson:"agentAliasId,omitempty" json:"agentAliasId,omitempty"`

	// Vertex AI: reasoning engine ID and project
	ReasoningEngineID string `bson:"reasoningEngineId,omitempty" json:"reasoningEngineId,omitempty"`
	ProjectID         string `bson:"projectId,omitempty" json:"projectId,omitempty"`

	// AWS/GCP: region
	Region string `bson:"region,omitempty" json:"region,omitempty"`

	// Azure OpenAI / LangGraph: base URL (required)
	BaseURL string `bson:"baseUrl,omitempty" json:"baseUrl,omitempty"`

	// === Custom protocol fields ===

	// Full URL to the conversation endpoint (e.g., "https://my-agent.example.com/api/v1/chat")
	Endpoint string `bson:"endpoint,omitempty" json:"endpoint,omitempty"`
	// Optional: URL to OpenAPI spec for API analysis
	SpecURL string `bson:"specUrl,omitempty" json:"specUrl,omitempty"`
	// Optional: separate session endpoint if different from derived base URL
	SessionEndpoint string `bson:"sessionEndpoint,omitempty" json:"sessionEndpoint,omitempty"`
	// Optional: health check endpoint
	HealthEndpoint string `bson:"healthEndpoint,omitempty" json:"healthEndpoint,omitempty"`

	// === A2A protocol fields ===

	// Full URL to the Agent Card (e.g., "https://my-agent.example.com/.well-known/agent.json")
	AgentCardURL string `bson:"agentCardUrl,omitempty" json:"agentCardUrl,omitempty"`
	// Discovered endpoint URL from AgentCard interfaces (e.g., "https://my-agent.example.com/a2a")
	A2AEndpoint string `bson:"a2aEndpoint,omitempty" json:"a2aEndpoint,omitempty"`
	// Discovered interface type: "http" or "websocket"
	A2AInterfaceType string `bson:"a2aInterfaceType,omitempty" json:"a2aInterfaceType,omitempty"`

	// === MCP protocol fields ===

	// MCP transport type: "sse" or "streamable_http"
	MCPTransport string `bson:"mcpTransport,omitempty" json:"mcpTransport,omitempty"`
	// MCP tool name - the specific tool to treat as an agent
	MCPToolName string `bson:"mcpToolName,omitempty" json:"mcpToolName,omitempty"`
	// MCP message field - the primary text input field discovered from tool schema
	MCPMessageField string `bson:"mcpMessageField,omitempty" json:"mcpMessageField,omitempty"`
	// Uses Endpoint field for MCP server URL
}

// AuthConfig holds authentication configuration for the agent (stored in DB)
type AuthConfig struct {
	// Type: none, bearer, api_key, basic, aws, azure_entra, google_oauth, custom
	Type         string `bson:"type" json:"type"`
	Credentials  string `bson:"credentials" json:"-"` // Never expose in API responses
	HeaderName   string `bson:"headerName,omitempty" json:"headerName,omitempty"`
	HeaderPrefix string `bson:"headerPrefix,omitempty" json:"headerPrefix,omitempty"`
	// AWS-specific: region for SigV4 signing (if different from endpoint region)
	AWSRegion string `bson:"awsRegion,omitempty" json:"awsRegion,omitempty"`
	// Azure-specific: tenant ID for Entra authentication
	AzureTenantID string `bson:"azureTenantId,omitempty" json:"azureTenantId,omitempty"`
}

// AuthConfigInput is used for API requests (allows credentials input)
type AuthConfigInput struct {
	// Type: none, bearer, api_key, basic, aws, azure_entra, google_oauth, custom
	Type         string `json:"type"`
	Credentials  string `json:"credentials"`
	HeaderName   string `json:"headerName,omitempty"`
	HeaderPrefix string `json:"headerPrefix,omitempty"`
	// AWS-specific: region for SigV4 signing
	AWSRegion string `json:"awsRegion,omitempty"`
	// Azure-specific: tenant ID for Entra authentication
	AzureTenantID string `json:"azureTenantId,omitempty"`
}

// AgentCapabilities represents declared capabilities of an agent
type AgentCapabilities struct {
	// Quick-start mode: just tool names (e.g., ["salesforce", "slack"])
	// These are expanded using the KnownTools registry
	Tools []string `bson:"tools,omitempty" json:"tools,omitempty"`

	// Advanced mode: full tool definitions with permissions
	ToolDefinitions []ToolDefinition `bson:"toolDefinitions,omitempty" json:"toolDefinitions,omitempty"`

	HasMemory          bool     `bson:"hasMemory" json:"hasMemory"`
	HasTools           bool     `bson:"hasTools" json:"hasTools"`
	HasRAG             bool     `bson:"hasRag" json:"hasRag"`
	SupportedLanguages []string `bson:"supportedLanguages,omitempty" json:"supportedLanguages,omitempty"`

	// Types of sensitive data the agent may access (for agentic evaluation)
	// e.g., ["pii", "phi", "financial", "confidential"]
	SensitiveDataTypes []string `bson:"sensitiveDataTypes,omitempty" json:"sensitiveDataTypes,omitempty"`
}

// ToolDefinition represents a detailed tool/integration definition
type ToolDefinition struct {
	Name         string   `bson:"name" json:"name"`
	Type         string   `bson:"type,omitempty" json:"type,omitempty"`               // mcp, openapi, custom, crm, messaging, etc.
	Permissions  []string `bson:"permissions,omitempty" json:"permissions,omitempty"` // e.g., ["read:contacts", "write:opportunities"]
	CanAccessPII bool     `bson:"canAccessPii,omitempty" json:"canAccessPii,omitempty"`
	Description  string   `bson:"description,omitempty" json:"description,omitempty"`
}

// RateLimits holds rate limiting configuration for agents
type RateLimits struct {
	RequestsPerMinute  int `bson:"requestsPerMinute" json:"requestsPerMinute"`
	ConcurrentSessions int `bson:"concurrentSessions" json:"concurrentSessions"`
	MaxTimeoutSeconds  int `bson:"maxTimeoutSeconds" json:"maxTimeoutSeconds"`
}

// DefaultRateLimits returns the default rate limiting configuration
func DefaultRateLimits() RateLimits {
	return RateLimits{
		RequestsPerMinute:  30,
		ConcurrentSessions: 3,
		MaxTimeoutSeconds:  120,
	}
}

// SensitiveDataType constants
const (
	SensitiveDataPII          = "pii"          // Personally identifiable information
	SensitiveDataPHI          = "phi"          // Protected health information
	SensitiveDataFinancial    = "financial"    // Financial data
	SensitiveDataConfidential = "confidential" // Business confidential
	SensitiveDataCredentials  = "credentials"  // API keys, passwords, tokens
)

// DiscoveredCapabilities represents capabilities discovered during evaluation
type DiscoveredCapabilities struct {
	HasMemory        bool      `bson:"hasMemory" json:"hasMemory"`
	HasTools         bool      `bson:"hasTools" json:"hasTools"`
	Tools            []string  `bson:"tools,omitempty" json:"tools,omitempty"`
	UndeclaredTools  []string  `bson:"undeclaredTools,omitempty" json:"undeclaredTools,omitempty"`
	LastDiscoveryRun time.Time `bson:"lastDiscoveryRun,omitempty" json:"lastDiscoveryRun,omitempty"`
}

// InferredAPISchema represents the API structure inferred by GPT-5.1
type InferredAPISchema struct {
	// API Style - determines how messages are formatted
	//
	// Core styles:
	// "chat_stateless" - Full history sent each request as messages array [{role, content}...] (OpenAI/Claude style)
	// "single_message" - Single message string per request, server may track context via session_id
	// "thread_based"   - Messages added to server-managed thread (OpenAI Assistants style)
	// "task_based"     - Task-oriented with contextId grouping (Google A2A style)
	//
	// Framework-specific styles:
	// "langserve"  - LangChain LangServe: {input: {key: value}, config: {...}} with /invoke, /stream endpoints
	// "flowise"    - Flowise prediction: {question: "...", history: [], sessionId: "..."}
	// "dify"       - Dify workflow: {inputs: {...}, user: "...", conversation_id: "..."}
	// "haystack"   - Haystack Hayhooks: dynamic pipeline inputs, often {query: "..."}
	// "botpress"   - Botpress: {conversationId, userId, type, payload: {text: "..."}}
	APIStyle string `bson:"apiStyle" json:"apiStyle"`

	// Request configuration
	RequestMethod      string `bson:"requestMethod" json:"requestMethod"`           // POST, GET, etc.
	RequestContentType string `bson:"requestContentType" json:"requestContentType"` // application/json, etc.
	MessageField       string `bson:"messageField" json:"messageField"`             // e.g., "messages", "message", "prompt", "input"
	SessionIDField     string `bson:"sessionIdField,omitempty" json:"sessionIdField,omitempty"`
	HistoryField       string `bson:"historyField,omitempty" json:"historyField,omitempty"` // Deprecated: use APIStyle instead
	AdditionalFields   map[string]interface{} `bson:"additionalFields,omitempty" json:"additionalFields,omitempty"`

	// Response configuration
	ResponseContentField string `bson:"responseContentField" json:"responseContentField"` // e.g., "response", "content", "choices[0].message.content"
	ResponseErrorField   string `bson:"responseErrorField,omitempty" json:"responseErrorField,omitempty"`
	StreamingSupported   bool   `bson:"streamingSupported" json:"streamingSupported"`

	// Session management
	SessionIDInResponse  string `bson:"sessionIdInResponse,omitempty" json:"sessionIdInResponse,omitempty"`
	SessionIDInHeader    string `bson:"sessionIdInHeader,omitempty" json:"sessionIdInHeader,omitempty"`
	SessionCreateMethod  string `bson:"sessionCreateMethod,omitempty" json:"sessionCreateMethod,omitempty"` // "auto", "explicit", "none"

	// Metadata
	InferredAt      time.Time `bson:"inferredAt" json:"inferredAt"`
	InferenceMethod string    `bson:"inferenceMethod" json:"inferenceMethod"` // "openapi_spec", "probe", "gpt_analysis"
	Confidence      float64   `bson:"confidence" json:"confidence"`           // 0.0 to 1.0
	RawAnalysis     string    `bson:"rawAnalysis,omitempty" json:"rawAnalysis,omitempty"` // GPT's reasoning
}

// AgentStatus constants
const (
	AgentStatusActive   = "active"
	AgentStatusInactive = "inactive"
	AgentStatusDeleted  = "deleted"
)

// Protocol constants - Platform-specific protocols
const (
	// Platform protocols (use native SDKs)
	ProtocolOpenAI      = "openai"       // OpenAI Responses/Conversations API
	ProtocolAzureOpenAI = "azure_openai" // Azure OpenAI (same API, Entra auth)
	ProtocolLangGraph   = "langgraph"    // LangGraph Platform (threads/runs)
	ProtocolBedrock     = "bedrock"      // AWS Bedrock Agents
	ProtocolVertex      = "vertex"       // Google Vertex AI Agent Engine

	// Generic protocols (custom HTTP adapters)
	ProtocolCustom   = "custom"    // Generic HTTP API
	ProtocolCustomWS = "custom_ws" // Generic WebSocket API

	// Future protocols (V2)
	ProtocolA2A = "a2a" // Google Agent-to-Agent protocol
	ProtocolMCP = "mcp" // Model Context Protocol
)

// AuthType constants
const (
	AuthTypeNone        = "none"
	AuthTypeBearer      = "bearer"
	AuthTypeAPIKey      = "api_key"
	AuthTypeBasic       = "basic"
	AuthTypeAWS         = "aws"          // AWS SigV4 (access_key:secret_key)
	AuthTypeAzureEntra  = "azure_entra"  // Azure Entra ID token
	AuthTypeGoogleOAuth = "google_oauth" // Google OAuth/Service Account
	AuthTypeCustom      = "custom"
)

// SessionStrategy constants
const (
	SessionStrategyAgentManaged = "agent_managed"
	SessionStrategyClientHistory = "client_history"
	SessionStrategyAutoDetect   = "auto_detect"
)

// CreateAgentRequest represents the request body for creating an agent
type CreateAgentRequest struct {
	AgentID              string             `json:"agentId" binding:"required"`
	Name                 string             `json:"name"`                         // Auto-populated for A2A/MCP
	Description          string             `json:"description"`                  // Auto-populated for A2A/MCP
	Goal                 string             `json:"goal"`                         // Auto-populated for A2A/MCP
	Industry             string             `json:"industry" binding:"required"`
	SystemPrompt         string             `json:"systemPrompt,omitempty"`
	EndpointConfig       EndpointConfig     `json:"endpointConfig" binding:"required"`
	AuthConfig           AuthConfigInput    `json:"authConfig" binding:"required"`
	RateLimits           *RateLimits        `json:"rateLimits,omitempty"`
	SessionStrategy      string             `json:"sessionStrategy,omitempty"`
	DeclaredCapabilities *AgentCapabilities `json:"declaredCapabilities,omitempty"`
}

// UpdateAgentRequest represents the request body for updating an agent
type UpdateAgentRequest struct {
	Name                 *string            `json:"name,omitempty"`
	Description          *string            `json:"description,omitempty"`
	Goal                 *string            `json:"goal,omitempty"`
	Industry             *string            `json:"industry,omitempty"`
	SystemPrompt         *string            `json:"systemPrompt,omitempty"`
	EndpointConfig       *EndpointConfig    `json:"endpointConfig,omitempty"`
	AuthConfig           *AuthConfigInput   `json:"authConfig,omitempty"`
	RateLimits           *RateLimits        `json:"rateLimits,omitempty"`
	SessionStrategy      *string            `json:"sessionStrategy,omitempty"`
	DeclaredCapabilities *AgentCapabilities `json:"declaredCapabilities,omitempty"`
	Status               *string            `json:"status,omitempty"`
}

// AgentListResponse represents a paginated list of agents
type AgentListResponse struct {
	Agents []AgentSummary `json:"agents"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// AgentSummary represents a summarized view of an agent for listings
type AgentSummary struct {
	AgentID     string     `json:"agentId"`
	Name        string     `json:"name"`
	Industry    string     `json:"industry"`
	Status      string     `json:"status"`
	LastEvalRun *time.Time `json:"lastEvalRun,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}
