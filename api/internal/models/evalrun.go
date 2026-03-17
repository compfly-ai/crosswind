package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EvalRun represents an evaluation run against an agent
type EvalRun struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	RunID   string             `bson:"runId" json:"runId"`
	AgentID string             `bson:"agentId" json:"agentId"`
	Mode    string             `bson:"mode" json:"mode"`
	// EvalType: "red_team" for security testing, "trust" for quality/bias testing
	EvalType             string                 `bson:"evalType" json:"evalType"`
	Status               string                 `bson:"status" json:"status"`
	Config               EvalRunConfig          `bson:"config" json:"config"`
	DatasetsUsed         []DatasetUsed          `bson:"datasetsUsed" json:"datasetsUsed"`
	ScenarioSetsUsed     []ScenarioSetUsed      `bson:"scenarioSetsUsed,omitempty" json:"scenarioSetsUsed,omitempty"`
	Progress             EvalProgress           `bson:"progress" json:"progress"`
	SummaryScores        *SummaryScores         `bson:"summaryScores,omitempty" json:"summaryScores,omitempty"`
	RegulatoryCompliance map[string]*Compliance `bson:"regulatoryCompliance,omitempty" json:"regulatoryCompliance,omitempty"`
	// ThreatAnalysis: Aggregated threat analysis for red_team evaluations (OWASP ASI focused)
	ThreatAnalysis *ThreatAnalysis `bson:"threatAnalysis,omitempty" json:"threatAnalysis,omitempty"`
	// TrustAnalysis: Aggregated quality analysis for trust evaluations
	TrustAnalysis   *TrustAnalysis   `bson:"trustAnalysis,omitempty" json:"trustAnalysis,omitempty"`
	Recommendations []Recommendation `bson:"recommendations,omitempty" json:"recommendations,omitempty"`
	// ReportPath: Path to the generated HTML report in file storage
	ReportPath           string                 `bson:"reportPath,omitempty" json:"reportPath,omitempty"`
	Errors               []EvalError            `bson:"errors,omitempty" json:"errors,omitempty"`
	StartedAt            *time.Time             `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	CompletedAt          *time.Time             `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
	CreatedAt            time.Time              `bson:"createdAt" json:"createdAt"`
	UpdatedAt            time.Time              `bson:"updatedAt" json:"updatedAt"`
}

// EvalRunConfig holds configuration for an evaluation run
type EvalRunConfig struct {
	RequestsPerMinute    int  `bson:"requestsPerMinute" json:"requestsPerMinute"`
	ConcurrentSessions   int  `bson:"concurrentSessions" json:"concurrentSessions"`
	TimeoutSeconds       int  `bson:"timeoutSeconds" json:"timeoutSeconds"`
	ResetSessionOnError  bool `bson:"resetSessionOnError" json:"resetSessionOnError"`
	MaxConsecutiveErrors int  `bson:"maxConsecutiveErrors" json:"maxConsecutiveErrors"`
}

// DatasetUsed represents a dataset used in an evaluation run
type DatasetUsed struct {
	DatasetID   string   `bson:"datasetId" json:"datasetId"`
	Version     string   `bson:"version" json:"version"`
	PromptCount int      `bson:"promptCount" json:"promptCount"`
	Categories  []string `bson:"categories" json:"categories"`
}

// ScenarioSetUsed represents a generated scenario set used in an evaluation run
type ScenarioSetUsed struct {
	SetID         string   `bson:"setId" json:"setId"`
	ScenarioCount int      `bson:"scenarioCount" json:"scenarioCount"`
	EvalType      string   `bson:"evalType" json:"evalType"`
	FocusAreas    []string `bson:"focusAreas" json:"focusAreas"`
}

// EvalProgress tracks the progress of an evaluation run
type EvalProgress struct {
	TotalPrompts       int       `bson:"totalPrompts" json:"totalPrompts"`
	CompletedPrompts   int       `bson:"completedPrompts" json:"completedPrompts"`
	PassedPrompts      int       `bson:"passedPrompts" json:"passedPrompts"`
	FailedPrompts      int       `bson:"failedPrompts" json:"failedPrompts"`
	UncertainPrompts   int       `bson:"uncertainPrompts" json:"uncertainPrompts"`
	ErrorPrompts       int       `bson:"errorPrompts" json:"errorPrompts"`
	CurrentDataset     string    `bson:"currentDataset" json:"currentDataset"`
	CurrentSession     string    `bson:"currentSession" json:"currentSession"`
	LastUpdated        time.Time `bson:"lastUpdated" json:"lastUpdated"`
	CompletedPromptIds   []string       `bson:"completedPromptIds,omitempty" json:"-"`
	CheckpointCounters   map[string]int `bson:"checkpointCounters,omitempty" json:"-"`
}

