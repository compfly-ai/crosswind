package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EvalResultsSummary holds the long-term summary of evaluation results
type EvalResultsSummary struct {
	ID                 primitive.ObjectID       `bson:"_id,omitempty" json:"-"`
	RunID              string                   `bson:"runId" json:"runId"`
	AgentID            string                   `bson:"agentId" json:"agentId"`
	Failures           []PromptResultDetail     `bson:"failures" json:"failures"`
	SamplePasses       []PromptResultDetail     `bson:"samplePasses" json:"samplePasses"`
	CategoryBreakdown  map[string]CategoryStats `bson:"categoryBreakdown" json:"categoryBreakdown"`
	SeverityBreakdown  map[string]CategoryStats `bson:"severityBreakdown,omitempty" json:"severityBreakdown,omitempty"`
	PerformanceMetrics PerformanceMetrics       `bson:"performanceMetrics" json:"performanceMetrics"`
	CreatedAt          time.Time                `bson:"createdAt" json:"createdAt"`
}

// ConversationMessage represents a single message in a conversation
type ConversationMessage struct {
	Role    string `bson:"role" json:"role"`
	Content string `bson:"content" json:"content"`
}

// PromptResultDetail holds detailed information about a single prompt result
type PromptResultDetail struct {
	RunID               string    `bson:"runId,omitempty" json:"runId,omitempty"`
	AgentID             string    `bson:"agentId,omitempty" json:"agentId,omitempty"`
	Verdict             string    `bson:"verdict,omitempty" json:"verdict,omitempty"` // pass, fail, error
	PromptID            string    `bson:"promptId" json:"promptId"`
	DatasetID           string    `bson:"datasetId" json:"datasetId"`
	Category            string    `bson:"category" json:"category"`
	AttackType          string    `bson:"attackType" json:"attackType"`
	Severity            string    `bson:"severity" json:"severity"`
	Prompt              string    `bson:"prompt" json:"prompt,omitempty"`
	Response            string    `bson:"response" json:"response,omitempty"`
	Judgment            string    `bson:"judgment" json:"judgment"`
	JudgmentConfidence  float64   `bson:"judgmentConfidence" json:"judgmentConfidence"`
	JudgmentReasoning   string    `bson:"judgmentReasoning,omitempty" json:"judgmentReasoning,omitempty"`
	JudgeModel          string    `bson:"judgeModel" json:"-"`
	FailureType         string    `bson:"failureType,omitempty" json:"failureType,omitempty"`
	RegulatoryFlags     []string  `bson:"regulatoryFlags,omitempty" json:"regulatoryFlags,omitempty"`
	TurnNumber          int       `bson:"turnNumber" json:"turnNumber"`
	ResponseLatencyMs   int       `bson:"responseLatencyMs" json:"responseLatencyMs"`
	Timestamp           time.Time `bson:"timestamp" json:"timestamp"`
	// Visibility inherited from source dataset - controls what can be exposed in API
	Visibility string `bson:"visibility,omitempty" json:"-"`
	// Full conversation history when follow-ups occurred (turnNumber > 1)
	ConversationHistory []ConversationMessage `bson:"conversationHistory,omitempty" json:"conversationHistory,omitempty"`
	// Type of deflection that triggered follow-ups (greeting, clarification, topic_change, vague)
	DeflectionType string `bson:"deflectionType,omitempty" json:"deflectionType,omitempty"`

	// Agentic evaluation fields
	// AttackSuccess: Classification of attack outcome (full, partial, none)
	AttackSuccess string `bson:"attackSuccess,omitempty" json:"attackSuccess,omitempty"`
	// OwaspAsiThreat: OWASP Agentic AI Top 10 threat ID (ASI01-ASI10) - primary classification
	OwaspAsiThreat string `bson:"owaspAsiThreat,omitempty" json:"owaspAsiThreat,omitempty"`
	// MaestroThreat: MAESTRO framework threat ID (T1-T8) - secondary classification
	MaestroThreat string `bson:"maestroThreat,omitempty" json:"maestroThreat,omitempty"`
	// AgenticAttackVector: How the attack was delivered (direct, indirect, multi_turn, memory, etc.)
	AgenticAttackVector string `bson:"agenticAttackVector,omitempty" json:"agenticAttackVector,omitempty"`
	// RegulatoryMapping: Regulatory framework references from the prompt
	RegulatoryMapping []string `bson:"regulatoryMapping,omitempty" json:"regulatoryMapping,omitempty"`
	ToolContext        []string           `bson:"toolContext,omitempty" json:"toolContext,omitempty"`
	MultiTurnJudgment  *MultiTurnJudgment `bson:"multiTurnJudgment,omitempty" json:"multiTurnJudgment,omitempty"`

	// Refusal quality fields (Phase 1 improvement)
	// RefusalQuality: How well the agent refused (hard_refuse, soft_refuse_redirect, partial_compliance, evasion, compliance)
	RefusalQuality string `bson:"refusalQuality,omitempty" json:"refusalQuality,omitempty"`
	// RefusalQualityScore: Numeric score 0.0-1.0 (1.0 = best refusal)
	RefusalQualityScore *float64 `bson:"refusalQualityScore,omitempty" json:"-"`
	// RefusalRationale: Brief explanation of the refusal quality classification
	RefusalRationale string `bson:"refusalRationale,omitempty" json:"refusalRationale,omitempty"`
}

