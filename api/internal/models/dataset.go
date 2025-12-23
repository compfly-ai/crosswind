package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Dataset represents a collection of evaluation prompts
type Dataset struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	DatasetID     string             `bson:"datasetId" json:"datasetId"`
	Version       string             `bson:"version" json:"version"`
	Name          string             `bson:"name" json:"name"`
	Description   string             `bson:"description" json:"description"`
	Category      string             `bson:"category" json:"category"`
	Subcategories []string           `bson:"subcategories,omitempty" json:"subcategories,omitempty"`
	Source        DatasetSource      `bson:"source" json:"source"`
	License       DatasetLicense     `bson:"license" json:"license"`
	Metadata      DatasetMetadata    `bson:"metadata" json:"metadata"`
	// EvalType determines whether this is a security (red_team) or safety/quality (trust) dataset
	// "red_team" - adversarial prompts to test security vulnerabilities (default)
	// "trust" - legitimate requests to test appropriate behavior, bias, accuracy
	EvalType string `bson:"evalType" json:"evalType"`
	// JudgmentMode determines how responses are judged
	// "safety" - did agent refuse harmful content? (default for red_team)
	// "quality" - did agent respond appropriately to legitimate request? (default for trust)
	JudgmentMode string `bson:"judgmentMode" json:"judgmentMode"`
	// Visibility controls what can be exposed in API results:
	// "full" - prompt/response content can be shown to users
	// "redacted" - only metadata (category, severity, pass/fail) shown, content hidden
	// "scores_only" - only aggregate scores, no individual results
	Visibility string  `bson:"visibility" json:"visibility"`
	IsShared   bool    `bson:"isShared" json:"isShared"`
	IsActive   bool    `bson:"isActive" json:"isActive"`
	CreatedAt  time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time `bson:"updatedAt" json:"updatedAt"`
}

// DatasetSource holds information about the dataset's origin
type DatasetSource struct {
	Name         string `bson:"name" json:"name"`
	URL          string `bson:"url" json:"url"`
	Paper        string `bson:"paper,omitempty" json:"paper,omitempty"`
	Contributors string `bson:"contributors" json:"contributors"`
}

// DatasetLicense holds licensing information
type DatasetLicense struct {
	Type         string  `bson:"type" json:"type"`
	URL          string  `bson:"url,omitempty" json:"url,omitempty"`
	Attribution  string  `bson:"attribution,omitempty" json:"attribution,omitempty"`
	Restrictions *string `bson:"restrictions,omitempty" json:"restrictions,omitempty"`
}

// DatasetMetadata holds additional dataset metadata
type DatasetMetadata struct {
	PromptCount          int      `bson:"promptCount" json:"promptCount"`
	IsMultiturn          bool     `bson:"isMultiturn" json:"isMultiturn"`
	Languages            []string `bson:"languages" json:"languages"`
	HarmCategories       []string `bson:"harmCategories" json:"harmCategories"`
	RegulatoryAlignments []string `bson:"regulatoryAlignments" json:"regulatoryAlignments"`
	AvgPromptLength      int      `bson:"avgPromptLength,omitempty" json:"avgPromptLength,omitempty"`
	HasBenignControls    bool     `bson:"hasBenignControls" json:"hasBenignControls"`
	BenignControlCount   int      `bson:"benignControlCount,omitempty" json:"benignControlCount,omitempty"`
}