// SummaryScores holds aggregated scores for an evaluation
type SummaryScores struct {
	Overall    float64            `bson:"overall" json:"overall"`
	ByCategory map[string]float64 `bson:"byCategory" json:"byCategory"`
	BySeverity map[string]float64 `bson:"bySeverity" json:"bySeverity"`

	// ASR (Attack Success Rate) metrics for red_team evaluations
	ASR *ASRMetrics `bson:"asr,omitempty" json:"asr,omitempty"`

	// RefusalAnalysis: Aggregate analysis of refusal quality (Phase 1 improvement)
	RefusalAnalysis *RefusalAnalysis `bson:"refusalAnalysis,omitempty" json:"refusalAnalysis,omitempty"`
}

// ASRMetrics holds Attack Success Rate metrics
// ASR = (Successful Attacks / Total Attack Attempts) × 100
type ASRMetrics struct {
	// Full: Percentage of attacks where objectives were completely achieved
	Full float64 `bson:"full" json:"full"`
	// Partial: Percentage of attacks where objectives were partially achieved
	Partial float64 `bson:"partial" json:"partial"`
	// Blocked: Percentage of attacks that were completely blocked
	Blocked float64 `bson:"blocked" json:"blocked"`
	// ScoredCount: Number of prompts that received a valid score (denominator for percentages)
	ScoredCount int `bson:"scoredCount" json:"scoredCount"`
	// UnscoredCount: Number of prompts that could not be scored (errors, timeouts)
	UnscoredCount int `bson:"unscoredCount" json:"unscoredCount"`
}

// RefusalAnalysis provides aggregate analysis of how the agent refused harmful requests
// This measures the QUALITY of refusals, not just pass/fail (Phase 1 improvement)
type RefusalAnalysis struct {
	// Distribution: Count of each refusal quality type (flat, all results)
	Distribution map[string]int `bson:"distribution" json:"distribution"`
	// AverageQualityScore: Average refusal quality score (0.0-1.0, higher is better)
	AverageQualityScore *float64 `bson:"averageQualityScore,omitempty" json:"averageQualityScore,omitempty"`
	// DominantPattern: Most common refusal style
	DominantPattern string `bson:"dominantPattern" json:"dominantPattern"`
	// ConcerningPatterns: Categories where agent showed weak refusals
	ConcerningPatterns []ConcerningPattern `bson:"concerningPatterns,omitempty" json:"concerningPatterns,omitempty"`
	// Interpretation: Human-readable summary of refusal quality
	Interpretation string `bson:"interpretation" json:"interpretation"`
	// TotalEvaluated: Total number of results analyzed
	TotalEvaluated int `bson:"totalEvaluated" json:"totalEvaluated"`
	// ScoredCount: Number of results that had refusal quality scores
	ScoredCount int `bson:"scoredCount" json:"scoredCount"`
	// Blocked: Breakdown of attacks that were blocked (PASS results)
	Blocked *DefenseBreakdown `bson:"blocked,omitempty" json:"blocked,omitempty"`
	// Compromised: Breakdown of attacks that succeeded (FAIL results)
	Compromised *DefenseBreakdown `bson:"compromised,omitempty" json:"compromised,omitempty"`
}

// DefenseBreakdown provides blocked/compromised counts with defense quality
type DefenseBreakdown struct {
	Total         int            `bson:"total" json:"total"`
	StrongDefense int            `bson:"strongDefense,omitempty" json:"strongDefense,omitempty"`
	WeakDefense   int            `bson:"weakDefense,omitempty" json:"weakDefense,omitempty"`
	Distribution  map[string]int `bson:"distribution" json:"distribution"`
}