// Redacted returns a copy with prompt/response content redacted
func (p PromptResultDetail) Redacted() PromptResultDetail {
	// Redact conversation history if present
	var redactedHistory []ConversationMessage
	if len(p.ConversationHistory) > 0 {
		redactedHistory = make([]ConversationMessage, len(p.ConversationHistory))
		for i, msg := range p.ConversationHistory {
			redactedHistory[i] = ConversationMessage{
				Role:    msg.Role,
				Content: "[REDACTED]",
			}
		}
	}

	return PromptResultDetail{
		PromptID:            p.PromptID,
		DatasetID:           p.DatasetID,
		Category:            p.Category,
		AttackType:          p.AttackType,
		Severity:            p.Severity,
		Prompt:              "[REDACTED - restricted dataset]",
		Response:            "[REDACTED - restricted dataset]",
		Judgment:            p.Judgment,
		JudgmentConfidence:  p.JudgmentConfidence,
		JudgmentReasoning:   "[REDACTED]",
		JudgeModel:          p.JudgeModel,
		FailureType:         p.FailureType,
		RegulatoryFlags:     p.RegulatoryFlags,
		TurnNumber:          p.TurnNumber,
		ResponseLatencyMs:   p.ResponseLatencyMs,
		Timestamp:           p.Timestamp,
		Visibility:          p.Visibility,
		ConversationHistory: redactedHistory,
		DeflectionType:      p.DeflectionType, // Keep deflection type visible
		// Agentic fields - keep metadata visible
		AttackSuccess:       p.AttackSuccess,
		OwaspAsiThreat:      p.OwaspAsiThreat,
		MaestroThreat:       p.MaestroThreat,
		AgenticAttackVector: p.AgenticAttackVector,
		RegulatoryMapping:   p.RegulatoryMapping,
		ToolContext:          p.ToolContext,
		MultiTurnJudgment:   p.MultiTurnJudgment,
		RefusalQuality:      p.RefusalQuality,
		RefusalQualityScore: p.RefusalQualityScore,
		RefusalRationale:    "[REDACTED]", // Rationale may contain content
	}
}

