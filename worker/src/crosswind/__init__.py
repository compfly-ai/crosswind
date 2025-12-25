"""Agent-Eval Core - Evaluation engine for AI agent security testing.

This package provides the core evaluation engine. It includes:
- Protocol adapters for communicating with agents (HTTP, WebSocket)
- Judgment pipeline for evaluating agent responses
- Session management for multi-turn conversations
- Rate limiting and circuit breaker patterns
- Storage backends for analytics (DuckDB, ClickHouse)

Usage:
    from crosswind import EvalRunner, JudgmentPipeline
    from crosswind.protocols import create_adapter
    from crosswind.config import settings

The package is designed to be extended:
- Add custom storage backends
- Implement custom recommendation generators
"""

__version__ = "0.1.0"

# Core evaluation engine
# Config
from crosswind.config import Settings, settings
from crosswind.evaluation.rate_limiter import RateLimiter
from crosswind.evaluation.runner import EvalRunner
from crosswind.evaluation.session import SessionManager

# Judgment pipeline
from crosswind.judgment import JudgmentPipeline, TurnEvaluator

# Models
from crosswind.models import (
    AgentCapabilities,
    ConversationRequest,
    ConversationResponse,
    Judgment,
    JudgmentResult,
    Message,
    MultiTurnConfig,
    Prompt,
    PromptResult,
)

# Protocol adapters
from crosswind.protocols import HTTPAgentError, ProtocolAdapter, create_adapter

# Storage
from crosswind.storage import AnalyticsStorage, create_storage

__all__ = [
    # Version
    "__version__",
    # Evaluation
    "EvalRunner",
    "SessionManager",
    "RateLimiter",
    # Judgment
    "JudgmentPipeline",
    "TurnEvaluator",
    # Protocols
    "create_adapter",
    "ProtocolAdapter",
    "HTTPAgentError",
    # Models
    "Prompt",
    "PromptResult",
    "Judgment",
    "JudgmentResult",
    "Message",
    "ConversationRequest",
    "ConversationResponse",
    "AgentCapabilities",
    "MultiTurnConfig",
    # Storage
    "AnalyticsStorage",
    "create_storage",
    # Config
    "settings",
    "Settings",
]