// ConcerningPattern represents a category with weak refusal behavior
type ConcerningPattern struct {
	// Backward compat fields
	Pattern  string   `bson:"pattern" json:"pattern"`
	Count    int      `bson:"count" json:"count"`
	Category string   `bson:"category" json:"category"`
	Severity string   `bson:"severity" json:"severity"`
	AvgScore *float64 `bson:"avgScore,omitempty" json:"avgScore,omitempty"`
	// New per-category defense breakdown
	Total             int            `bson:"total,omitempty" json:"total,omitempty"`
	Compromised       int            `bson:"compromised,omitempty" json:"compromised,omitempty"`
	WeakDefense       int            `bson:"weakDefense,omitempty" json:"weakDefense,omitempty"`
	StrongDefense     int            `bson:"strongDefense,omitempty" json:"strongDefense,omitempty"`
	AvgRefusalScore   *float64       `bson:"avgRefusalScore,omitempty" json:"avgRefusalScore,omitempty"`
	FailureSeverities map[string]int `bson:"failureSeverities,omitempty" json:"failureSeverities,omitempty"`
	ConcernLevel      string         `bson:"concernLevel,omitempty" json:"concernLevel,omitempty"`
}

// RefusalQuality constants for classifying refusal behavior
const (
	RefusalQualityHardRefuse         = "hard_refuse"          // Clear refusal with reasoning - best
	RefusalQualitySoftRefuseRedirect = "soft_refuse_redirect" // Polite decline + alternative
	RefusalQualityPartialCompliance  = "partial_compliance"   // Refuses but leaks some info
	RefusalQualityEvasion            = "evasion"              // Avoids without clear refusal
	RefusalQualityCompliance         = "compliance"           // Fully complies - worst
)

// ThreatAnalysis provides aggregated threat analysis for red_team evaluations
// Maps results to OWASP Agentic AI Top 10 (December 2025) and MAESTRO frameworks
type ThreatAnalysis struct {
	// Framework: The threat framework used (e.g., "OWASP Agentic AI Top 10 (December 2025)")
	Framework string `bson:"framework,omitempty" json:"framework,omitempty"`
	// TopVulnerabilities: Ranked list of most exploitable threats
	TopVulnerabilities []ThreatVulnerability `bson:"topVulnerabilities" json:"topVulnerabilities"`
	// ByOwaspAsi: Breakdown by OWASP Agentic AI Top 10 threat (ASI01-ASI10)
	ByOwaspAsi map[string]*ThreatStats `bson:"byOwaspAsi" json:"byOwaspAsi"`
	// Coverage: Summary of which threats were tested vs not tested
	Coverage *ThreatCoverage `bson:"coverage,omitempty" json:"coverage,omitempty"`
	// CoverageInterpretation: Human-readable summary of coverage quality
	CoverageInterpretation string `bson:"coverageInterpretation,omitempty" json:"coverageInterpretation,omitempty"`
	// ByMaestro: Breakdown by MAESTRO framework threat (T1-T8) - secondary
	ByMaestro map[string]*ThreatStats `bson:"byMaestro,omitempty" json:"byMaestro,omitempty"`
	// ByAttackVector: Breakdown by attack vector type
	ByAttackVector map[string]*ThreatStats `bson:"byAttackVector" json:"byAttackVector"`
	// MultiTurnAnalysis: Analysis of multi-turn vs single-turn attack success
	MultiTurnAnalysis *MultiTurnAnalysis `bson:"multiTurnAnalysis,omitempty" json:"multiTurnAnalysis,omitempty"`
}

// ThreatCoverage summarizes which OWASP threats were tested
type ThreatCoverage struct {
	// ThreatsTested: List of threat IDs that were tested (e.g., ["ASI01", "ASI02"])
	ThreatsTested []string `bson:"threatsTested" json:"threatsTested"`
	// ThreatsNotTested: List of threat IDs not covered by this eval
	ThreatsNotTested []string `bson:"threatsNotTested" json:"threatsNotTested"`
	// CoveragePercent: Percentage of OWASP Top 10 covered (0-100)
	CoveragePercent float64 `bson:"coveragePercent" json:"coveragePercent"`
	// TotalThreats: Total number of threats in the framework (10)
	TotalThreats int `bson:"totalThreats" json:"totalThreats"`
	// TestedCount: Number of threats tested
	TestedCount int `bson:"testedCount" json:"testedCount"`
}

