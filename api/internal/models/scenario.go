package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScenarioSet represents a set of generated attack scenarios for an agent
type ScenarioSet struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	SetID     string             `bson:"setId" json:"setId"`
	AgentID   string             `bson:"agentId" json:"agentId"`
	Status    string             `bson:"status" json:"status"`
	Config    ScenarioGenConfig  `bson:"config" json:"config"`
	Scenarios []Scenario         `bson:"scenarios" json:"scenarios"`
	Summary   ScenarioSummary    `bson:"summary" json:"summary"`
	// Progress tracking for live updates
	Progress  *GenerationProgress `bson:"progress,omitempty" json:"progress,omitempty"`
	// Error message if generation failed
	Error     string              `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time           `bson:"updatedAt" json:"updatedAt"`
}

// GenerationProgress tracks live generation progress
type GenerationProgress struct {
	Total       int              `bson:"total" json:"total"`             // Target count (from plan)
	Generated   int              `bson:"generated" json:"generated"`     // Scenarios generated so far
	Stage       string           `bson:"stage" json:"stage"`             // Current stage: planning, preparing, processing_context, generating, complete
	Message     string           `bson:"message" json:"message"`         // User-friendly status message
	Plan        *GenerationPlan  `bson:"plan,omitempty" json:"plan,omitempty"` // The generation plan
	LastUpdated time.Time        `bson:"lastUpdated" json:"lastUpdated"` // Last progress update
}

// GenerationPlan represents the intelligent scenario generation plan
type GenerationPlan struct {
	RequestedCount    int                    `bson:"requestedCount" json:"requestedCount"`       // What user asked for
	RecommendedCount  int                    `bson:"recommendedCount" json:"recommendedCount"`   // What we recommend
	Rationale         string                 `bson:"rationale" json:"rationale"`                 // Why we recommend this count
	CategoryBreakdown []CategoryPlan         `bson:"categoryBreakdown" json:"categoryBreakdown"` // Per-category plan
	Batches           []GenerationBatch      `bson:"batches" json:"batches"`                     // Execution batches
	Warnings          []string               `bson:"warnings,omitempty" json:"warnings,omitempty"` // Any warnings for user
	AgentAnalysis     *AgentCapabilityAnalysis `bson:"agentAnalysis,omitempty" json:"agentAnalysis,omitempty"` // Analysis of agent
}

// CategoryPlan represents the plan for a single category
type CategoryPlan struct {
	Category      string   `bson:"category" json:"category"`           // Focus area
	Recommended   int      `bson:"recommended" json:"recommended"`     // Scenarios for this category
	Rationale     string   `bson:"rationale" json:"rationale"`         // Why this count
	Subcategories []string `bson:"subcategories" json:"subcategories"` // Subcategories to cover
	Priority      int      `bson:"priority" json:"priority"`           // 1=high, 2=medium, 3=low
}

// GenerationBatch represents a batch of scenarios to generate
type GenerationBatch struct {
	BatchID    string   `bson:"batchId" json:"batchId"`       // Unique batch identifier
	Category   string   `bson:"category" json:"category"`     // Focus area for this batch
	Count      int      `bson:"count" json:"count"`           // Scenarios to generate
	Status     string   `bson:"status" json:"status"`         // pending, generating, complete, failed
	Generated  int      `bson:"generated" json:"generated"`   // Scenarios generated so far
	Variation  string   `bson:"variation,omitempty" json:"variation,omitempty"` // For multiple batches in same category
}

// AgentCapabilityAnalysis represents the planner's analysis of the agent
type AgentCapabilityAnalysis struct {
	ToolCount        int      `bson:"toolCount" json:"toolCount"`               // Number of tools
	ToolCategories   []string `bson:"toolCategories" json:"toolCategories"`     // Types of tools (read, write, admin, etc.)
	AttackSurface    string   `bson:"attackSurface" json:"attackSurface"`       // limited, moderate, extensive
	DataSensitivity  string   `bson:"dataSensitivity" json:"dataSensitivity"`   // low, medium, high, critical
	RiskFactors      []string `bson:"riskFactors" json:"riskFactors"`           // Key risk factors identified
}

// Generation stages
const (
	StagePlanning           = "planning"           // Creating generation plan
	StagePreparingContext   = "preparing_context"  // Fetching context documents
	StageProcessingDocs     = "processing_documents"
	StageGenerating         = "generating"         // Generating scenarios in batches
	StageComplete           = "complete"
	StageFailed             = "failed"
)

// Batch status constants
const (
	BatchStatusPending    = "pending"
	BatchStatusGenerating = "generating"
	BatchStatusComplete   = "complete"
	BatchStatusFailed     = "failed"
)

// ScenarioGenConfig holds the configuration for scenario generation
type ScenarioGenConfig struct {
	EvalType           string   `bson:"evalType" json:"evalType"`                                     // "red_team" or "trust"
	Tools              []string `bson:"tools" json:"tools"`
	FocusAreas         []string `bson:"focusAreas" json:"focusAreas"`
	CustomInstructions string   `bson:"customInstructions,omitempty" json:"customInstructions,omitempty"` // Additional instructions for scenario generation
	ContextIDs         []string `bson:"contextIds,omitempty" json:"contextIds,omitempty"`             // Context document IDs
	ContextContent     string   `bson:"-" json:"-"`                                                   // Extracted text (not persisted)
	Industry           string   `bson:"industry,omitempty" json:"industry,omitempty"`
	Count              int      `bson:"count" json:"count"`
	IncludeMultiTurn   bool     `bson:"includeMultiTurn" json:"includeMultiTurn"`
}

// Scenario represents a single test scenario (red_team attack or trust quality test)
type Scenario struct {
	ID               string         `bson:"id" json:"id"`
	Category         string         `bson:"category" json:"category"`
	Subcategory      string         `bson:"subcategory,omitempty" json:"subcategory,omitempty"`
	Tool             string         `bson:"tool,omitempty" json:"tool,omitempty"`
	Severity         string         `bson:"severity" json:"severity"`
	Prompt           string         `bson:"prompt" json:"prompt"`
	ScenarioType     string         `bson:"scenarioType" json:"scenarioType"` // Attack vector (red_team) or test type (trust)
	// ExpectedBehavior is the enum value: refuse, comply, comply_with_caveats, redirect, context_dependent, comply_safe
	ExpectedBehavior string `bson:"expectedBehavior" json:"expectedBehavior"`
	// ExpectedBehaviorDescription is a detailed description for judge context (e.g., "The agent should deny bulk export...")
	ExpectedBehaviorDescription string         `bson:"expectedBehaviorDescription,omitempty" json:"expectedBehaviorDescription,omitempty"`
	Tags                        []string       `bson:"tags,omitempty" json:"tags,omitempty"`
	MultiTurn                   bool           `bson:"multiTurn" json:"multiTurn"`
	Turns                       []ScenarioTurn `bson:"turns,omitempty" json:"turns,omitempty"`
	Enabled                     bool           `bson:"enabled" json:"enabled"`
	Rationale                   string         `bson:"rationale,omitempty" json:"rationale,omitempty"`

	// Trust evaluation specific fields
	GroundTruth       []string `bson:"groundTruth,omitempty" json:"groundTruth,omitempty"`             // Patterns for good response
	FailureIndicators []string `bson:"failureIndicators,omitempty" json:"failureIndicators,omitempty"` // Patterns indicating failure

	// Agentic evaluation fields
	// ToolContext: Tools to mention in prompt context for agentic scenarios (e.g., ["salesforce", "slack"])
	ToolContext []string `bson:"toolContext,omitempty" json:"toolContext,omitempty"`
	// AgenticAttackVector: Type of agentic attack (direct, indirect, multi_turn, tool_response, memory, inter_agent)
	AgenticAttackVector string `bson:"agenticAttackVector,omitempty" json:"agenticAttackVector,omitempty"`
	// MaestroThreat: MAESTRO framework threat ID (T1-T8) for categorization
	MaestroThreat string `bson:"maestroThreat,omitempty" json:"maestroThreat,omitempty"`
	// OwaspAsiThreat: OWASP Agentic AI Top 10 threat ID (ASI01-ASI10)
	OwaspAsiThreat string `bson:"owaspAsiThreat,omitempty" json:"owaspAsiThreat,omitempty"`
	// FullSuccessIndicators: Patterns indicating full attack success (agent fully complied)
	FullSuccessIndicators []string `bson:"fullSuccessIndicators,omitempty" json:"fullSuccessIndicators,omitempty"`
	// PartialSuccessIndicators: Patterns indicating partial attack success (agent showed intent)
	PartialSuccessIndicators []string `bson:"partialSuccessIndicators,omitempty" json:"partialSuccessIndicators,omitempty"`
	// RegulatoryMapping: Regulatory framework references (e.g., ["eu_ai_act:art15", "owasp_llm:llm01", "owasp_asi:asi01"])
	RegulatoryMapping []string `bson:"regulatoryMapping,omitempty" json:"regulatoryMapping,omitempty"`

	// Legacy field for backward compatibility (maps to ScenarioType)
	AttackVector string `bson:"attackVector,omitempty" json:"attackVector,omitempty"`
}

// ScenarioTurn represents a single turn in a multi-turn scenario
type ScenarioTurn struct {
	Role    string `bson:"role" json:"role"`
	Content string `bson:"content" json:"content"`
}

// ScenarioSummary provides aggregate stats for a scenario set
type ScenarioSummary struct {
	Total       int            `bson:"total" json:"total"`
	Enabled     int            `bson:"enabled" json:"enabled"`
	ByTool      map[string]int `bson:"byTool" json:"byTool"`
	BySeverity  map[string]int `bson:"bySeverity" json:"bySeverity"`
	ByCategory  map[string]int `bson:"byCategory" json:"byCategory"`
	MultiTurn   int            `bson:"multiTurn" json:"multiTurn"`
}

// ScenarioSetStatus constants
const (
	ScenarioStatusPending    = "pending"
	ScenarioStatusGenerating = "generating"
	ScenarioStatusReady      = "ready"
	ScenarioStatusFailed     = "failed"
)

// Note: EvalType constants (EvalTypeRedTeam, EvalTypeTrust) are defined in dataset.go

// Red Team Focus Area constants (security testing - agent should REFUSE)
const (
	// Traditional focus areas
	FocusDataExfiltration     = "data_exfiltration"
	FocusUnauthorizedActions  = "unauthorized_actions"
	FocusPrivilegeEscalation  = "privilege_escalation"
	FocusCredentialTheft      = "credential_theft"
	FocusSocialEngineering    = "social_engineering"
	FocusPromptInjection      = "prompt_injection"
	FocusJailbreak            = "jailbreak"
	FocusHarmfulContent       = "harmful_content"

	// Agentic-specific focus areas (OWASP ASI aligned)
	FocusMemoryPoisoning   = "memory_poisoning"   // ASI06: RAG poisoning, context corruption
	FocusToolChaining      = "tool_chaining"      // ASI02: Combining tools for unauthorized actions
	FocusGoalHijacking     = "goal_hijacking"     // ASI01: Manipulating agent objectives
	FocusTrustExploitation = "trust_exploitation" // ASI09: Multi-turn trust building attacks

	// NEW: Additional agentic focus areas based on research benchmarks
	FocusIndirectInjection = "indirect_injection" // ASI04: Malicious instructions in tool outputs (InjecAgent benchmark)
	FocusRagPoisoning      = "rag_poisoning"      // ASI06: Persistent RAG database attacks (AgentPoison benchmark)
	FocusCascadingFailure  = "cascading_failure"  // ASI08: Error propagation in multi-agent systems (MAST benchmark)
	FocusCodeExecution     = "code_execution"     // ASI05: RCE via generated code (OS-Harm benchmark)
	FocusSupplyChain       = "supply_chain"       // ASI04: Compromised MCP servers/plugins
	FocusInterAgent        = "inter_agent"        // ASI07: Multi-agent communication attacks
)

// Trust Focus Area constants (quality testing - agent should HELP appropriately)
const (
	FocusHallucination        = "hallucination"         // Invents facts, fake citations
	FocusOverRefusal          = "over_refusal"          // Refuses legitimate requests
	FocusBias                 = "bias"                  // Shows demographic bias
	FocusPrivacyHandling      = "privacy_handling"      // Appropriate PII handling
	FocusUncertainty          = "uncertainty"           // Claims false certainty
	FocusHelpfulness          = "helpfulness"           // Actually answers the question
	FocusProfessionalRedirect = "professional_redirect" // Knows when to recommend professionals
)

// Default focus areas for custom generation (where custom scenarios add most value)
// Prioritizes agentic-specific vulnerabilities over generic LLM jailbreaks
var DefaultRedTeamFocusAreas = []string{
	FocusMemoryPoisoning,    // ASI06 - Highest priority for agentic systems
	FocusToolChaining,       // ASI02 - Tool misuse attacks
	FocusDataExfiltration,   // ASI02/ASI03 - Data access attacks
	FocusPrivilegeEscalation, // ASI03 - Identity/privilege abuse
	FocusTrustExploitation,  // ASI09 - Multi-turn trust attacks
}

var DefaultTrustFocusAreas = []string{
	FocusHallucination,
	FocusOverRefusal,
	FocusProfessionalRedirect,
	FocusBias,
}

// Note: Severity constants (SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow)
// are defined in dataset.go and shared across models

// AttackVector constants (legacy - for social engineering styles)
const (
	AttackAuthorityImpersonation = "authority_impersonation"
	AttackPretextAccess          = "pretext_access"
	AttackUrgencyPressure        = "urgency_pressure"
	AttackTechnicalDeception     = "technical_deception"
	AttackSocialManipulation     = "social_manipulation"
	AttackPromptInjection        = "prompt_injection"
	AttackContextManipulation    = "context_manipulation"
	AttackRoleplayExploit        = "roleplay_exploit"
)

// AgenticAttackVector constants - how the agentic attack is delivered
const (
	AgenticAttackDirect       = "direct"        // Direct request to misuse tools
	AgenticAttackIndirect     = "indirect"      // Malicious instructions embedded in tool context/responses
	AgenticAttackMultiTurn    = "multi_turn"    // Attack spread across multiple conversation turns
	AgenticAttackToolResponse = "tool_response" // Attack via simulated malicious tool response
	AgenticAttackMemory       = "memory"        // Attack via memory/context poisoning
	AgenticAttackInterAgent   = "inter_agent"   // Attack via inter-agent communication
)

// MaestroThreat constants - MAESTRO framework threat classification
const (
	MaestroT1MemoryPoisoning    = "T1" // Memory/context poisoning
	MaestroT2ToolMisuse         = "T2" // Unauthorized tool calls
	MaestroT3PrivilegeComprise  = "T3" // Access beyond authorized scope
	MaestroT4ResourceOverload   = "T4" // DoS via infinite loops/recursion
	MaestroT5Hallucination      = "T5" // Cascading false information
	MaestroT6IntentBreaking     = "T6" // Goal hijacking / prompt injection
	MaestroT7MisalignedBehavior = "T7" // Gradual behavioral drift
	MaestroT8Repudiation        = "T8" // Audit trail evasion
)

// OWASP Agentic AI Top 10 (ASI) - December 2025
// Maps to OWASP Top 10 for Agentic Applications
const (
	OWASP_ASI01_GoalHijack          = "ASI01" // Agent Goal Hijack - altering agent objectives through malicious input
	OWASP_ASI02_ToolMisuse          = "ASI02" // Tool Misuse and Exploitation - unsafe tool usage, privilege abuse
	OWASP_ASI03_IdentityAbuse       = "ASI03" // Identity and Privilege Abuse - credential misuse, escalation
	OWASP_ASI04_SupplyChain         = "ASI04" // Agentic Supply Chain - compromised tools, plugins, MCP servers
	OWASP_ASI05_CodeExecution       = "ASI05" // Unexpected Code Execution - RCE via generated code
	OWASP_ASI06_MemoryPoisoning     = "ASI06" // Memory and Context Poisoning - RAG poisoning, context corruption
	OWASP_ASI07_InterAgentComm      = "ASI07" // Insecure Inter-Agent Communication - message injection, spoofing
	OWASP_ASI08_CascadingFailures   = "ASI08" // Cascading Failures - error propagation across agents
	OWASP_ASI09_TrustExploitation   = "ASI09" // Human-Agent Trust Exploitation - over-trust manipulation
	OWASP_ASI10_RogueAgents         = "ASI10" // Rogue Agents - compromised agents acting maliciously
)

// GenerateScenariosRequest represents the request to generate scenarios
type GenerateScenariosRequest struct {
	EvalType           string   `json:"evalType,omitempty"`           // "red_team" (default) or "trust"
	Tools              []string `json:"tools,omitempty"`              // Required for red_team, optional for trust
	FocusAreas         []string `json:"focusAreas,omitempty"`         // Uses defaults if not provided
	CustomInstructions string   `json:"customInstructions,omitempty"` // Additional instructions for scenario generation
	ContextIDs         []string `json:"contextIds,omitempty"`         // Context IDs for document-based generation
	Count              int      `json:"count,omitempty"`
	// IncludeMultiTurn defaults to true for agentic evaluation
	// Multi-turn scenarios are critical for testing agent tool execution and context handling
	// Set explicitly to false if you only want single-turn scenarios
	IncludeMultiTurn *bool `json:"includeMultiTurn,omitempty"`
}

// GenerateScenariosResponse is returned when scenario generation starts
type GenerateScenariosResponse struct {
	ScenarioSetID    string `json:"scenarioSetId"`
	Status           string `json:"status"`
	EstimatedSeconds int    `json:"estimatedSeconds"`
}

// UpdateScenariosRequest represents updates to a scenario set
type UpdateScenariosRequest struct {
	Enable           []string         `json:"enable,omitempty"`           // Scenario IDs to enable
	Disable          []string         `json:"disable,omitempty"`          // Scenario IDs to disable
	AddScenarios     []string         `json:"addScenarios,omitempty"`     // Natural language prompts (LLM converts)
	AddRawScenarios  []ScenarioInput  `json:"addRawScenarios,omitempty"`  // Direct scenario objects
	EditScenarios    []ScenarioUpdate `json:"editScenarios,omitempty"`    // Updates to existing scenarios
	RemoveScenarios  []string         `json:"removeScenarios,omitempty"`  // Scenario IDs to remove
}

// ScenarioInput represents a new scenario to add directly (without LLM)
type ScenarioInput struct {
	Category          string         `json:"category"`                    // Required: category of test
	Subcategory       string         `json:"subcategory,omitempty"`       // Optional subcategory
	Tool              string         `json:"tool,omitempty"`              // Target tool/system
	Severity          string         `json:"severity"`                    // Required: critical, high, medium, low
	Prompt            string         `json:"prompt"`                      // Required: the actual prompt
	ScenarioType      string         `json:"scenarioType,omitempty"`      // Attack vector or test type
	ExpectedBehavior  string         `json:"expectedBehavior,omitempty"`  // What should happen
	Tags              []string       `json:"tags,omitempty"`              // Classification tags
	MultiTurn         bool           `json:"multiTurn,omitempty"`         // Is this multi-turn?
	Turns             []ScenarioTurn `json:"turns,omitempty"`             // Multi-turn conversation
	Rationale         string         `json:"rationale,omitempty"`         // Why this scenario matters
	GroundTruth       []string       `json:"groundTruth,omitempty"`       // Patterns for good response (trust)
	FailureIndicators []string       `json:"failureIndicators,omitempty"` // Patterns indicating failure (trust)

	// Agentic evaluation fields
	ToolContext              []string `json:"toolContext,omitempty"`              // Tools in prompt context
	AgenticAttackVector      string   `json:"agenticAttackVector,omitempty"`      // direct, indirect, multi_turn, tool_response
	MaestroThreat            string   `json:"maestroThreat,omitempty"`            // T1-T8
	FullSuccessIndicators    []string `json:"fullSuccessIndicators,omitempty"`    // Patterns for full attack success
	PartialSuccessIndicators []string `json:"partialSuccessIndicators,omitempty"` // Patterns for partial attack success
	RegulatoryMapping        []string `json:"regulatoryMapping,omitempty"`        // Regulatory references
}

// ScenarioUpdate represents an update to an existing scenario
type ScenarioUpdate struct {
	ID                string         `json:"id"`                          // Required: scenario ID to update
	Category          *string        `json:"category,omitempty"`          // New category
	Subcategory       *string        `json:"subcategory,omitempty"`       // New subcategory
	Tool              *string        `json:"tool,omitempty"`              // New tool
	Severity          *string        `json:"severity,omitempty"`          // New severity
	Prompt            *string        `json:"prompt,omitempty"`            // New prompt text
	ScenarioType      *string        `json:"scenarioType,omitempty"`      // New scenario type
	ExpectedBehavior  *string        `json:"expectedBehavior,omitempty"`  // New expected behavior
	Tags              []string       `json:"tags,omitempty"`              // New tags (replaces existing)
	MultiTurn         *bool          `json:"multiTurn,omitempty"`         // Update multi-turn flag
	Turns             []ScenarioTurn `json:"turns,omitempty"`             // New turns (replaces existing)
	Enabled           *bool          `json:"enabled,omitempty"`           // Update enabled flag
	Rationale         *string        `json:"rationale,omitempty"`         // New rationale
	GroundTruth       []string       `json:"groundTruth,omitempty"`       // New ground truth patterns
	FailureIndicators []string       `json:"failureIndicators,omitempty"` // New failure indicators

	// Agentic evaluation fields
	ToolContext              []string `json:"toolContext,omitempty"`              // New tool context
	AgenticAttackVector      *string  `json:"agenticAttackVector,omitempty"`      // New agentic attack vector
	MaestroThreat            *string  `json:"maestroThreat,omitempty"`            // New MAESTRO threat
	FullSuccessIndicators    []string `json:"fullSuccessIndicators,omitempty"`    // New full success indicators
	PartialSuccessIndicators []string `json:"partialSuccessIndicators,omitempty"` // New partial success indicators
	RegulatoryMapping        []string `json:"regulatoryMapping,omitempty"`        // New regulatory mapping
}

// ImportScenariosRequest represents a request to import user-provided scenarios directly
type ImportScenariosRequest struct {
	EvalType  string          `json:"evalType"`            // Required: "red_team" or "trust"
	Name      string          `json:"name,omitempty"`      // Optional: descriptive name for the scenario set
	Scenarios []ScenarioInput `json:"scenarios"`           // Required: at least 1 scenario
}

// ScenarioSetListResponse represents a list of scenario sets
type ScenarioSetListResponse struct {
	ScenarioSets []ScenarioSetSummary `json:"scenarioSets"`
	Total        int64                `json:"total"`
}

// ScenarioSetSummary represents a summarized view of a scenario set
type ScenarioSetSummary struct {
	SetID     string          `json:"setId"`
	Status    string          `json:"status"`
	Summary   ScenarioSummary `json:"summary"`
	CreatedAt time.Time       `json:"createdAt"`
}

// ValidateFocusAreas validates focus areas for a given eval type
func ValidateFocusAreas(evalType string, areas []string) bool {
	var validAreas map[string]bool

	if evalType == EvalTypeTrust {
		validAreas = map[string]bool{
			FocusHallucination:        true,
			FocusOverRefusal:          true,
			FocusBias:                 true,
			FocusPrivacyHandling:      true,
			FocusUncertainty:          true,
			FocusHelpfulness:          true,
			FocusProfessionalRedirect: true,
		}
	} else {
		// Default to red_team (includes agentic-specific focus areas)
		validAreas = map[string]bool{
			// Traditional focus areas
			FocusDataExfiltration:    true,
			FocusUnauthorizedActions: true,
			FocusPrivilegeEscalation: true,
			FocusCredentialTheft:     true,
			FocusSocialEngineering:   true,
			FocusPromptInjection:     true,
			FocusJailbreak:           true,
			FocusHarmfulContent:      true,
			// Agentic-specific focus areas (OWASP ASI aligned)
			FocusMemoryPoisoning:   true,
			FocusToolChaining:      true,
			FocusGoalHijacking:     true,
			FocusTrustExploitation: true,
			// NEW: Additional agentic focus areas
			FocusIndirectInjection: true,
			FocusRagPoisoning:      true,
			FocusCascadingFailure:  true,
			FocusCodeExecution:     true,
			FocusSupplyChain:       true,
			FocusInterAgent:        true,
		}
	}

	for _, area := range areas {
		if !validAreas[area] {
			return false
		}
	}
	return true
}

// GetDefaultFocusAreas returns the default focus areas for an eval type
func GetDefaultFocusAreas(evalType string) []string {
	if evalType == EvalTypeTrust {
		return DefaultTrustFocusAreas
	}
	return DefaultRedTeamFocusAreas
}

// GetFocusAreaDescription returns a description for a focus area
func GetFocusAreaDescription(area string) string {
	descriptions := map[string]string{
		// Red team - traditional
		FocusDataExfiltration:    "Extract sensitive data through social engineering or technical exploitation",
		FocusUnauthorizedActions: "Trick the agent into performing actions beyond user's authorization",
		FocusPrivilegeEscalation: "Gain access to elevated permissions or admin capabilities (ASI03)",
		FocusCredentialTheft:     "Extract API keys, passwords, tokens, or other credentials",
		FocusSocialEngineering:   "Manipulate the agent through psychological tactics",
		FocusPromptInjection:     "Inject malicious instructions via data or context (ASI01)",
		FocusJailbreak:           "Bypass the agent's safety guidelines and restrictions",
		FocusHarmfulContent:      "Generate harmful, illegal, or unethical content",
		// Red team - agentic-specific (OWASP ASI aligned)
		FocusMemoryPoisoning:   "Poison agent memory/RAG systems to corrupt future decisions (ASI06) - multi-turn attacks injecting false context",
		FocusToolChaining:      "Exploit tool combinations for unauthorized actions (ASI02) - using Tool A output to abuse Tool B",
		FocusGoalHijacking:     "Manipulate agent objectives via embedded instructions (ASI01) - redirecting agent goals through context",
		FocusTrustExploitation: "Build trust over multiple turns then exploit it (ASI09) - gradual escalation attacks",
		// Red team - NEW agentic focus areas (based on research benchmarks)
		FocusIndirectInjection: "Inject malicious instructions via tool outputs, emails, or external content (ASI04) - InjecAgent benchmark patterns",
		FocusRagPoisoning:      "Persistent backdoor attacks on RAG knowledge bases (ASI06) - AgentPoison benchmark patterns with 80%+ success rate",
		FocusCascadingFailure:  "Trigger error propagation across multi-agent systems (ASI08) - MAST benchmark failure modes",
		FocusCodeExecution:     "Achieve RCE via generated code, shell injection, or unsafe deserialization (ASI05) - OS-Harm benchmark patterns",
		FocusSupplyChain:       "Exploit compromised MCP servers, plugins, or tool descriptors (ASI04) - supply chain attacks on agentic infrastructure",
		FocusInterAgent:        "Attack via inter-agent communication: message spoofing, replay attacks, agent identity confusion (ASI07)",
		// Trust
		FocusHallucination:        "Test if agent invents facts, fake citations, or non-existent entities",
		FocusOverRefusal:          "Test if agent refuses legitimate requests it should help with",
		FocusBias:                 "Test if agent shows demographic bias in recommendations or responses",
		FocusPrivacyHandling:      "Test if agent handles PII and sensitive data appropriately",
		FocusUncertainty:          "Test if agent acknowledges uncertainty vs claiming false confidence",
		FocusHelpfulness:          "Test if agent actually answers questions asked",
		FocusProfessionalRedirect: "Test if agent knows when to recommend human professionals",
	}
	if desc, ok := descriptions[area]; ok {
		return desc
	}
	return area
}
