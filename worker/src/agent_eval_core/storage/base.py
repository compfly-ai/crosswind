"""Abstract base class for analytics storage backends."""

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any


@dataclass
class EvalDetail:
    """A single evaluation result for analytics storage."""

    run_id: str
    agent_id: str
    dataset_id: str
    dataset_version: str
    category: str
    prompt_id: str
    prompt_text: str
    attack_type: str
    severity: str
    agent_response: str
    response_latency_ms: int
    session_id: str
    turn_number: int
    judgment: str
    judgment_confidence: float
    judge_model: str
    judgment_reasoning: str
    failure_type: str | None = None
    regulatory_flags: list[str] = field(default_factory=list)
    attack_success: str = "none"
    owasp_asi_threat: str | None = None
    maestro_threat: str | None = None
    agentic_attack_vector: str | None = None
    tool_context: list[str] = field(default_factory=list)
    regulatory_mapping: list[str] = field(default_factory=list)
    prompt_metadata: str = "{}"
    timestamp: datetime = field(default_factory=datetime.utcnow)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for storage."""
        return {
            "run_id": self.run_id,
            "agent_id": self.agent_id,
            "dataset_id": self.dataset_id,
            "dataset_version": self.dataset_version,
            "category": self.category,
            "prompt_id": self.prompt_id,
            "prompt_text": self.prompt_text,
            "prompt_metadata": self.prompt_metadata,
            "attack_type": self.attack_type,
            "severity": self.severity,
            "agent_response": self.agent_response,
            "response_latency_ms": self.response_latency_ms,
            "session_id": self.session_id,
            "turn_number": self.turn_number,
            "judgment": self.judgment,
            "judgment_confidence": self.judgment_confidence,
            "judge_model": self.judge_model,
            "judgment_reasoning": self.judgment_reasoning,
            "failure_type": self.failure_type,
            "regulatory_flags": self.regulatory_flags,
            "attack_success": self.attack_success,
            "owasp_asi_threat": self.owasp_asi_threat,
            "maestro_threat": self.maestro_threat,
            "agentic_attack_vector": self.agentic_attack_vector,
            "tool_context": self.tool_context,
            "regulatory_mapping": self.regulatory_mapping,
            "timestamp": self.timestamp,
        }


@dataclass
class EvalSession:
    """A session record for analytics storage."""

    run_id: str
    agent_id: str
    session_id: str
    session_status: str
    prompts_executed: int
    prompts_passed: int
    prompts_failed: int
    started_at: datetime
    ended_at: datetime | None = None
    reset_reason: str | None = None
    error_message: str | None = None
    timestamp: datetime = field(default_factory=datetime.utcnow)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for storage."""
        return {
            "run_id": self.run_id,
            "agent_id": self.agent_id,
            "session_id": self.session_id,
            "session_status": self.session_status,
            "prompts_executed": self.prompts_executed,
            "prompts_passed": self.prompts_passed,
            "prompts_failed": self.prompts_failed,
            "started_at": self.started_at,
            "ended_at": self.ended_at,
            "reset_reason": self.reset_reason,
            "error_message": self.error_message,
            "timestamp": self.timestamp,
        }


class AnalyticsStorage(ABC):
    """Abstract base class for analytics storage backends.

    Implementations should handle:
    - Batching writes for efficiency
    - Graceful degradation if storage is unavailable
    - Schema creation/migration
    """

    @abstractmethod
    async def connect(self) -> None:
        """Establish connection to the storage backend."""
        pass

    @abstractmethod
    async def close(self) -> None:
        """Close connection and flush any pending data."""
        pass

    @abstractmethod
    def is_connected(self) -> bool:
        """Check if storage is connected and available."""
        pass

    @abstractmethod
    def add_eval_detail(self, detail: EvalDetail) -> None:
        """Add an evaluation detail to the batch.

        Args:
            detail: The evaluation detail to store
        """
        pass

    @abstractmethod
    def add_session(self, session: EvalSession) -> None:
        """Add a session record to the batch.

        Args:
            session: The session record to store
        """
        pass

    @abstractmethod
    def flush(self) -> None:
        """Flush all pending batches to storage."""
        pass
