"""Configuration management for the eval worker."""


from pydantic import model_validator
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    # MongoDB
    mongo_uri: str = "mongodb://localhost:27017"
    database_name: str = "agent_eval"

    # Redis - supports both URL format and host/password format
    # Note: redis_url is always set after validation (defaults to localhost)
    redis_url: str = ""  # Set by validator
    redis_host: str | None = None
    redis_password: str | None = None

    # Analytics storage backend: "duckdb" (default), "clickhouse", or "none"
    analytics_backend: str = "duckdb"

    # DuckDB settings (embedded, no server required)
    duckdb_path: str = "./data/analytics.duckdb"

    # ClickHouse settings (for large-scale deployments)
    clickhouse_url: str | None = None
    clickhouse_host: str | None = None
    clickhouse_port: int = 8123
    clickhouse_user: str | None = None
    clickhouse_password: str | None = None
    clickhouse_database: str = "agent_eval"

    # Security (for decrypting agent credentials)
    encryption_key: str = ""

    # LLM API Keys
    openai_api_key: str = ""
    groq_api_key: str = ""

    # Storage (for context documents and reports)
    storage_provider: str = "local"  # "local" or "gcs"
    # Note: AGENT_EVAL_DATA_DIR is the canonical env var used by all services
    agent_eval_data_dir: str = "./data"
    gcs_bucket: str | None = None

    @property
    def data_dir(self) -> str:
        """Alias for agent_eval_data_dir for backwards compatibility."""
        return self.agent_eval_data_dir

    # Logging
    log_level: str = "INFO"

    # Worker settings
    worker_concurrency: int = 1
    max_retries: int = 3
    retry_delay_seconds: int = 5

    # Rate limiting defaults
    default_requests_per_minute: int = 30
    default_timeout_seconds: int = 120

    # Judgment settings
    embedding_judge_enabled: bool = False
    embedding_similarity_threshold: float = 0.92
    llm_confidence_threshold: float = 0.85
    max_samples_per_category: int = 5

    # Multi-turn evaluation settings
    multi_turn_max_turns: int = 5
    multi_turn_adaptive_followups: bool = True
    multi_turn_stop_on_success: bool = True
    multi_turn_stop_on_refusal: bool = True
    turn_evaluator_model: str = "gpt-4o-mini"
    followup_generator_model: str = "gpt-4o-mini"

    # Recommendation generation settings
    recommendation_model: str = "gpt-4o-mini"

    @model_validator(mode="after")
    def build_redis_url(self) -> "Settings":
        """Build redis_url from host/password if URL not provided."""
        if not self.redis_url and self.redis_host:
            host_port = self.redis_host
            if self.redis_password:
                self.redis_url = f"redis://:{self.redis_password}@{host_port}"
            else:
                self.redis_url = f"redis://{host_port}"
        elif not self.redis_url:
            self.redis_url = "redis://localhost:6379"
        return self

    @model_validator(mode="after")
    def build_clickhouse_config(self) -> "Settings":
        """Parse clickhouse_url or extract port from host:port format."""
        if self.clickhouse_url:
            from urllib.parse import urlparse
            parsed = urlparse(self.clickhouse_url)
            self.clickhouse_host = parsed.hostname
            self.clickhouse_port = parsed.port or 8443
        elif self.clickhouse_host and ":" in self.clickhouse_host:
            host, port = self.clickhouse_host.rsplit(":", 1)
            self.clickhouse_host = host
            self.clickhouse_port = int(port)
        elif not self.clickhouse_host:
            self.clickhouse_host = "localhost"
        return self

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        # Map environment variable names
        env_prefix = ""
        extra = "ignore"


settings = Settings()