// ThreatVulnerability represents a ranked vulnerability finding
type ThreatVulnerability struct {
	// ThreatID: OWASP ASI threat ID (e.g., "ASI06")
	ThreatID string `bson:"threatId" json:"threatId"`
	// ThreatName: Human-readable name (e.g., "Memory and Context Poisoning")
	ThreatName string `bson:"threatName" json:"threatName"`
	// SuccessRate: Percentage of attacks that succeeded (full + partial)
	SuccessRate float64 `bson:"successRate" json:"successRate"`
	// FullSuccessRate: Percentage with full attack success
	FullSuccessRate float64 `bson:"fullSuccessRate" json:"fullSuccessRate"`
	// Severity: Highest severity level observed (critical, high, medium, low)
	Severity string `bson:"severity" json:"severity"`
	// TotalAttempts: Number of scenarios tested for this threat
	TotalAttempts int `bson:"totalAttempts" json:"totalAttempts"`
	// SuccessfulAttempts: Number of successful attacks (full + partial)
	SuccessfulAttempts int `bson:"successfulAttempts" json:"successfulAttempts"`
	// Recommendation: Specific recommendation to address this vulnerability
	Recommendation string `bson:"recommendation" json:"recommendation"`
}

// ThreatStats holds statistics for a specific threat category
type ThreatStats struct {
	Total          int     `bson:"total" json:"total"`
	FullSuccess    int     `bson:"fullSuccess" json:"fullSuccess"`
	PartialSuccess int     `bson:"partialSuccess" json:"partialSuccess"`
	Blocked        int     `bson:"blocked" json:"blocked"`
	SuccessRate    float64 `bson:"successRate" json:"successRate"` // (full + partial) / total
	// ThreatName: Human-readable name for display (e.g., "Agent Goal Hijack")
	ThreatName string `bson:"threatName,omitempty" json:"threatName,omitempty"`
}

// MultiTurnAnalysis compares single-turn vs multi-turn attack effectiveness
type MultiTurnAnalysis struct {
	// SingleTurn stats
	SingleTurnTotal       int     `bson:"singleTurnTotal" json:"singleTurnTotal"`
	SingleTurnSuccessRate float64 `bson:"singleTurnSuccessRate" json:"singleTurnSuccessRate"`
	// MultiTurn stats
	MultiTurnTotal       int     `bson:"multiTurnTotal" json:"multiTurnTotal"`
	MultiTurnSuccessRate float64 `bson:"multiTurnSuccessRate" json:"multiTurnSuccessRate"`
	// Insight: Key finding about multi-turn vs single-turn effectiveness
	Insight string `bson:"insight" json:"insight"`
}

// TrustAnalysis provides aggregated quality analysis for trust evaluations
// Maps results to quality dimensions and regulatory frameworks
type TrustAnalysis struct {
	// TopIssues: Ranked list of most significant quality issues
	TopIssues []TrustIssue `bson:"topIssues" json:"topIssues"`
	// ByQualityDimension: Breakdown by quality dimension (hallucination, bias, etc.)
	ByQualityDimension map[string]*QualityStats `bson:"byQualityDimension" json:"byQualityDimension"`
	// RegulatoryMapping: How issues map to regulatory requirements
	RegulatoryMapping *RegulatoryImpact `bson:"regulatoryMapping" json:"regulatoryMapping"`
	// ConsistencyAnalysis: Analysis of cross-turn consistency (for multi-turn)
	ConsistencyAnalysis *ConsistencyAnalysis `bson:"consistencyAnalysis,omitempty" json:"consistencyAnalysis,omitempty"`
}

// TrustIssue represents a ranked quality issue
type TrustIssue struct {
	// Dimension: Quality dimension (hallucination, over_refusal, bias, etc.)
	Dimension string `bson:"dimension" json:"dimension"`
	// DimensionName: Human-readable name
	DimensionName string `bson:"dimensionName" json:"dimensionName"`
	// FailureRate: Percentage of tests that failed
	FailureRate float64 `bson:"failureRate" json:"failureRate"`
	// Severity: Impact severity (critical, high, medium, low)
	Severity string `bson:"severity" json:"severity"`
	// TotalTests: Number of tests for this dimension
	TotalTests int `bson:"totalTests" json:"totalTests"`
	// FailedTests: Number of failed tests
	FailedTests int `bson:"failedTests" json:"failedTests"`
	// RegulatoryImpact: Affected regulations (EU AI Act, NIST, etc.)
	RegulatoryImpact []string `bson:"regulatoryImpact" json:"regulatoryImpact"`
	// Recommendation: Specific recommendation to address this issue
	Recommendation string `bson:"recommendation" json:"recommendation"`
}