// FilterResultsByVisibility applies visibility rules to a slice of results
// Results with "redacted" visibility have content replaced
// Results with "scores_only" visibility are excluded entirely
func FilterResultsByVisibility(results []PromptResultDetail) []PromptResultDetail {
	filtered := make([]PromptResultDetail, 0, len(results))
	for _, r := range results {
		switch r.Visibility {
		case VisibilityScoresOnly:
			// Exclude entirely from individual results
			continue
		case VisibilityRedacted:
			// Include but redact content
			filtered = append(filtered, r.Redacted())
		default:
			// Full visibility - include as-is
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// MultiTurnJudgment holds detailed judgment for multi-turn attack sequences
type MultiTurnJudgment struct {
	FinalAttackSuccess string       `bson:"finalAttackSuccess" json:"finalAttackSuccess"`
	TurnResults        []TurnResult `bson:"turnResults" json:"turnResults"`
	EscalationDetected bool         `bson:"escalationDetected" json:"escalationDetected"`
	Reasoning          string       `bson:"reasoning" json:"reasoning"`
}

// TurnResult holds the judgment for a single turn in a multi-turn sequence
type TurnResult struct {
	TurnNumber      int     `bson:"turnNumber" json:"turnNumber"`
	Judgment        string  `bson:"judgment" json:"judgment"`
	AttackSuccess   string  `bson:"attackSuccess" json:"attackSuccess"`
	ConfidenceScore float64 `bson:"confidenceScore" json:"confidenceScore"`
}

// CategoryStats holds statistics for a specific category
type CategoryStats struct {
	Total        int     `bson:"total" json:"total"`
	Passed       int     `bson:"passed" json:"passed"`
	Failed       int     `bson:"failed" json:"failed"`
	Uncertain    int     `bson:"uncertain" json:"uncertain"`
	AvgLatencyMs float64 `bson:"avgLatencyMs" json:"avgLatencyMs"`
}

// PerformanceMetrics holds performance statistics for the evaluation
type PerformanceMetrics struct {
	AvgResponseLatencyMs  float64 `bson:"avgResponseLatencyMs" json:"avgResponseLatencyMs"`
	P50LatencyMs          float64 `bson:"p50LatencyMs" json:"p50LatencyMs"`
	P95LatencyMs          float64 `bson:"p95LatencyMs" json:"p95LatencyMs"`
	P99LatencyMs          float64 `bson:"p99LatencyMs" json:"p99LatencyMs"`
	TotalDurationSeconds  float64 `bson:"totalDurationSeconds" json:"totalDurationSeconds"`
	SessionsCreated       int     `bson:"sessionsCreated" json:"sessionsCreated"`
	SessionResets         int     `bson:"sessionResets" json:"sessionResets"`
}

// FailureType constants
const (
	FailureTypeJailbreakSuccess  = "jailbreak_success"
	FailureTypePIILeak           = "pii_leak"
	FailureTypeHarmfulContent    = "harmful_content"
	FailureTypeToolAbuse         = "tool_abuse"
	FailureTypeBoundaryViolation = "boundary_violation"
	FailureTypeMemoryLeak        = "memory_leak"
)

// JudgeModel constants
const (
	JudgeModelKeyword      = "keyword"
	JudgeModelEmbedding    = "embedding"
	JudgeModelGPT4oMini    = "gpt-4.1-mini"
	JudgeModelGPT4o        = "gpt-4o"
)

// Judgment constants
const (
	JudgmentPass      = "pass"
	JudgmentFail      = "fail"
	JudgmentUncertain = "uncertain"
	JudgmentError     = "error"
)

// GetResultsResponse represents the response for getting evaluation results
type GetResultsResponse struct {
	RunID                string                   `json:"runId"`
	Status               string                   `json:"status"`
	EvalType             string                   `json:"evalType"`
	SummaryScores        *SummaryScores           `json:"summaryScores,omitempty"`
	RegulatoryCompliance map[string]*Compliance   `json:"regulatoryCompliance,omitempty"`
	// ThreatAnalysis: Aggregated threat analysis for red_team evaluations
	// Maps results to OWASP Agentic AI Top 10 (primary) with attack success rates
	ThreatAnalysis *ThreatAnalysis `json:"threatAnalysis,omitempty"`
	// TrustAnalysis: Aggregated quality analysis for trust evaluations
	// Maps results to quality dimensions and regulatory frameworks
	TrustAnalysis       *TrustAnalysis           `json:"trustAnalysis,omitempty"`
	Recommendations     []Recommendation         `json:"recommendations,omitempty"`
	Failures            []PromptResultDetail     `json:"failures,omitempty"`
	SamplePasses        []PromptResultDetail     `json:"samplePasses,omitempty"`
	CategoryBreakdown   map[string]CategoryStats `json:"categoryBreakdown,omitempty"`
	SeverityBreakdown   map[string]CategoryStats `json:"severityBreakdown,omitempty"`
	PerformanceMetrics  *PerformanceMetrics      `json:"performanceMetrics,omitempty"`
}
