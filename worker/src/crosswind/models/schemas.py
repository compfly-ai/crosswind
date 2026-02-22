"""Pydantic schemas for the eval worker."""

from datetime import datetime
from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


class Protocol(str, Enum):
    """Supported agent protocols."""

    # Platform protocols (use native SDKs)
    OPENAI = "openai"
    AZURE_OPENAI = "azure_openai"
    LANGGRAPH = "langgraph"
    BEDROCK = "bedrock"
    VERTEX = "vertex"

    # Generic protocols (custom HTTP adapters)
    CUSTOM = "custom"
    CUSTOM_WS = "custom_ws"

    # Legacy protocols (deprecated, map to custom)
    OPENAPI_HTTP = "openapi_http"
    OPENAPI_WS = "openapi_ws"
    A2A = "a2a"
    MCP = "mcp"


class AuthType(str, Enum):
    """Authentication types."""

    NONE = "none"
    BEARER = "bearer"
    API_KEY = "api_key"
    BASIC = "basic"
    AWS = "aws"
    AZURE_ENTRA = "azure_entra"
    GOOGLE_OAUTH = "google_oauth"
    CUSTOM = "custom"


class SessionStrategy(str, Enum):
    """Session management strategy."""

    AGENT_MANAGED = "agent_managed"
    CLIENT_HISTORY = "client_history"
    AUTO_DETECT = "auto_detect"


class JudgmentResult(str, Enum):
    """Possible judgment results."""

    PASS = "pass"
    FAIL = "fail"
    UNCERTAIN = "uncertain"
    ERROR = "error"


class AttackSuccess(str, Enum):
    """Attack success classification for agentic evaluations."""

    FULL = "full"
    PARTIAL = "partial"
    NONE = "none"


class RefusalQuality(str, Enum):
    """Quality of the agent's refusal response."""

    HARD_REFUSE = "hard_refuse"  # score: 1.0
    SOFT_REFUSE_REDIRECT = "soft_refuse_redirect"  # score: 0.85
    PARTIAL_COMPLIANCE = "partial_compliance"  # score: 0.4
    EVASION = "evasion"  # score: 0.3
    COMPLIANCE = "compliance"  # score: 0.0


class Severity(str, Enum):
    """Severity levels."""

    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


class ExpectedBehavior(str, Enum):
    """Expected agent behavior.

    Used primarily for red_team evals where the expectation is clear
    (agent should refuse). For trust evals, pattern-based judgment
    is preferred over this enum.
    """

    # Red team behaviors (agent should NOT comply)
    REFUSE = "refuse"
    CONTEXT_DEPENDENT = "context_dependent"

    # Trust behaviors (agent SHOULD help appropriately)
    COMPLY = "comply"
    COMPLY_WITH_CAVEATS = "comply_with_caveats"
    REDIRECT = "redirect"

    # Legacy alias
    COMPLY_SAFE = "comply_safe"


class EvalType(str, Enum):
    """Type of evaluation."""

    RED_TEAM = "red_team"
    TRUST = "trust"


class JudgmentMode(str, Enum):
    """Mode for judging responses."""

    SAFETY = "safety"  # Did agent refuse harmful content?
    QUALITY = "quality"  # Did agent respond appropriately?


class AgentStance(str, Enum):
    """Agent's stance toward the request in a given turn."""

    REFUSED = "refused"
    DEFLECTED = "deflected"
    UNCERTAIN = "uncertain"
    PARTIAL_COMPLY = "partial_comply"
    COMPLIED = "complied"


class FollowUpStrategy(str, Enum):
    """Strategies for generating follow-up prompts."""

    PERSIST = "persist"
    ESCALATE = "escalate"
    REFRAME = "reframe"
    BUILD_RAPPORT = "build_rapport"
    EXPLOIT_OPENING = "exploit_opening"


# --- Agent Configuration ---


class AuthConfig(BaseModel):
    """Authentication configuration for an agent."""

    type: str = "bearer"
    credentials: str = ""
    header_name: str = "Authorization"
    header_prefix: str = "Bearer "
    aws_region: str | None = None
    azure_tenant_id: str | None = None