// QualityStats holds statistics for a quality dimension
type QualityStats struct {
	Total      int     `bson:"total" json:"total"`
	Passed     int     `bson:"passed" json:"passed"`
	Failed     int     `bson:"failed" json:"failed"`
	Uncertain  int     `bson:"uncertain" json:"uncertain"`
	PassRate   float64 `bson:"passRate" json:"passRate"`
	AvgLatency float64 `bson:"avgLatency" json:"avgLatency"`
}

// RegulatoryImpact maps quality issues to regulatory frameworks
type RegulatoryImpact struct {
	// EuAiAct: EU AI Act compliance issues
	EuAiAct *RegulatoryFrameworkImpact `bson:"euAiAct,omitempty" json:"euAiAct,omitempty"`
	// NistAiRmf: NIST AI Risk Management Framework issues
	NistAiRmf *RegulatoryFrameworkImpact `bson:"nistAiRmf,omitempty" json:"nistAiRmf,omitempty"`
	// OwaspLlm: OWASP LLM Top 10 issues
	OwaspLlm *RegulatoryFrameworkImpact `bson:"owaspLlm,omitempty" json:"owaspLlm,omitempty"`
}

// RegulatoryFrameworkImpact represents impact on a specific regulatory framework
type RegulatoryFrameworkImpact struct {
	// Compliant: Overall compliance status
	Compliant bool `bson:"compliant" json:"compliant"`
	// Score: Compliance score (0-100)
	Score float64 `bson:"score" json:"score"`
	// AffectedArticles: Specific articles/requirements affected
	AffectedArticles []string `bson:"affectedArticles" json:"affectedArticles"`
	// CriticalFindings: High-priority compliance gaps
	CriticalFindings []string `bson:"criticalFindings" json:"criticalFindings"`
}

// ConsistencyAnalysis measures cross-turn consistency in multi-turn tests
type ConsistencyAnalysis struct {
	// TotalMultiTurnTests: Number of multi-turn test sequences
	TotalMultiTurnTests int `bson:"totalMultiTurnTests" json:"totalMultiTurnTests"`
	// ConsistentResponses: Tests where agent maintained consistency
	ConsistentResponses int `bson:"consistentResponses" json:"consistentResponses"`
	// ConsistencyRate: Percentage of consistent multi-turn sequences
	ConsistencyRate float64 `bson:"consistencyRate" json:"consistencyRate"`
	// CommonInconsistencies: Patterns of inconsistency observed
	CommonInconsistencies []string `bson:"commonInconsistencies,omitempty" json:"commonInconsistencies,omitempty"`
}

// AttackSuccess constants for classifying attack outcomes
const (
	AttackSuccessFull    = "full"    // Attack objectives completely achieved
	AttackSuccessPartial = "partial" // Attack objectives partially achieved
	AttackSuccessNone    = "none"    // Attack completely blocked
)

// GetOwaspAsiThreatName returns the human-readable name for an OWASP ASI threat ID
// Names aligned with OWASP Agentic AI Top 10 (December 2025)
// https://genai.owasp.org/resource/agentic-ai-threats-and-mitigations/
func GetOwaspAsiThreatName(threatID string) string {
	names := map[string]string{
		"ASI01": "Agent Goal Hijack",
		"ASI02": "Tool Misuse & Exploitation",
		"ASI03": "Identity & Privilege Abuse",
		"ASI04": "Agentic Supply Chain Vulnerabilities",
		"ASI05": "Unexpected Code Execution (RCE)",
		"ASI06": "Memory & Context Poisoning",
		"ASI07": "Insecure Inter-Agent Communication",
		"ASI08": "Cascading Failures",
		"ASI09": "Human-Agent Trust Exploitation",
		"ASI10": "Rogue Agents",
	}
	if name, ok := names[threatID]; ok {
		return name
	}
	return threatID
}

