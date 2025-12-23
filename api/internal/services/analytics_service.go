package services

import (
	"context"

	"github.com/compfly-ai/crosswind/internal/repository/clickhouse"
)

// AnalyticsService provides analytics and reporting capabilities
type AnalyticsService struct {
	ch *clickhouse.Client
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(ch *clickhouse.Client) *AnalyticsService {
	return &AnalyticsService{ch: ch}
}

// IsEnabled returns whether analytics is available
func (s *AnalyticsService) IsEnabled() bool {
	return s.ch != nil
}

// RunStats represents aggregated statistics for an evaluation run
type RunStats struct {
	TotalPrompts   uint64  `json:"totalPrompts"`
	PassCount      uint64  `json:"passCount"`
	FailCount      uint64  `json:"failCount"`
	UncertainCount uint64  `json:"uncertainCount"`
	ErrorCount     uint64  `json:"errorCount"`
	PassRate       float64 `json:"passRate"`
	AvgLatencyMs   float64 `json:"avgLatencyMs"`
	P50LatencyMs   float64 `json:"p50LatencyMs"`
	P95LatencyMs   float64 `json:"p95LatencyMs"`
	P99LatencyMs   float64 `json:"p99LatencyMs"`
}

// GetRunStats retrieves aggregated statistics for a run
func (s *AnalyticsService) GetRunStats(ctx context.Context, runID string) (*RunStats, error) {
	if s.ch == nil {
		return nil, nil
	}

	stats, err := s.ch.EvalDetails().GetStatsByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}

	passRate := float64(0)
	if stats.TotalPrompts > 0 {
		passRate = float64(stats.PassCount) / float64(stats.TotalPrompts)
	}

	return &RunStats{
		TotalPrompts:   stats.TotalPrompts,
		PassCount:      stats.PassCount,
		FailCount:      stats.FailCount,
		UncertainCount: stats.UncertainCount,
		ErrorCount:     stats.ErrorCount,
		PassRate:       passRate,
		AvgLatencyMs:   stats.AvgLatencyMs,
		P50LatencyMs:   stats.P50LatencyMs,
		P95LatencyMs:   stats.P95LatencyMs,
		P99LatencyMs:   stats.P99LatencyMs,
	}, nil
}

// CategoryStats represents statistics for a category
type CategoryStats struct {
	Category  string  `json:"category"`
	Total     uint64  `json:"total"`
	PassCount uint64  `json:"passCount"`
	FailCount uint64  `json:"failCount"`
	PassRate  float64 `json:"passRate"`
}

// GetCategoryBreakdown retrieves statistics broken down by category
func (s *AnalyticsService) GetCategoryBreakdown(ctx context.Context, runID string) ([]CategoryStats, error) {
	if s.ch == nil {
		return nil, nil
	}

	breakdown, err := s.ch.EvalDetails().GetCategoryBreakdown(ctx, runID)
	if err != nil {
		return nil, err
	}

	result := make([]CategoryStats, len(breakdown))
	for i, b := range breakdown {
		result[i] = CategoryStats{
			Category:  b.Category,
			Total:     b.Total,
			PassCount: b.PassCount,
			FailCount: b.FailCount,
			PassRate:  b.PassRate,
		}
	}

	return result, nil
}

// SeverityStats represents statistics for a severity level
type SeverityStats struct {
	Severity  string `json:"severity"`
	Total     uint64 `json:"total"`
	PassCount uint64 `json:"passCount"`
	FailCount uint64 `json:"failCount"`
}

// GetSeverityBreakdown retrieves statistics broken down by severity
func (s *AnalyticsService) GetSeverityBreakdown(ctx context.Context, runID string) ([]SeverityStats, error) {
	if s.ch == nil {
		return nil, nil
	}

	breakdown, err := s.ch.EvalDetails().GetSeverityBreakdown(ctx, runID)
	if err != nil {
		return nil, err
	}

	result := make([]SeverityStats, len(breakdown))
	for i, b := range breakdown {
		result[i] = SeverityStats{
			Severity:  b.Severity,
			Total:     b.Total,
			PassCount: b.PassCount,
			FailCount: b.FailCount,
		}
	}

	return result, nil
}