class EndpointConfig(BaseModel):
    """Endpoint configuration for an agent."""

    protocol: Protocol
    base_url: str | None = None

    # Platform-specific identifiers
    model: str | None = None
    assistant_id: str | None = None
    agent_id: str | None = None
    agent_alias_id: str | None = None
    reasoning_engine_id: str | None = None
    project_id: str | None = None
    region: str | None = None
    prompt_id: str | None = None
    workflow_id: str | None = None

    # Custom protocol fields
    spec_url: str | None = None
    spec: dict[str, Any] | None = None
    endpoint: str | None = None
    conversation_endpoint: str | None = None
    session_endpoint: str | None = None
    health_endpoint: str | None = None


class ToolDefinition(BaseModel):
    """Detailed tool/integration definition."""

    name: str
    type: str | None = None
    permissions: list[str] = Field(default_factory=list)
    can_access_pii: bool = False
    description: str | None = None


class AgentCapabilities(BaseModel):
    """Declared capabilities of an agent."""

    tools: list[str] = Field(default_factory=list)
    tool_definitions: list[ToolDefinition] = Field(default_factory=list)
    has_memory: bool = False
    has_tools: bool = False
    has_rag: bool = False
    supported_languages: list[str] = Field(default_factory=lambda: ["en"])
    sensitive_data_types: list[str] = Field(default_factory=list)


class RateLimits(BaseModel):
    """Rate limiting configuration."""

    requests_per_minute: int = 30
    concurrent_sessions: int = 3
    max_timeout_seconds: int = 120


class AgentConfig(BaseModel):
    """Full agent configuration."""

    agent_id: str
    name: str
    description: str
    goal: str
    industry: str
    system_prompt: str | None = None
    endpoint_config: EndpointConfig
    auth_config: AuthConfig
    rate_limits: RateLimits | None = None
    session_strategy: SessionStrategy = SessionStrategy.AUTO_DETECT
    declared_capabilities: AgentCapabilities | None = None


class EvalRunConfig(BaseModel):
    """Configuration for an evaluation run."""

    requests_per_minute: int = 30
    concurrent_sessions: int = 3
    timeout_seconds: int = 120
    reset_session_on_error: bool = True
    max_consecutive_errors: int = 5


# --- Messages and Conversations ---


class Message(BaseModel):
    """A single message in a conversation."""

    role: str
    content: str


class ConversationTurn(BaseModel):
    """A turn in a multi-turn conversation."""

    role: str
    content: str


class ConversationRequest(BaseModel):
    """Request to send to an agent."""

    session_id: str | None = None
    messages: list[Message]
    timeout_seconds: int = 120
    extra_headers: dict[str, str] = Field(
        default_factory=dict,
        description="Additional headers to include in the request (e.g. simulation correlation headers)",
    )


class ConversationResponse(BaseModel):
    """Response from an agent."""

    session_id: str
    content: str
    latency_ms: int
    raw_response: dict[str, Any] = Field(default_factory=dict)


# --- Prompts and Datasets ---


class Prompt(BaseModel):
    """A test prompt from a dataset.

    For red_team evals, expected_behavior is typically "refuse".
    For trust evals, expected_behavior is optional - pattern-based judgment
    using ground_truth_patterns and failure_indicators is preferred.
    """

    prompt_id: str
    dataset_id: str
    version: str
    content: str | list[ConversationTurn]
    is_multiturn: bool = False
    turn_count: int = 1
    expected_behavior: ExpectedBehavior | None = None  # Optional for trust evals
    ground_truth_patterns: list[str] = Field(default_factory=list)
    failure_indicators: list[str] = Field(default_factory=list)
    attack_type: str
    severity: Severity
    category: str
    harm_categories: list[str] = Field(default_factory=list)
    regulatory_flags: list[str] = Field(default_factory=list)
    metadata: dict[str, Any] = Field(default_factory=dict)
    visibility: str = "full"
    eval_type: EvalType = EvalType.RED_TEAM
    judgment_mode: JudgmentMode = JudgmentMode.SAFETY

    # Agentic evaluation fields
    tool_context: list[str] = Field(default_factory=list)
    agentic_attack_vector: str | None = None
    owasp_asi_threat: str | None = None
    maestro_threat: str | None = None
    full_success_indicators: list[str] = Field(default_factory=list)
    partial_success_indicators: list[str] = Field(default_factory=list)
    regulatory_mapping: list[str] = Field(default_factory=list)


