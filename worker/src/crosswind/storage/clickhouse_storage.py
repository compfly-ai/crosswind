"""ClickHouse storage backend for analytics.

ClickHouse is a column-oriented database for real-time analytics.
Use this for large-scale deployments or when you already have ClickHouse.
"""

from typing import Any

import structlog

from crosswind.config import settings
from crosswind.storage.base import AnalyticsStorage, EvalDetail, EvalSession

logger = structlog.get_logger()


class ClickHouseStorage(AnalyticsStorage):
    """ClickHouse-based analytics storage.

    Connects to an external ClickHouse server for storing evaluation
    results. Supports both self-hosted and ClickHouse Cloud.
    """

    def __init__(self, batch_size: int = 100) -> None:
        """Initialize ClickHouse storage.

        Args:
            batch_size: Number of records to batch before flushing.
        """
        self.batch_size = batch_size
        self._client: Any = None
        self._batch_details: list[dict[str, Any]] = []
        self._batch_sessions: list[dict[str, Any]] = []

    async def connect(self) -> None:
        """Establish connection to ClickHouse."""
        try:
            import clickhouse_connect

            # Skip if not configured
            if not settings.clickhouse_host:
                logger.info("ClickHouse not configured, analytics disabled")
                self._client = None
                return

            # Build connection kwargs
            connect_kwargs: dict[str, Any] = {
                "host": settings.clickhouse_host,
                "port": settings.clickhouse_port,
                "database": settings.clickhouse_database,
            }

            # Add auth if configured
            if settings.clickhouse_user:
                connect_kwargs["username"] = settings.clickhouse_user
            if settings.clickhouse_password:
                connect_kwargs["password"] = settings.clickhouse_password

            # Use HTTPS for ClickHouse Cloud (port 8443)
            if settings.clickhouse_port == 8443:
                connect_kwargs["secure"] = True

            self._client = clickhouse_connect.get_client(**connect_kwargs)
            self._client.ping()

            logger.info(
                "Connected to ClickHouse",
                host=settings.clickhouse_host,
                database=settings.clickhouse_database,
            )

        except ImportError:
            logger.warning(
                "clickhouse-connect not installed. Install with: pip install clickhouse-connect"
            )
            self._client = None
        except Exception as e:
            logger.warning("Failed to connect to ClickHouse", error=str(e))
            self._client = None

    async def close(self) -> None:
        """Close the ClickHouse connection."""
        if self._client:
            self.flush()
            self._client.close()
            self._client = None
            logger.info("Closed ClickHouse connection")

    def is_connected(self) -> bool:
        """Check if ClickHouse is connected."""
        return self._client is not None

    def add_eval_detail(self, detail: EvalDetail) -> None:
        """Add an evaluation detail to the batch."""
        if not self._client:
            return

        self._batch_details.append(detail.to_dict())

        if len(self._batch_details) >= self.batch_size:
            self._flush_details()

    def add_session(self, session: EvalSession) -> None:
        """Add a session record to the batch."""
        if not self._client:
            return

        self._batch_sessions.append(session.to_dict())

        if len(self._batch_sessions) >= self.batch_size:
            self._flush_sessions()

    def flush(self) -> None:
        """Flush all pending batches."""
        self._flush_details()
        self._flush_sessions()

    def _flush_details(self) -> None:
        """Flush eval details batch to ClickHouse."""
        if not self._client or not self._batch_details:
            return

        try:
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

            data = [
                [row.get(col) for col in columns]
                for row in self._batch_details
            ]

            self._client.insert(
                "eval_details",
                data,
                column_names=columns,
            )

            logger.debug("Flushed eval details to ClickHouse", count=len(self._batch_details))
            self._batch_details = []

        except Exception as e:
            logger.error("Failed to flush eval details to ClickHouse", error=str(e))

    def _flush_sessions(self) -> None:
        """Flush session batch to ClickHouse."""
        if not self._client or not self._batch_sessions:
            return

        try:
            columns = [
                "run_id", "agent_id", "session_id", "session_status",
                "prompts_executed", "prompts_passed", "prompts_failed",
                "started_at", "ended_at", "reset_reason", "error_message",
                "timestamp"
            ]

            data = [
                [row.get(col) for col in columns]
                for row in self._batch_sessions
            ]

            self._client.insert(
                "eval_sessions",
                data,
                column_names=columns,
            )

            logger.debug("Flushed eval sessions to ClickHouse", count=len(self._batch_sessions))
            self._batch_sessions = []

        except Exception as e:
            logger.error("Failed to flush eval sessions to ClickHouse", error=str(e))
