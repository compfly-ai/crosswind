package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"
)

// EvalSession represents a session tracking row
type EvalSession struct {
	RunID           string     `ch:"run_id"`
	OrgID           string     `ch:"org_id"`
	AgentID         string     `ch:"agent_id"`
	SessionID       string     `ch:"session_id"`
	SessionStatus   string     `ch:"session_status"` // active | completed | reset | error
	PromptsExecuted uint32     `ch:"prompts_executed"`
	PromptsPassed   uint32     `ch:"prompts_passed"`
	PromptsFailed   uint32     `ch:"prompts_failed"`
	ResetReason     *string    `ch:"reset_reason"`
	ErrorMessage    *string    `ch:"error_message"`
	StartedAt       time.Time  `ch:"started_at"`
	EndedAt         *time.Time `ch:"ended_at"`
	Timestamp       time.Time  `ch:"timestamp"`
}

// EvalSessionsRepository handles eval_sessions table operations
type EvalSessionsRepository struct {
	conn     driver.Conn
	database string
	logger   *zap.Logger
}

// Insert inserts a single session record
func (r *EvalSessionsRepository) Insert(ctx context.Context, session *EvalSession) error {
	err := r.conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.eval_sessions (
			run_id, org_id, agent_id, session_id, session_status,
			prompts_executed, prompts_passed, prompts_failed,
			reset_reason, error_message, started_at, ended_at, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.database),
		session.RunID, session.OrgID, session.AgentID, session.SessionID, session.SessionStatus,
		session.PromptsExecuted, session.PromptsPassed, session.PromptsFailed,
		session.ResetReason, session.ErrorMessage, session.StartedAt, session.EndedAt, session.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}

	r.logger.Debug("inserted eval session",
		zap.String("run_id", session.RunID),
		zap.String("session_id", session.SessionID),
		zap.String("status", session.SessionStatus))

	return nil
}

// InsertBatch inserts multiple session records
func (r *EvalSessionsRepository) InsertBatch(ctx context.Context, sessions []EvalSession) error {
	if len(sessions) == 0 {
		return nil
	}

	batch, err := r.conn.PrepareBatch(ctx, fmt.Sprintf(`
		INSERT INTO %s.eval_sessions (
			run_id, org_id, agent_id, session_id, session_status,
			prompts_executed, prompts_passed, prompts_failed,
			reset_reason, error_message, started_at, ended_at, timestamp
		)
	`, r.database))
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, s := range sessions {
		err := batch.Append(
			s.RunID, s.OrgID, s.AgentID, s.SessionID, s.SessionStatus,
			s.PromptsExecuted, s.PromptsPassed, s.PromptsFailed,
			s.ResetReason, s.ErrorMessage, s.StartedAt, s.EndedAt, s.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	r.logger.Debug("inserted eval sessions batch",
		zap.Int("count", len(sessions)),
		zap.String("run_id", sessions[0].RunID))

	return nil
}

// GetByRunID retrieves all sessions for a run
func (r *EvalSessionsRepository) GetByRunID(ctx context.Context, runID string) ([]EvalSession, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(`
		SELECT
			run_id, org_id, agent_id, session_id, session_status,
			prompts_executed, prompts_passed, prompts_failed,
			reset_reason, error_message, started_at, ended_at, timestamp
		FROM %s.eval_sessions
		WHERE run_id = ?
		ORDER BY started_at
	`, r.database), runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []EvalSession
	for rows.Next() {
		var s EvalSession
		err := rows.Scan(
			&s.RunID, &s.OrgID, &s.AgentID, &s.SessionID, &s.SessionStatus,
			&s.PromptsExecuted, &s.PromptsPassed, &s.PromptsFailed,
			&s.ResetReason, &s.ErrorMessage, &s.StartedAt, &s.EndedAt, &s.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// SessionSummary represents aggregated session statistics
type SessionSummary struct {
	TotalSessions    uint64  `ch:"total_sessions"`
	CompletedCount   uint64  `ch:"completed_count"`
	ResetCount       uint64  `ch:"reset_count"`
	ErrorCount       uint64  `ch:"error_count"`
	TotalDurationSec float64 `ch:"total_duration_sec"`
	AvgDurationSec   float64 `ch:"avg_duration_sec"`
}

// GetSummaryByRunID retrieves session summary statistics
func (r *EvalSessionsRepository) GetSummaryByRunID(ctx context.Context, runID string) (*SessionSummary, error) {
	row := r.conn.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			count() as total_sessions,
			countIf(session_status = 'completed') as completed_count,
			countIf(session_status = 'reset') as reset_count,
			countIf(session_status = 'error') as error_count,
			sum(if(ended_at IS NOT NULL, dateDiff('second', started_at, ended_at), 0)) as total_duration_sec,
			avg(if(ended_at IS NOT NULL, dateDiff('second', started_at, ended_at), 0)) as avg_duration_sec
		FROM %s.eval_sessions
		WHERE run_id = ?
	`, r.database), runID)

	var summary SessionSummary
	err := row.Scan(
		&summary.TotalSessions,
		&summary.CompletedCount,
		&summary.ResetCount,
		&summary.ErrorCount,
		&summary.TotalDurationSec,
		&summary.AvgDurationSec,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get session summary: %w", err)
	}

	return &summary, nil
}

// GetResetReasons retrieves reset reasons and counts
func (r *EvalSessionsRepository) GetResetReasons(ctx context.Context, runID string) (map[string]uint64, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(`
		SELECT
			reset_reason,
			count() as cnt
		FROM %s.eval_sessions
		WHERE run_id = ? AND session_status = 'reset' AND reset_reason IS NOT NULL
		GROUP BY reset_reason
		ORDER BY cnt DESC
	`, r.database), runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query reset reasons: %w", err)
	}
	defer rows.Close()

	reasons := make(map[string]uint64)
	for rows.Next() {
		var reason string
		var count uint64
		if err := rows.Scan(&reason, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		reasons[reason] = count
	}

	return reasons, nil
}