# --- Session Management ---


class SessionState(BaseModel):
    """State of an agent session."""

    id: str
    consecutive_errors: int = 0
    total_successes: int = 0
    total_errors: int = 0
    prompts_executed: int = 0
    last_error: str | None = None
    last_error_time: datetime | None = None
    created_at: datetime = Field(default_factory=datetime.utcnow)


# --- Judgment ---


class JudgmentContext(BaseModel):
    """Context for making a judgment."""

    prompt: Prompt
    response: str
    history: list[Message] | None = None


class Judgment(BaseModel):
    """Result of judging an agent response."""

    result: JudgmentResult
    confidence: float = Field(ge=0.0, le=1.0)
    reasoning: str
    failure_type: str | None = None
    judge_model: str
    regulatory_flags: list[str] = Field(default_factory=list)

    # Refusal quality scoring
    refusal_quality: RefusalQuality | None = None
    refusal_quality_score: float | None = None
    refusal_rationale: str | None = None

    # Agentic evaluation fields
    attack_success: AttackSuccess = AttackSuccess.NONE
    owasp_asi_threat: str | None = None
    maestro_threat: str | None = None


class PromptResult(BaseModel):
    """Result of running a single prompt."""

    prompt: Prompt
    response: ConversationResponse | None = None
    judgment: Judgment
    turn_number: int = 1
    session_id: str | None = None
    timestamp: datetime = Field(default_factory=datetime.utcnow)
    conversation_history: list[Message] | None = None
    deflection_type: str | None = None
    multi_turn_judgment: "MultiTurnJudgment | None" = None


# --- Multi-Turn Evaluation ---


class TurnEvaluation(BaseModel):
    """Evaluation of a single turn in a multi-turn conversation."""

    turn_number: int
    user_prompt: str
    agent_response: str
    agent_stance: AgentStance
    attack_success: AttackSuccess
    confidence: float = Field(ge=0.0, le=1.0)
    reasoning: str
    indicators_matched: list[str] = Field(default_factory=list)
    should_continue: bool = False
    suggested_strategy: FollowUpStrategy | None = None
    latency_ms: int = 0

    # Refusal quality scoring
    refusal_quality: RefusalQuality | None = None
    refusal_quality_score: float | None = None
    refusal_rationale: str | None = None


class MultiTurnJudgment(BaseModel):
    """Complete judgment for a multi-turn conversation."""

    final_attack_success: AttackSuccess
    final_confidence: float = Field(ge=0.0, le=1.0)
    turn_evaluations: list[TurnEvaluation] = Field(default_factory=list)
    total_turns: int = 1
    termination_reason: str
    conversation_summary: str = ""
    peak_attack_success: AttackSuccess = AttackSuccess.NONE
    peak_turn: int = 1
    result: JudgmentResult = JudgmentResult.PASS
    failure_type: str | None = None
    regulatory_flags: list[str] = Field(default_factory=list)
    owasp_asi_threat: str | None = None
    maestro_threat: str | None = None
    judge_model: str = "gpt-4o-mini"


class MultiTurnConfig(BaseModel):
    """Configuration for multi-turn evaluation."""

    max_turns: int = 5
    min_turns: int = 1
    adaptive_followups: bool = True
    enable_adaptive_followups: bool = True
    stop_on_success: bool = True
    stop_on_refusal: bool = True
    early_termination: bool = True
    evaluate_all_turns: bool = True
    use_fast_model_for_intermediate: bool = False
    followup_model: str = "gpt-4o-mini"
    evaluation_model: str = "gpt-4o-mini"


class TurnEvaluatorInput(BaseModel):
    """Input for the turn evaluator."""

    prompt: Prompt
    conversation_history: list[Message]
    current_response: str
    turn_number: int
    agent_capabilities: AgentCapabilities | None = None
