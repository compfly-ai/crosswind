package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"
)

// EvalDetail represents a single evaluation result row
type EvalDetail struct {
	RunID            string    `ch:"run_id"`
	OrgID            string    `ch:"org_id"`
	AgentID          string    `ch:"agent_id"`
	DatasetID        string    `ch:"dataset_id"`
	DatasetVersion   string    `ch:"dataset_version"`
	Category         string    `ch:"category"`
	PromptID         string    `ch:"prompt_id"`
	PromptText       string    `ch:"prompt_text"`
	PromptMetadata   string    `ch:"prompt_metadata"` // JSON
	AttackType       string    `ch:"attack_type"`
	Severity         string    `ch:"severity"`
	AgentResponse    string    `ch:"agent_response"`
	ResponseLatencyMs uint32   `ch:"response_latency_ms"`
	SessionID        string    `ch:"session_id"`
	TurnNumber       uint8     `ch:"turn_number"`
	Judgment         string    `ch:"judgment"` // pass | fail | uncertain | error
	JudgmentConfidence float32 `ch:"judgment_confidence"`
	JudgeModel       string    `ch:"judge_model"`
	JudgmentReasoning string   `ch:"judgment_reasoning"`
	FailureType      *string   `ch:"failure_type"`
	RegulatoryFlags  []string  `ch:"regulatory_flags"`
	Timestamp        time.Time `ch:"timestamp"`
}

// EvalDetailsRepository handles eval_details table operations
type EvalDetailsRepository struct {
	conn     driver.Conn
	database string
	logger   *zap.Logger
}

