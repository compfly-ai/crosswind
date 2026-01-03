"""Shared test utilities for protocol adapter testing.

Provides reusable factories and fixtures for creating test data.
"""

from typing import Any
from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4

from crosswind.models import (
    AuthConfig,
    ConversationRequest,
    ConversationResponse,
    EvalType,
    ExpectedBehavior,
    Judgment,
    JudgmentMode,
    JudgmentResult,
    Message,
    Prompt,
    Severity,
)


# =============================================================================
# Async Helpers
# =============================================================================


class AsyncIteratorMock:
    """Mock async iterator for testing."""

    def __init__(self, items: list[Any]):
        self.items = items
        self.index = 0

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self.index >= len(self.items):
            raise StopAsyncIteration
        item = self.items[self.index]
        self.index += 1
        return item


# =============================================================================
# Data Factories
# =============================================================================


def create_sample_prompt(**kwargs) -> Prompt:
    """Create a sample Prompt for testing."""
    defaults = {
        "prompt_id": "test-prompt-1",
        "dataset_id": "test-dataset",
        "version": "1.0.0",
        "content": "Test prompt content",
        "expected_behavior": ExpectedBehavior.REFUSE,
        "attack_type": "test",
        "severity": Severity.HIGH,
        "category": "test",
        "eval_type": EvalType.RED_TEAM,
        "judgment_mode": JudgmentMode.SAFETY,
    }
    defaults.update(kwargs)
    return Prompt(**defaults)


def create_sample_request(
    content: str = "Hello",
    session_id: str | None = None,
) -> ConversationRequest:
    """Create a sample ConversationRequest for testing."""
    return ConversationRequest(
        messages=[Message(role="user", content=content)],
        session_id=session_id or str(uuid4()),
    )


def create_sample_response(
    content: str = "Test response",
    session_id: str = "test-session",
    latency_ms: int = 100,
) -> ConversationResponse:
    """Create a sample ConversationResponse for testing."""
    return ConversationResponse(
        content=content,
        session_id=session_id,
        latency_ms=latency_ms,
    )


def create_sample_judgment(
    result: JudgmentResult = JudgmentResult.PASS,
    confidence: float = 0.95,
    reasoning: str = "Test judgment",
) -> Judgment:
    """Create a sample Judgment for testing."""
    return Judgment(
        result=result,
        confidence=confidence,
        reasoning=reasoning,
        judge_model="test",
    )


def create_agent_config(protocol: str = "custom", **kwargs) -> dict[str, Any]:
    """Create a sample agent configuration.

    Args:
        protocol: a2a, mcp, custom, openapi_http
        **kwargs: Override any field (endpoint, agent_card_url, auth_type, etc.)
    """
    config = {
        "agentId": kwargs.get("agent_id", "test-agent"),
        "name": kwargs.get("name", "Test Agent"),
        "endpointConfig": {"protocol": protocol},
        "authConfig": {
            "type": kwargs.get("auth_type", "bearer"),
            "credentials": kwargs.get("credentials", ""),
            "headerName": kwargs.get("header_name", "Authorization"),
        },
        "rateLimits": {"requestsPerMinute": 60},
    }

    # Protocol-specific fields
    if protocol in ("custom", "openapi_http"):
        config["endpointConfig"]["endpoint"] = kwargs.get(
            "endpoint", "http://localhost:8000/chat"
        )
    elif protocol == "a2a":
        config["endpointConfig"]["agentCardUrl"] = kwargs.get(
            "agent_card_url", "http://localhost:8903/.well-known/agent.json"
        )
    elif protocol == "mcp":
        config["endpointConfig"]["endpoint"] = kwargs.get(
            "endpoint", "http://localhost:8000/mcp"
        )
        config["endpointConfig"]["mcpTransport"] = kwargs.get(
            "mcp_transport", "streamable_http"
        )
        config["endpointConfig"]["mcpToolName"] = kwargs.get("mcp_tool_name", "chat")

    return config


def create_auth_config(**kwargs) -> AuthConfig:
    """Create a sample AuthConfig for testing."""
    defaults = {
        "type": "bearer",
        "credentials": "test-token",
        "header_name": "Authorization",
        "header_prefix": "Bearer ",
    }
    defaults.update(kwargs)
    return AuthConfig(**defaults)


# =============================================================================
# Mock Fixtures
# =============================================================================


def create_mock_db() -> MagicMock:
    """Create a mock MongoDB database."""
    db = MagicMock()
    db.evalRuns = MagicMock()
    db.evalRuns.find_one = AsyncMock(return_value={"status": "running"})
    db.evalRuns.update_one = AsyncMock()
    db.evalResultsSummary = MagicMock()
    db.evalResultsSummary.find_one = AsyncMock(return_value={"samplePasses": []})
    db.evalResultsSummary.update_one = AsyncMock()
    db.datasets = MagicMock()
    db.datasets.find = MagicMock(return_value=AsyncIteratorMock([]))
    db.scenarioSets = MagicMock()
    db.scenarioSets.find_one = AsyncMock(return_value=None)
    return db


def create_mock_redis() -> MagicMock:
    """Create a mock Redis client."""
    return MagicMock()


def create_mock_adapter(
    response: ConversationResponse | None = None,
    error: Exception | None = None,
) -> MagicMock:
    """Create a mock protocol adapter."""
    adapter = MagicMock()

    if error:
        adapter.send_message = AsyncMock(side_effect=error)
    else:
        adapter.send_message = AsyncMock(return_value=response or create_sample_response())

    adapter.create_session = AsyncMock(return_value=str(uuid4()))
    adapter.close_session = AsyncMock()
    adapter.cleanup = AsyncMock()
    adapter.health_check = AsyncMock(return_value=True)

    return adapter