// GetOwaspAsiRecommendation returns a remediation recommendation for an OWASP ASI threat
func GetOwaspAsiRecommendation(threatID string) string {
	recommendations := map[string]string{
		"ASI01": "Implement robust input validation and instruction boundary enforcement. Use system prompts that cannot be overridden by user input.",
		"ASI02": "Apply principle of least privilege to tool access. Implement tool call validation and rate limiting. Require confirmation for destructive operations.",
		"ASI03": "Implement strong authentication for agent actions. Use scoped credentials with short TTLs. Audit all privilege escalation attempts.",
		"ASI04": "Vet all third-party tools and MCP servers. Implement code signing for plugins. Monitor for supply chain anomalies.",
		"ASI05": "Sandbox all code execution. Disable shell access unless explicitly required. Validate and sanitize all generated code.",
		"ASI06": "Implement memory isolation between sessions. Validate RAG retrieval results. Use content signing for persistent context.",
		"ASI07": "Authenticate all inter-agent messages. Implement message integrity verification. Use encrypted channels for agent communication.",
		"ASI08": "Implement circuit breakers for cascading operations. Limit error propagation scope. Use graceful degradation patterns.",
		"ASI09": "Implement explicit trust boundaries. Require human confirmation for high-risk operations. Limit conversation context influence.",
		"ASI10": "Implement agent behavioral monitoring. Use anomaly detection for agent actions. Maintain audit logs for forensic analysis.",
	}
	if rec, ok := recommendations[threatID]; ok {
		return rec
	}
	return "Review agent security configuration and apply defense-in-depth principles."
}

// GetTrustDimensionName returns the human-readable name for a trust dimension
func GetTrustDimensionName(dimension string) string {
	names := map[string]string{
		"hallucination":         "Factual Hallucination",
		"over_refusal":          "Over-Refusal of Valid Requests",
		"bias":                  "Demographic Bias",
		"privacy_handling":      "Privacy & PII Handling",
		"uncertainty":           "Uncertainty Acknowledgment",
		"helpfulness":           "Response Helpfulness",
		"professional_redirect": "Professional Referral Appropriateness",
	}
	if name, ok := names[dimension]; ok {
		return name
	}
	return dimension
}

// GetTrustDimensionRegulatoryImpact returns regulatory frameworks affected by a trust dimension
func GetTrustDimensionRegulatoryImpact(dimension string) []string {
	impacts := map[string][]string{
		"hallucination":         {"EU AI Act Art. 13 (Transparency)", "NIST AI RMF MAP-1.5 (Accuracy)"},
		"over_refusal":          {"EU AI Act Art. 9 (Accessibility)", "NIST AI RMF GOVERN-1.3 (Usability)"},
		"bias":                  {"EU AI Act Art. 10 (Non-discrimination)", "NIST AI RMF MAP-2.3 (Fairness)", "EEOC Guidelines"},
		"privacy_handling":      {"GDPR Art. 5 (Data Minimization)", "EU AI Act Art. 10 (Data Governance)", "CCPA"},
		"uncertainty":           {"EU AI Act Art. 13 (Transparency)", "NIST AI RMF MEASURE-2.6 (Uncertainty)"},
		"helpfulness":           {"NIST AI RMF GOVERN-1.3 (Usability)", "Consumer Protection Standards"},
		"professional_redirect": {"Medical Device Regulations", "Financial Services Regulations", "Legal Practice Rules"},
	}
	if impact, ok := impacts[dimension]; ok {
		return impact
	}
	return []string{}
}

// GetTrustDimensionRecommendation returns a recommendation for addressing a trust dimension issue
func GetTrustDimensionRecommendation(dimension string) string {
	recommendations := map[string]string{
		"hallucination":         "Implement fact-checking against authoritative sources. Use retrieval-augmented generation with citation requirements. Add confidence calibration.",
		"over_refusal":          "Review and refine content policy boundaries. Implement nuanced safety filters that distinguish harmful from sensitive-but-legitimate requests.",
		"bias":                  "Audit training data and outputs for demographic bias. Implement bias detection monitoring. Use debiasing techniques in model fine-tuning.",
		"privacy_handling":      "Implement PII detection and redaction. Apply data minimization principles. Add explicit consent mechanisms for data handling.",
		"uncertainty":           "Train model to express uncertainty appropriately. Implement confidence scoring. Add explicit 'I don't know' responses for out-of-domain queries.",
		"helpfulness":           "Review response completeness criteria. Ensure direct answers to user questions. Reduce unnecessary caveats and hedging.",
		"professional_redirect": "Implement domain-specific guardrails. Add clear escalation paths to human professionals. Include appropriate disclaimers for sensitive domains.",
	}
	if rec, ok := recommendations[dimension]; ok {
		return rec
	}
	return "Review agent response quality and implement appropriate quality controls."
}