// InsertBatch inserts multiple eval details in a batch
func (r *EvalDetailsRepository) InsertBatch(ctx context.Context, details []EvalDetail) error {
	if len(details) == 0 {
		return nil
	}

	batch, err := r.conn.PrepareBatch(ctx, fmt.Sprintf(`
		INSERT INTO %s.eval_details (
			run_id, org_id, agent_id, dataset_id, dataset_version, category,
			prompt_id, prompt_text, prompt_metadata, attack_type, severity,
			agent_response, response_latency_ms, session_id, turn_number,
			judgment, judgment_confidence, judge_model, judgment_reasoning,
			failure_type, regulatory_flags, timestamp
		)
	`, r.database))
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, d := range details {
		err := batch.Append(
			d.RunID, d.OrgID, d.AgentID, d.DatasetID, d.DatasetVersion, d.Category,
			d.PromptID, d.PromptText, d.PromptMetadata, d.AttackType, d.Severity,
			d.AgentResponse, d.ResponseLatencyMs, d.SessionID, d.TurnNumber,
			d.Judgment, d.JudgmentConfidence, d.JudgeModel, d.JudgmentReasoning,
			d.FailureType, d.RegulatoryFlags, d.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	r.logger.Debug("inserted eval details batch",
		zap.Int("count", len(details)),
		zap.String("run_id", details[0].RunID))

	return nil
}

// GetByRunID retrieves all eval details for a run
func (r *EvalDetailsRepository) GetByRunID(ctx context.Context, runID string) ([]EvalDetail, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(`
		SELECT
			run_id, org_id, agent_id, dataset_id, dataset_version, category,
			prompt_id, prompt_text, prompt_metadata, attack_type, severity,
			agent_response, response_latency_ms, session_id, turn_number,
			judgment, judgment_confidence, judge_model, judgment_reasoning,
			failure_type, regulatory_flags, timestamp
		FROM %s.eval_details
		WHERE run_id = ?
		ORDER BY timestamp
	`, r.database), runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query eval details: %w", err)
	}
	defer rows.Close()

	var details []EvalDetail
	for rows.Next() {
		var d EvalDetail
		err := rows.Scan(
			&d.RunID, &d.OrgID, &d.AgentID, &d.DatasetID, &d.DatasetVersion, &d.Category,
			&d.PromptID, &d.PromptText, &d.PromptMetadata, &d.AttackType, &d.Severity,
			&d.AgentResponse, &d.ResponseLatencyMs, &d.SessionID, &d.TurnNumber,
			&d.Judgment, &d.JudgmentConfidence, &d.JudgeModel, &d.JudgmentReasoning,
			&d.FailureType, &d.RegulatoryFlags, &d.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		details = append(details, d)
	}

	return details, nil
}

// GetFailuresByRunID retrieves only failed eval details for a run
func (r *EvalDetailsRepository) GetFailuresByRunID(ctx context.Context, runID string, limit int) ([]EvalDetail, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(`
		SELECT
			run_id, org_id, agent_id, dataset_id, dataset_version, category,
			prompt_id, prompt_text, prompt_metadata, attack_type, severity,
			agent_response, response_latency_ms, session_id, turn_number,
			judgment, judgment_confidence, judge_model, judgment_reasoning,
			failure_type, regulatory_flags, timestamp
		FROM %s.eval_details
		WHERE run_id = ? AND judgment = 'fail'
		ORDER BY
			CASE severity
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				ELSE 4
			END,
			timestamp
		LIMIT ?
	`, r.database), runID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query failures: %w", err)
	}
	defer rows.Close()

	var details []EvalDetail
	for rows.Next() {
		var d EvalDetail
		err := rows.Scan(
			&d.RunID, &d.OrgID, &d.AgentID, &d.DatasetID, &d.DatasetVersion, &d.Category,
			&d.PromptID, &d.PromptText, &d.PromptMetadata, &d.AttackType, &d.Severity,
			&d.AgentResponse, &d.ResponseLatencyMs, &d.SessionID, &d.TurnNumber,
			&d.Judgment, &d.JudgmentConfidence, &d.JudgeModel, &d.JudgmentReasoning,
			&d.FailureType, &d.RegulatoryFlags, &d.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		details = append(details, d)
	}

	return details, nil
}

// AggregateStats represents aggregated statistics for a run
type AggregateStats struct {
	TotalPrompts      uint64  `ch:"total"`
	PassCount         uint64  `ch:"pass_count"`
	FailCount         uint64  `ch:"fail_count"`
	UncertainCount    uint64  `ch:"uncertain_count"`
	ErrorCount        uint64  `ch:"error_count"`
	AvgLatencyMs      float64 `ch:"avg_latency"`
	P50LatencyMs      float64 `ch:"p50_latency"`
	P95LatencyMs      float64 `ch:"p95_latency"`
	P99LatencyMs      float64 `ch:"p99_latency"`
}

// GetStatsByRunID retrieves aggregated statistics for a run
func (r *EvalDetailsRepository) GetStatsByRunID(ctx context.Context, runID string) (*AggregateStats, error) {
	row := r.conn.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			count() as total,
			countIf(judgment = 'pass') as pass_count,
			countIf(judgment = 'fail') as fail_count,
			countIf(judgment = 'uncertain') as uncertain_count,
			countIf(judgment = 'error') as error_count,
			avg(response_latency_ms) as avg_latency,
			quantile(0.5)(response_latency_ms) as p50_latency,
			quantile(0.95)(response_latency_ms) as p95_latency,
			quantile(0.99)(response_latency_ms) as p99_latency
		FROM %s.eval_details
		WHERE run_id = ?
	`, r.database), runID)

	var stats AggregateStats
	err := row.Scan(
		&stats.TotalPrompts,
		&stats.PassCount,
		&stats.FailCount,
		&stats.UncertainCount,
		&stats.ErrorCount,
		&stats.AvgLatencyMs,
		&stats.P50LatencyMs,
		&stats.P95LatencyMs,
		&stats.P99LatencyMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return &stats, nil
}

// CategoryBreakdown represents stats broken down by category
type CategoryBreakdown struct {
	Category   string `ch:"category"`
	Total      uint64 `ch:"total"`
	PassCount  uint64 `ch:"pass_count"`
	FailCount  uint64 `ch:"fail_count"`
	PassRate   float64 `ch:"pass_rate"`
}

// GetCategoryBreakdown retrieves statistics broken down by category
func (r *EvalDetailsRepository) GetCategoryBreakdown(ctx context.Context, runID string) ([]CategoryBreakdown, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(`
		SELECT
			category,
			count() as total,
			countIf(judgment = 'pass') as pass_count,
			countIf(judgment = 'fail') as fail_count,
			if(count() > 0, countIf(judgment = 'pass') / count(), 0) as pass_rate
		FROM %s.eval_details
		WHERE run_id = ?
		GROUP BY category
		ORDER BY fail_count DESC
	`, r.database), runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query breakdown: %w", err)
	}
	defer rows.Close()

	var breakdown []CategoryBreakdown
	for rows.Next() {
		var b CategoryBreakdown
		err := rows.Scan(&b.Category, &b.Total, &b.PassCount, &b.FailCount, &b.PassRate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		breakdown = append(breakdown, b)
	}

	return breakdown, nil
}

// SeverityBreakdown represents stats broken down by severity
type SeverityBreakdown struct {
	Severity  string `ch:"severity"`
	Total     uint64 `ch:"total"`
	PassCount uint64 `ch:"pass_count"`
	FailCount uint64 `ch:"fail_count"`
}

// GetSeverityBreakdown retrieves statistics broken down by severity
func (r *EvalDetailsRepository) GetSeverityBreakdown(ctx context.Context, runID string) ([]SeverityBreakdown, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(`
		SELECT
			severity,
			count() as total,
			countIf(judgment = 'pass') as pass_count,
			countIf(judgment = 'fail') as fail_count
		FROM %s.eval_details
		WHERE run_id = ?
		GROUP BY severity
		ORDER BY
			CASE severity
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				ELSE 4
			END
	`, r.database), runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query severity breakdown: %w", err)
	}
	defer rows.Close()

	var breakdown []SeverityBreakdown
	for rows.Next() {
		var b SeverityBreakdown
		err := rows.Scan(&b.Severity, &b.Total, &b.PassCount, &b.FailCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		breakdown = append(breakdown, b)
	}

	return breakdown, nil
}
