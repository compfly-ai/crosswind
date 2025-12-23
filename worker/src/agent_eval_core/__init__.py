"""Agent-Eval Core - Evaluation engine for AI agent security testing.

This package provides the core evaluation engine. It includes:
- Protocol adapters for communicating with agents (HTTP, WebSocket)
- Judgment pipeline for evaluating agent responses
- Session management for multi-turn conversations
- Rate limiting and circuit breaker patterns
- Storage backends for analytics (DuckDB, ClickHouse)

Usage:
    from agent_eval_core import EvalRunner, JudgmentPipeline
    from agent_eval_core.protocols import create_adapter
    from agent_eval_core.config import settings

The package is designed to be extended:
- Add custom storage backends
- Implement custom recommendation generators
"""

__version__ = "0.1.0"

# Core evaluation engine
from agent_eval_core.evaluation.runner import EvalRunner
from agent_eval_core.evaluation.session import SessionManager
from agent_eval_core.evaluation.rate_limiter import RateLimiter

# Judgment pipeline
from agent_eval_core.judgment import JudgmentPipeline, TurnEvaluator

# Protocol adapters
from agent_eval_core.protocols import create_adapter, ProtocolAdapter, HTTPAgentError

# Models
from agent_eval_core.models import (
    Prompt,
    PromptResult,
    Judgment,
    JudgmentResult,
    Message,
    ConversationRequest,
    ConversationResponse,
    AgentCapabilities,
    MultiTurnConfig,
)

# Storage
from agent_eval_core.storage import AnalyticsStorage, create_storage

# Config
from agent_eval_core.config import settings, Settings

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
