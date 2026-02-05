# Crosswind Development Guide

## Project Overview

Crosswind is an open-source security evaluation platform for AI agents. It tests agents against adversarial prompts to identify safety vulnerabilities before deployment.

## Architecture

```
crosswind/
├── api/                    # Go API server
│   ├── cmd/server/         # Entry point
│   ├── internal/
│   │   ├── config/         # Configuration
│   │   ├── handlers/       # HTTP handlers
│   │   ├── middleware/     # Auth, logging, CORS
│   │   ├── models/         # Data models
│   │   ├── repository/     # MongoDB, ClickHouse
│   │   ├── queue/          # Redis job queue
│   │   └── services/       # Business logic
│   └── pkg/
│       ├── api/            # Public exports
│       ├── crypto/         # Encryption utilities
│       ├── repository/     # Repository interfaces
│       └── storage/        # File storage abstraction
├── worker/                 # Python evaluation worker
│   └── src/crosswind/
│       ├── evaluation/     # Test runner, rate limiting
│       ├── judgment/       # LLM-as-judge pipeline
│       ├── protocols/      # Agent adapters (HTTP, WS)
│       ├── models/         # Pydantic schemas
│       └── storage/        # Analytics backends
├── context-processor/      # Python document processor
│   └── src/crosswind_context/
│       ├── context/        # Text extraction
│       └── storage/        # File storage
├── cli/                    # Go CLI (planned)
└── deploy/                 # Docker Compose files
```

## Build Commands

```bash
# API (Go 1.24+)
cd api && go build ./...

# Worker (Python 3.11+)
cd worker && uv sync && uv run python -m py_compile src/crosswind/__init__.py

# Context Processor
cd context-processor && uv sync
```

## Key Services

### API (Go)

| Service | File | Description |
|---------|------|-------------|
| `AgentService` | `services/agent_service.go` | Agent CRUD, API analysis |
| `EvalService` | `services/eval_service.go` | Evaluation runs, results |
| `ScenarioService` | `services/scenario_service.go` | Custom scenario generation |
| `DatasetService` | `services/dataset_service.go` | Built-in datasets |
| `ContextService` | `services/context_service.go` | Document uploads |
| `AnalyticsService` | `services/analytics_service.go` | ClickHouse queries |
| `APIAnalyzer` | `services/api_analyzer.go` | GPT-powered API inference |

### Worker (Python)

| Module | Description |
|--------|-------------|
| `main` | Worker loop: per-agent queues, reliable delivery, heartbeat |
| `evaluation.runner` | Eval engine: checkpoint/resume, concurrent execution |
| `evaluation.session` | Multi-turn conversation management |
| `judgment.pipeline` | LLM-as-judge evaluation |
| `protocols.openapi_http` | HTTP agent adapter |
| `protocols.a2a_adapter` | A2A protocol adapter |
| `protocols.mcp_adapter` | MCP protocol adapter |
| `storage.duckdb_storage` | Local analytics storage |
| `storage.clickhouse_storage` | Cloud analytics storage |

### Eval Worker Architecture

The worker uses a reliable queue pattern with per-agent isolation:

**Redis Keys**:
- `eval_jobs:{agentId}` — Per-agent job queue
- `eval_agents` — SET of agents with pending jobs
- `eval_lock:{agentId}` — Per-agent execution lock
- `eval_processing:{workerId}` — In-flight jobs for crash recovery
- `eval_heartbeat:{workerId}` — Worker liveness (30s TTL)

**Concurrency**: `WORKER_CONCURRENCY` (default 3) controls parallel evals via asyncio.Semaphore.

**Checkpoint/Resume**: Every `CHECKPOINT_INTERVAL` (default 10) prompts, completed IDs and progress counters are flushed to MongoDB. On crash recovery, state is restored from the checkpoint snapshot and non-checkpointed results are filtered out.

## Environment Variables