// SessionStats represents session-level statistics
type SessionStats struct {
	TotalSessions    uint64             `json:"totalSessions"`
	CompletedCount   uint64             `json:"completedCount"`
	ResetCount       uint64             `json:"resetCount"`
	ErrorCount       uint64             `json:"errorCount"`
	TotalDurationSec float64            `json:"totalDurationSec"`
	AvgDurationSec   float64            `json:"avgDurationSec"`
	ResetReasons     map[string]uint64  `json:"resetReasons,omitempty"`
}

// GetSessionStats retrieves session-level statistics for a run
func (s *AnalyticsService) GetSessionStats(ctx context.Context, runID string) (*SessionStats, error) {
	if s.ch == nil {
		return nil, nil
	}

	summary, err := s.ch.EvalSessions().GetSummaryByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}

	reasons, err := s.ch.EvalSessions().GetResetReasons(ctx, runID)
	if err != nil {
		return nil, err
	}

	return &SessionStats{
		TotalSessions:    summary.TotalSessions,
		CompletedCount:   summary.CompletedCount,
		ResetCount:       summary.ResetCount,
		ErrorCount:       summary.ErrorCount,
		TotalDurationSec: summary.TotalDurationSec,
		AvgDurationSec:   summary.AvgDurationSec,
		ResetReasons:     reasons,
	}, nil
}

// FailureDetail represents a detailed failure record
type FailureDetail struct {
	PromptID          string   `json:"promptId"`
	Category          string   `json:"category"`
	Severity          string   `json:"severity"`
	PromptText        string   `json:"promptText"`
	AgentResponse     string   `json:"agentResponse"`
	JudgeModel        string   `json:"judgeModel"`
	JudgmentReasoning string   `json:"judgmentReasoning"`
	FailureType       *string  `json:"failureType,omitempty"`
	RegulatoryFlags   []string `json:"regulatoryFlags,omitempty"`
	ResponseLatencyMs uint32   `json:"responseLatencyMs"`
}

// GetFailures retrieves detailed failure records for a run
func (s *AnalyticsService) GetFailures(ctx context.Context, runID string, limit int) ([]FailureDetail, error) {
	if s.ch == nil {
		return nil, nil
	}

	if limit <= 0 {
		limit = 100
	}

	failures, err := s.ch.EvalDetails().GetFailuresByRunID(ctx, runID, limit)
	if err != nil {
		return nil, err
	}

	result := make([]FailureDetail, len(failures))
	for i, f := range failures {
		result[i] = FailureDetail{
			PromptID:          f.PromptID,
			Category:          f.Category,
			Severity:          f.Severity,
			PromptText:        f.PromptText,
			AgentResponse:     f.AgentResponse,
			JudgeModel:        f.JudgeModel,
			JudgmentReasoning: f.JudgmentReasoning,
			FailureType:       f.FailureType,
			RegulatoryFlags:   f.RegulatoryFlags,
			ResponseLatencyMs: f.ResponseLatencyMs,
		}
	}

	return result, nil
}

// StoreEvalDetail stores a single evaluation detail to ClickHouse
func (s *AnalyticsService) StoreEvalDetail(ctx context.Context, detail *clickhouse.EvalDetail) error {
	if s.ch == nil {
		return nil
	}

	return s.ch.EvalDetails().InsertBatch(ctx, []clickhouse.EvalDetail{*detail})
}

// StoreEvalDetails stores multiple evaluation details to ClickHouse
func (s *AnalyticsService) StoreEvalDetails(ctx context.Context, details []clickhouse.EvalDetail) error {
	if s.ch == nil {
		return nil
	}

	return s.ch.EvalDetails().InsertBatch(ctx, details)
}

// StoreEvalSession stores a session record to ClickHouse
func (s *AnalyticsService) StoreEvalSession(ctx context.Context, session *clickhouse.EvalSession) error {
	if s.ch == nil {
		return nil
	}

	return s.ch.EvalSessions().Insert(ctx, session)
}