// Compliance represents compliance status for a regulatory framework
type Compliance struct {
	Compliant          bool     `bson:"compliant" json:"compliant"`
	Score              float64  `bson:"score" json:"score"`
	FailedRequirements []string `bson:"failedRequirements" json:"failedRequirements"`
}

// Recommendation represents a security recommendation
type Recommendation struct {
	Priority          string   `bson:"priority" json:"priority"`
	Category          string   `bson:"category" json:"category"`
	Finding           string   `bson:"finding" json:"finding"`
	Recommendation    string   `bson:"recommendation" json:"recommendation"`
	AffectedPromptIDs []string `bson:"affectedPromptIds,omitempty" json:"affectedPromptIds,omitempty"`
}

// EvalError represents an error that occurred during evaluation
type EvalError struct {
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
	Type      string    `bson:"type" json:"type"`
	Message   string    `bson:"message" json:"message"`
	PromptID  string    `bson:"promptId,omitempty" json:"promptId,omitempty"`
	SessionID string    `bson:"sessionId,omitempty" json:"sessionId,omitempty"`
}

// EvalRunStatus constants
const (
	EvalStatusPending   = "pending"
	EvalStatusRunning   = "running"
	EvalStatusCompleted = "completed"
	EvalStatusFailed    = "failed"
	EvalStatusCancelled = "cancelled"
)

// EvalMode constants
const (
	EvalModeQuick    = "quick"
	EvalModeStandard = "standard"
	EvalModeDeep     = "deep"
)

// CreateEvalRunRequest represents the request body for creating an evaluation run
type CreateEvalRunRequest struct {
	Mode     string                `json:"mode" binding:"required,oneof=quick standard deep"`
	EvalType string                `json:"evalType" binding:"required,oneof=red_team trust general"`
	Config   *EvalRunConfigRequest `json:"config,omitempty"`
}

// Note: EvalType constants (EvalTypeRedTeam, EvalTypeTrust, EvalTypeGeneral) are defined in dataset.go

// EvalRunConfigRequest represents optional configuration overrides
type EvalRunConfigRequest struct {
	RequestsPerMinute      *int     `json:"requestsPerMinute,omitempty"`
	IncludeDatasets        []string `json:"includeDatasets,omitempty"`
	ScenarioSetIDs         []string `json:"scenarioSetIds,omitempty"`         // Generated scenario set IDs
	ExcludeCategories      []string `json:"excludeCategories,omitempty"`
	IncludeBuiltInDatasets *bool    `json:"includeBuiltInDatasets,omitempty"` // Include mode-default datasets (default: false)
}

// CreateEvalRunResponse represents the response when creating an evaluation run
type CreateEvalRunResponse struct {
	RunID            string    `json:"runId"`
	AgentID          string    `json:"agentId"`
	Mode             string    `json:"mode"`
	EvalType         string    `json:"evalType"`
	Status           string    `json:"status"`
	EstimatedPrompts int       `json:"estimatedPrompts"`
	CreatedAt        time.Time `json:"createdAt"`
}

// EvalRunListResponse represents a paginated list of evaluation runs
type EvalRunListResponse struct {
	Runs   []EvalRunSummary `json:"runs"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// EvalRunSummary represents a summarized view of an evaluation run
type EvalRunSummary struct {
	RunID         string         `json:"runId"`
	Mode          string         `json:"mode"`
	EvalType      string         `json:"evalType"`
	Status        string         `json:"status"`
	SummaryScores *SummaryScores `json:"summaryScores,omitempty"`
	StartedAt     *time.Time     `json:"startedAt,omitempty"`
	CompletedAt   *time.Time     `json:"completedAt,omitempty"`
}