```bash
# Required
ENCRYPTION_KEY=<32-byte-hex>      # AES-256 for credential encryption
API_KEY=<your-api-key>            # Bearer token for API auth
OPENAI_API_KEY=<key>              # For judgment (or GROQ_API_KEY)

# Database
MONGO_URI=mongodb://localhost:27017
DATABASE_NAME=crosswind
REDIS_URL=redis://localhost:6379

# Worker
WORKER_CONCURRENCY=3              # Max concurrent evals per worker
CHECKPOINT_INTERVAL=10            # Flush checkpoint every N prompts

# Storage
STORAGE_PROVIDER=local            # "local" or "gcs"
CROSSWIND_DATA_DIR=./data         # Local storage path
GCS_BUCKET_NAME=                  # Required if STORAGE_PROVIDER=gcs

# Analytics (optional)
ANALYTICS_BACKEND=duckdb          # "duckdb", "clickhouse", or "none"
CLICKHOUSE_HOST=                  # e.g., host:9440
CLICKHOUSE_DATABASE=crosswind
CLICKHOUSE_USER=default
CLICKHOUSE_PASSWORD=
```

## API Endpoints

All routes use `/v1/orgs/{orgId}/...` pattern:

| Method | Path | Description |
|--------|------|-------------|
| POST | `/agents` | Register agent |
| GET | `/agents` | List agents |
| GET | `/agents/:agentId` | Get agent |
| PATCH | `/agents/:agentId` | Update agent |
| DELETE | `/agents/:agentId` | Delete agent |
| POST | `/agents/:agentId/analyze` | Trigger API analysis |
| POST | `/agents/:agentId/evals` | Start evaluation |
| GET | `/evals/:runId` | Get eval status |
| GET | `/evals/:runId/results` | Get eval results |
| POST | `/agents/:agentId/scenarios/generate` | Generate scenarios |
| GET | `/agents/:agentId/scenarios/:setId/stream` | SSE progress |
| POST | `/contexts` | Upload documents |
| GET | `/datasets` | List datasets |

## Protocol Adapters

| Protocol | Description | Required Fields |
|----------|-------------|-----------------|
| `openai` | OpenAI APIs | `promptId` or `assistantId` |
| `azure_openai` | Azure OpenAI | `baseUrl`, `promptId` or `assistantId` |
| `langgraph` | LangGraph Cloud | `baseUrl` |
| `bedrock` | AWS Bedrock | `agentId` |
| `vertex` | Vertex AI | `projectId`, `reasoningEngineId` |
| `custom` | HTTP endpoint | `endpoint` |
| `custom_ws` | WebSocket | `endpoint` |

## Key Patterns

### Repository Interface Pattern

Services use repository interfaces for data access:

```go
// pkg/repository/interfaces.go
type AgentRepository interface {
    Create(ctx context.Context, agent *models.Agent) error
    FindByID(ctx context.Context, agentID string) (*models.Agent, error)
    // ...
}
```

### Storage Abstraction

File storage supports local and GCS backends:

```go
// pkg/storage/interfaces.go
type FileStorage interface {
    Upload(ctx context.Context, path string, reader io.Reader, contentType string) error
    Download(ctx context.Context, path string) (io.ReadCloser, error)
    Delete(ctx context.Context, path string) error
}
```

### Analytics Backends

Worker supports DuckDB (local) or ClickHouse (production):

```python
# storage/factory.py
async def create_storage() -> AnalyticsStorage:
    backend = settings.analytics_backend
    if backend == "duckdb":
        return DuckDBStorage()
    elif backend == "clickhouse":
        return ClickHouseStorage()
    return NullStorage()
```

## Testing

```bash
# API
cd api && go test ./...

# Worker
cd worker && uv run pytest

# Context Processor
cd context-processor && uv run pytest
```

## Commit Guidelines

- Keep commit messages short and descriptive
- Use conventional commits when possible (feat:, fix:, docs:, etc.)

## Common Tasks

### Add a New Protocol Adapter

1. Create adapter in `worker/src/crosswind/protocols/`
2. Implement `ProtocolAdapter` base class
3. Register in `protocols/__init__.py`
4. Add protocol constant in `api/internal/models/agent.go`

### Add a New Dataset

1. Add dataset metadata to `scripts/seed_datasets.py`
2. Run seeding script
3. Dataset will be available via API

### Add a New Judgment Method

1. Create evaluator in `worker/src/crosswind/judgment/`
2. Implement scoring logic
3. Add to judgment pipeline in `judgment/pipeline.py`
