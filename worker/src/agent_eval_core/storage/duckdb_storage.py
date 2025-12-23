"""DuckDB storage backend for analytics.

DuckDB is an embedded analytical database - no server required.
Perfect for single-node OSS deployments.
"""

import os
from pathlib import Path
from typing import Any

import structlog

from agent_eval_core.config import settings
from agent_eval_core.storage.base import AnalyticsStorage, EvalDetail, EvalSession

logger = structlog.get_logger()

# Schema for eval_details table
EVAL_DETAILS_SCHEMA = """
CREATE TABLE IF NOT EXISTS eval_details (
    run_id VARCHAR,
    agent_id VARCHAR,
    dataset_id VARCHAR,
    dataset_version VARCHAR,
    category VARCHAR,
    prompt_id VARCHAR,
    prompt_text VARCHAR,
    prompt_metadata VARCHAR,
    attack_type VARCHAR,
    severity VARCHAR,
    agent_response VARCHAR,
    response_latency_ms INTEGER,
    session_id VARCHAR,
    turn_number INTEGER,
    judgment VARCHAR,
    judgment_confidence DOUBLE,
    judge_model VARCHAR,
    judgment_reasoning VARCHAR,
    failure_type VARCHAR,
    regulatory_flags VARCHAR[],
    attack_success VARCHAR,
    owasp_asi_threat VARCHAR,
    maestro_threat VARCHAR,
    agentic_attack_vector VARCHAR,
    tool_context VARCHAR[],
    regulatory_mapping VARCHAR[],
    timestamp TIMESTAMP
)
"""

# Schema for eval_sessions table
EVAL_SESSIONS_SCHEMA = """
CREATE TABLE IF NOT EXISTS eval_sessions (
    run_id VARCHAR,
    agent_id VARCHAR,
    session_id VARCHAR,
    session_status VARCHAR,
    prompts_executed INTEGER,
    prompts_passed INTEGER,
    prompts_failed INTEGER,
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    reset_reason VARCHAR,
    error_message VARCHAR,
    timestamp TIMESTAMP
)
"""


class DuckDBStorage(AnalyticsStorage):
    """DuckDB-based analytics storage.

    Uses an embedded DuckDB database file for storing evaluation
    results. No external server required.
    """

    def __init__(self, db_path: str | None = None, batch_size: int = 100) -> None:
        """Initialize DuckDB storage.

        Args:
            db_path: Path to the DuckDB database file. Defaults to config.
            batch_size: Number of records to batch before flushing.
        """
        self.db_path = db_path or settings.duckdb_path
        self.batch_size = batch_size
        self._conn: Any = None
        self._batch_details: list[dict[str, Any]] = []
        self._batch_sessions: list[dict[str, Any]] = []

    async def connect(self) -> None:
        """Connect to DuckDB and create tables if needed."""
        try:
            import duckdb

            # Ensure directory exists
            db_dir = os.path.dirname(self.db_path)
            if db_dir:
                Path(db_dir).mkdir(parents=True, exist_ok=True)

            self._conn = duckdb.connect(self.db_path)

            # Create tables
            self._conn.execute(EVAL_DETAILS_SCHEMA)
            self._conn.execute(EVAL_SESSIONS_SCHEMA)

            logger.info("Connected to DuckDB", path=self.db_path)

        except ImportError:
            logger.warning("DuckDB not installed, analytics disabled. Install with: pip install duckdb")
            self._conn = None
        except Exception as e:
            logger.warning("Failed to connect to DuckDB", error=str(e))
            self._conn = None

    async def close(self) -> None:
        """Close DuckDB connection."""
        if self._conn:
            self.flush()
            self._conn.close()
            self._conn = None
            logger.info("Closed DuckDB connection")

    def is_connected(self) -> bool:
        """Check if DuckDB is connected."""
        return self._conn is not None

    def add_eval_detail(self, detail: EvalDetail) -> None:
        """Add an evaluation detail to the batch."""
        if not self._conn:
            return

        self._batch_details.append(detail.to_dict())

        if len(self._batch_details) >= self.batch_size:
            self._flush_details()

    def add_session(self, session: EvalSession) -> None:
        """Add a session record to the batch."""
        if not self._conn:
            return

        self._batch_sessions.append(session.to_dict())

        if len(self._batch_sessions) >= self.batch_size:
            self._flush_sessions()

    def flush(self) -> None:
        """Flush all pending batches."""
        self._flush_details()
        self._flush_sessions()

    def _flush_details(self) -> None:
        """Flush eval details batch to DuckDB."""
        if not self._conn or not self._batch_details:
            return

        try:
            # Use parameterized insert for safety
            columns = [
                "run_id", "agent_id", "dataset_id", "dataset_version",
                "category", "prompt_id", "prompt_text", "prompt_metadata",
                "attack_type", "severity", "agent_response", "response_latency_ms",
                "session_id", "turn_number", "judgment", "judgment_confidence",
                "judge_model", "judgment_reasoning", "failure_type",
                "regulatory_flags", "attack_success", "owasp_asi_threat",
                "maestro_threat", "agentic_attack_vector", "tool_context",
                "regulatory_mapping", "timestamp"
            ]

            placeholders = ", ".join(["?" for _ in columns])
            col_names = ", ".join(columns)

            for row in self._batch_details:
                values = [row.get(col) for col in columns]
                self._conn.execute(
                    f"INSERT INTO eval_details ({col_names}) VALUES ({placeholders})",
                    values
                )

            logger.debug("Flushed eval details to DuckDB", count=len(self._batch_details))
            self._batch_details = []

        except Exception as e:
            logger.error("Failed to flush eval details to DuckDB", error=str(e))

    def _flush_sessions(self) -> None:
        """Flush session batch to DuckDB."""
        if not self._conn or not self._batch_sessions:
            return

        try:
            columns = [
                "run_id", "agent_id", "session_id", "session_status",
                "prompts_executed", "prompts_passed", "prompts_failed",
                "started_at", "ended_at", "reset_reason", "error_message",
                "timestamp"
            ]

            placeholders = ", ".join(["?" for _ in columns])
            col_names = ", ".join(columns)

            for row in self._batch_sessions:
                values = [row.get(col) for col in columns]
                self._conn.execute(
                    f"INSERT INTO eval_sessions ({col_names}) VALUES ({placeholders})",
                    values
                )

            logger.debug("Flushed eval sessions to DuckDB", count=len(self._batch_sessions))
            self._batch_sessions = []

        except Exception as e:
            logger.error("Failed to flush eval sessions to DuckDB", error=str(e))

    def query(self, sql: str) -> list[dict[str, Any]]:
        """Execute a query and return results as list of dicts.

        Useful for analytics queries.

        Args:
            sql: SQL query to execute

        Returns:
            List of result rows as dictionaries
        """
        if not self._conn:
            return []

        try:
            result = self._conn.execute(sql).fetchall()
            columns = [desc[0] for desc in self._conn.description]
            return [dict(zip(columns, row)) for row in result]
        except Exception as e:
            logger.error("DuckDB query failed", error=str(e), sql=sql[:100])
            return []