// DatasetPrompt represents a single prompt in a dataset
type DatasetPrompt struct {
	ID                  primitive.ObjectID     `bson:"_id,omitempty" json:"-"`
	DatasetID           string                 `bson:"datasetId" json:"datasetId"`
	Version             string                 `bson:"version" json:"version"`
	PromptID            string                 `bson:"promptId" json:"promptId"`
	Content             interface{}            `bson:"content" json:"content"` // string or []ConversationTurn
	IsMultiturn         bool                   `bson:"isMultiturn" json:"isMultiturn"`
	TurnCount           int                    `bson:"turnCount" json:"turnCount"`
	ExpectedBehavior    string                 `bson:"expectedBehavior" json:"expectedBehavior"`
	GroundTruthPatterns []string               `bson:"groundTruthPatterns" json:"groundTruthPatterns"`
	// FailureIndicators - patterns that indicate a failure (for trust evals)
	// e.g., ["assumes male", "assumes female", "stereotypes"] for bias detection
	FailureIndicators []string               `bson:"failureIndicators,omitempty" json:"failureIndicators,omitempty"`
	AttackType        string                 `bson:"attackType" json:"attackType"`
	Severity          string                 `bson:"severity" json:"severity"`
	HarmCategories    []string               `bson:"harmCategories" json:"harmCategories"`
	RegulatoryFlags   []string               `bson:"regulatoryFlags" json:"regulatoryFlags"`
	Metadata          map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`

	// Agentic evaluation fields
	// ToolContext: Tools to mention in prompt context for agentic scenarios
	ToolContext []string `bson:"toolContext,omitempty" json:"toolContext,omitempty"`
	// AgenticAttackVector: Type of agentic attack (direct, indirect, multi_turn, tool_response)
	AgenticAttackVector string `bson:"agenticAttackVector,omitempty" json:"agenticAttackVector,omitempty"`
	// MaestroThreat: MAESTRO framework threat ID (T1-T8)
	MaestroThreat string `bson:"maestroThreat,omitempty" json:"maestroThreat,omitempty"`
	// FullSuccessIndicators: Patterns indicating full attack success
	FullSuccessIndicators []string `bson:"fullSuccessIndicators,omitempty" json:"fullSuccessIndicators,omitempty"`
	// PartialSuccessIndicators: Patterns indicating partial attack success
	PartialSuccessIndicators []string `bson:"partialSuccessIndicators,omitempty" json:"partialSuccessIndicators,omitempty"`
	// RegulatoryMapping: Regulatory framework references (e.g., ["eu_ai_act:art15", "owasp_llm:llm01"])
	RegulatoryMapping []string `bson:"regulatoryMapping,omitempty" json:"regulatoryMapping,omitempty"`
}

// ConversationTurn represents a single turn in a multi-turn conversation
type ConversationTurn struct {
	Role    string `bson:"role" json:"role"`
	Content string `bson:"content" json:"content"`
}

// Category constants
const (
	// Red Team (Security) categories - Static/Traditional
	CategoryJailbreak           = "jailbreak"
	CategoryPromptInjection     = "prompt_injection"
	CategorySafety              = "safety"
	CategoryMultiturn           = "multiturn"
	CategoryMemory              = "memory"
	CategoryToxicity            = "toxicity"
	CategoryRedteam             = "redteam"
	CategoryEthics              = "ethics"
	CategoryPolicy              = "policy"
	CategorySystemPromptExtract = "system_prompt_extract" // Attempts to leak system prompt

	// Red Team (Security) categories - Agentic
	CategoryAgentic           = "agentic"            // General agentic security
	CategoryToolMisuse        = "tool_misuse"        // Unauthorized tool calls or parameter abuse
	CategoryToolAbuse         = "tool_abuse"         // Legacy alias for tool_misuse
	CategoryIndirectInjection = "indirect_injection" // Malicious content in retrieved context/tool responses
	CategoryDataExfiltration  = "data_exfiltration"  // Extract sensitive data via tools
	CategoryPrivilegeEscalation = "privilege_escalation" // Gain elevated access beyond authorization
	CategoryMemoryPoisoning   = "memory_poisoning"   // Corrupt agent context over multi-turn
	CategoryGoalHijacking     = "goal_hijacking"     // Redirect agent from original objective

	// Trust (Safety/Quality) categories
	CategoryBiasDetection   = "bias_detection"   // Prompts that might reveal demographic bias
	CategoryOverRefusal     = "over_refusal"     // Legitimate requests agents wrongly refuse
	CategoryHallucination   = "hallucination"    // Questions with verifiable answers
	CategorySensitiveTopics = "sensitive_topics" // Medical, legal, financial advice boundaries
	CategoryPIIHandling     = "pii_handling"     // Appropriate handling of user PII
	CategoryTransparency    = "transparency"     // Does agent identify as AI when asked?
	CategoryAccuracy        = "accuracy"         // Factual correctness

	// General
	CategoryCustom = "custom"
)

// AgenticCategories is the list of agentic-specific red_team categories
var AgenticCategories = []string{
	CategoryToolMisuse,
	CategoryIndirectInjection,
	CategoryDataExfiltration,
	CategoryPrivilegeEscalation,
	CategoryMemoryPoisoning,
	CategoryGoalHijacking,
}

// ExpectedBehavior constants
const (
	// Red team behaviors (agent should NOT comply)
	ExpectedBehaviorRefuse           = "refuse"           // Agent should refuse the request
	ExpectedBehaviorContextDependent = "context_dependent" // Depends on context

	// Trust behaviors (agent SHOULD help appropriately)
	ExpectedBehaviorComply            = "comply"              // Agent should help with the request
	ExpectedBehaviorComplyWithCaveats = "comply_with_caveats" // Agent should help but with appropriate disclaimers
	ExpectedBehaviorRedirect          = "redirect"            // Agent should redirect to professionals (medical, legal, etc.)

	// Legacy alias for backward compatibility
	ExpectedBehaviorComplySafe = "comply_safe"
)

// EvalType constants
const (
	EvalTypeRedTeam  = "red_team" // Security evaluation - adversarial prompts
	EvalTypeTrust    = "trust"    // Safety/quality evaluation - legitimate requests
	EvalTypeGeneral  = "general"  // Combined evaluation - both security and quality
)

// JudgmentMode constants
const (
	JudgmentModeSafety  = "safety"  // Did agent refuse harmful content? (for red_team)
	JudgmentModeQuality = "quality" // Did agent respond appropriately? (for trust)
)

// Severity constants
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Visibility constants - controls what can be exposed in API results
const (
	// VisibilityFull allows full prompt/response content to be shown to users
	VisibilityFull = "full"
	// VisibilityRedacted shows only metadata (category, severity, pass/fail), content is hidden
	VisibilityRedacted = "redacted"
	// VisibilityScoresOnly shows only aggregate scores, no individual results
	VisibilityScoresOnly = "scores_only"
)

// DatasetListResponse represents a paginated list of datasets
type DatasetListResponse struct {
	Datasets   []DatasetSummary `json:"datasets"`
	Total      int64            `json:"total"`
	Categories []string         `json:"categories"` // All unique categories across datasets
	EvalTypes  []string         `json:"evalTypes"`  // All unique eval types (red_team, trust)
}

// DatasetSummary represents a summarized view of a dataset
type DatasetSummary struct {
	DatasetID    string `json:"datasetId"`
	Version      string `json:"version"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	EvalType     string `json:"evalType"`     // "red_team" or "trust"
	JudgmentMode string `json:"judgmentMode"` // "safety" or "quality"
	PromptCount  int    `json:"promptCount"`
	IsMultiturn  bool   `json:"isMultiturn"`
	License      string `json:"license"`
	Source       string `json:"source"`
}

// DatasetPromptsResponse represents a paginated list of prompts
type DatasetPromptsResponse struct {
	Prompts []PromptSummary `json:"prompts"`
	Total   int64           `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
}

// PromptSummary represents a summarized view of a prompt
type PromptSummary struct {
	PromptID         string `json:"promptId"`
	Content          string `json:"content"`
	Category         string `json:"category"`
	AttackType       string `json:"attackType"`
	Severity         string `json:"severity"`
	ExpectedBehavior string `json:"expectedBehavior"`
}
