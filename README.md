# Crosswind

Security evaluation for AI agents—not just LLMs, but the full agentic system with tools, memory, and multi-turn conversations.

Crosswind tests your agent against adversarial scenarios: prompt injections that target tool calls, jailbreaks that exploit conversation context, and attacks that chain across multiple turns. You get a report showing exactly where your agent's defenses break down.

## What You Can Test

**Red Team (Security)**
- Prompt injection - Does your agent execute injected instructions from untrusted data?
- Tool misuse - Can attackers trick your agent into harmful tool calls?
- Jailbreaks - Can users bypass your system prompt through conversation?
- Data exfiltration - Will your agent leak sensitive context or tool outputs?

**Trust (Compliance)**
- Hallucination - Does your agent fabricate facts or tool results?
- Bias - Are agent decisions fair across demographics?
- Over-refusal - Does your agent block legitimate actions?
- Policy adherence - Does your agent follow your operational guidelines?

Both evaluation types map to regulatory frameworks (EU AI Act, NIST AI RMF) for compliance reporting.

## Quick Start

```bash
# Clone and configure
git clone https://github.com/compfly-ai/crosswind.git
cd crosswind/deploy
cp .env.example .env

# Generate encryption key
openssl rand -hex 32  # Add to .env as ENCRYPTION_KEY

# Add your OpenAI key for LLM judgment
# OPENAI_API_KEY=sk-...

# Start services
docker compose up -d

# Verify
curl http://localhost:8080/health
```

## Run Your First Eval

```bash
export API_KEY="your-api-key"  # From .env

# 1. Register your agent
curl -X POST http://localhost:8080/v1/orgs/default/agents \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Support Bot",
    "description": "Customer support agent",
    "goal": "Help customers with product questions",
    "industry": "ecommerce",
    "endpointConfig": {
      "protocol": "custom",
      "endpoint": "https://your-agent.com/chat"
    },
    "authConfig": {
      "type": "bearer",
      "credentials": "your-agent-api-key"
    }
  }'
# Returns: {"id": "agent_abc123", ...}

# 2. Run a quick security eval (~200 prompts)
curl -X POST http://localhost:8080/v1/orgs/default/agents/agent_abc123/evals \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"mode": "quick", "evalType": "red_team"}'
# Returns: {"runId": "run_xyz789", ...}

# 3. Check results
curl http://localhost:8080/v1/orgs/default/evals/run_xyz789/results \
  -H "Authorization: Bearer $API_KEY"
```

## Evaluation Modes

| Mode | Prompts | Time | Use Case |
|------|---------|------|----------|
| `quick` | ~200 | 2-5 min | CI/CD gates, quick checks |
| `standard` | ~2,000 | 15-30 min | Regular testing |
| `deep` | ~10,000 | 1-2 hrs | Pre-release audits |

## Supported Agent Frameworks

Works with any agent that exposes an HTTP or WebSocket endpoint:

| Protocol | Agent Framework |
|----------|-----------------|
| `openai` | OpenAI Assistants API |
| `azure_openai` | Azure OpenAI Agents |
| `langgraph` | LangGraph Cloud agents |
| `bedrock` | AWS Bedrock Agents |
| `vertex` | Vertex AI Agents (Reasoning Engines) |
| `custom` | Any agentic HTTP endpoint |
| `custom_ws` | Any agentic WebSocket endpoint |

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Crosswind API  │────▶│      Redis      │────▶│   Eval Worker   │
│      (Go)       │     │     (Queue)     │     │    (Python)     │
└────────┬────────┘     └─────────────────┘     └────────┬────────┘
         │                                               │
         ▼                                               ▼
┌─────────────────┐                            ┌─────────────────┐
│    MongoDB      │                            │   Your Agent    │
│                 │                            │   (HTTP/WS)     │
└─────────────────┘                            └─────────────────┘
```

**Components:**
- `api/` - Go REST API, job orchestration
- `worker/` - Python eval runner with multi-turn session support
- `context-processor/` - Document extraction for agent-specific scenarios

## Custom Scenarios

Upload your agent's context documents to generate targeted attack scenarios:

```bash
# Upload product docs, policies, etc.
curl -X POST http://localhost:8080/v1/orgs/default/contexts \
  -H "Authorization: Bearer $API_KEY" \
  -F "files=@product-catalog.pdf" \
  -F "files=@return-policy.docx"

# Generate scenarios from context
curl -X POST http://localhost:8080/v1/orgs/default/agents/{agentId}/scenarios/generate \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"contextId": "ctx_123", "count": 50}'
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ENCRYPTION_KEY` | AES-256 key (required) | - |
| `API_KEY` | Bearer token for API auth | - |
| `OPENAI_API_KEY` | For LLM judgment | - |
| `GROQ_API_KEY` | Alternative to OpenAI | - |
| `ANALYTICS_BACKEND` | `duckdb`, `clickhouse`, `none` | `duckdb` |

## Local Development

```bash
# API (Go 1.24+)
cd api && go run ./cmd/server

# Worker (Python 3.11+)
cd worker && uv sync && uv run python -m crosswind.main

# Run tests
cd api && go test ./...
cd worker && uv run pytest
```

## Acknowledgements

Crosswind is built on these excellent open-source projects:

**Infrastructure**
- [MongoDB](https://www.mongodb.com/) - Document database
- [Redis](https://redis.io/) - Job queue and caching
- [DuckDB](https://duckdb.org/) - Embedded analytics database
- [ClickHouse](https://clickhouse.com/) - Columnar analytics (optional)

**API (Go)**
- [Gin](https://gin-gonic.com/) - HTTP framework
- [mongo-driver](https://github.com/mongodb/mongo-go-driver) - MongoDB client
- [go-redis](https://github.com/redis/go-redis) - Redis client
- [Zap](https://github.com/uber-go/zap) - Structured logging

**Worker (Python)**
- [OpenAI SDK](https://github.com/openai/openai-python) - LLM-as-judge
- [Pydantic](https://docs.pydantic.dev/) - Data validation
- [httpx](https://www.python-httpx.org/) - HTTP client
- [structlog](https://www.structlog.org/) - Structured logging
- [Motor](https://motor.readthedocs.io/) - Async MongoDB driver

**Document Processing**
- [Docling](https://github.com/DS4SD/docling) - PDF/document extraction

## License

Apache 2.0 - See [LICENSE](LICENSE)
