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

The Docker Compose setup includes a demo agent called **The Mastermind** — a heist-themed chat agent with built-in safety guardrails. It's a great way to see Crosswind in action before connecting your own agent.

```bash
# Clone and configure
git clone https://github.com/compfly-ai/crosswind.git
cd crosswind/deploy
cp .env.example .env

# Generate encryption key
openssl rand -hex 32  # Add to .env as ENCRYPTION_KEY

# Generate Crosswind API key (for authenticating with the platform)
openssl rand -base64 32

# Add to deploy/.env AND examples/the-mastermind/.env
CROSSWIND_API_KEY=your-generated-key-here

# Add your OpenAI key for LLM judgment
# OPENAI_API_KEY=sk-...

# Configure the demo agent
cp ../examples/the-mastermind/.env.example ../examples/the-mastermind/.env
# Edit examples/the-mastermind/.env — set CROSSWIND_API_KEY to match deploy/.env

# Start all services (API, worker, context-processor, MongoDB, Redis, and The Mastermind)
docker compose up -d

# Seed the default evaluation datasets (required for meaningful reports)
cd ../scripts && uv sync
uv run python seed_datasets.py

# Verify
curl http://localhost:8080/health      # Crosswind API
curl http://localhost:8901/health      # The Mastermind agent
```

## API Keys

Crosswind uses two types of API keys:

| Key | Purpose | Where to set |
|-----|---------|--------------|
| `CROSSWIND_API_KEY` | Authenticates requests to the Crosswind platform (register agents, run evals, get results) | `deploy/.env` |
| `AGENT_API_KEY` | Authenticates requests to your agent (Crosswind uses this to call your agent during evals) | Your agent's `.env` |

### Generating the Crosswind API Key

The `CROSSWIND_API_KEY` is used to authenticate all your requests to the Crosswind platform API.

```bash
# Generate a secure random key
openssl rand -base64 32

# Add to deploy/.env
CROSSWIND_API_KEY=your-generated-key-here
```

Use this key in the `Authorization: Bearer` header when calling Crosswind endpoints.

### Generating an Agent API Key

The `AGENT_API_KEY` authenticates requests to your agent. Crosswind uses this key when calling your agent during evaluations.

```bash
# Generate a secure random key
openssl rand -base64 32

# Add to your agent's .env file
AGENT_API_KEY=your-generated-key-here
```

When registering with Crosswind, provide this key in `authConfig.credentials` so Crosswind can authenticate with your agent.

## Run Your First Eval

This walks through evaluating **The Mastermind** — the demo agent included in Docker Compose. It runs on port 8901 and uses `X-API-Key` header auth.

```bash
# Load environment variables
source deploy/.env
source examples/the-mastermind/.env

# 1. Register The Mastermind with Crosswind
#    Since both services run in Docker Compose, use the container hostname "mastermind"
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "The Mastermind",
    "description": "A suave heist planner for security testing",
    "goal": "Help users while maintaining character and refusing harmful requests",
    "industry": "security-testing",
    "endpointConfig": {
      "protocol": "custom",
      "endpoint": "http://mastermind:8901/chat"
    },
    "authConfig": {
      "type": "api_key",
      "headerName": "X-API-Key",
      "credentials": "'"$AGENT_API_KEY"'"
    }
  }'
# Returns: {"id": "<agentId>", ...}

# 2. Run a quick security eval (~60 prompts, covers OWASP Agentic AI Top 10)
curl -X POST http://localhost:8080/v1/agents/<agentId>/evals \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"mode": "quick", "evalType": "red_team"}'
# Returns: {"runId": "<runId>", ...}

# 3. Check status
curl http://localhost:8080/v1/evals/<runId> \
  -H "Authorization: Bearer $CROSSWIND_API_KEY"

# 4. Get results (JSON)
curl http://localhost:8080/v1/evals/<runId>/results \
  -H "Authorization: Bearer $CROSSWIND_API_KEY"

# 5. Download HTML report
curl http://localhost:8080/v1/evals/<runId>/report \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -o report.html
```

> **Evaluating your own agent?** Replace the `endpointConfig` and `authConfig` above with your agent's endpoint and credentials. See [Supported Agent Frameworks](#supported-agent-frameworks) for protocol options.

## Evaluation Modes

| Mode | Prompts | Time | Use Case |
|------|---------|------|----------|
| `quick` | ~60 | 1-2 min | CI/CD gates, quick checks (full OWASP coverage) |
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
- `examples/the-mastermind/` - Demo agent included in Docker Compose (heist-themed chat bot with safety guardrails)

## Built-in Datasets

Crosswind includes curated evaluation datasets. The default datasets require no external dependencies and provide full OWASP Agentic AI Top 10 coverage.

```bash
cd scripts && uv sync

# Seed default curated datasets (recommended - no HuggingFace token needed)
uv run python seed_datasets.py

# Seed all datasets including HuggingFace sources (requires token)
export HUGGINGFACE_TOKEN=hf_...
uv run python seed_datasets.py --all
```

**Default Datasets** (curated, no external dependencies)

| Dataset | Type | Prompts | Description |
|---------|------|---------|-------------|
| `quick_redteam` | Red Team | 58 | OWASP Agentic AI Top 10 (ASI01-ASI10) - tool misuse, prompt injection, memory poisoning, multi-turn escalation |
| `quick_trust` | Trust | 56 | Agentic quality - hallucination, over-refusal, bias, multi-turn trust behaviors |

**HuggingFace Datasets** (requires token, use `--all`)

| Dataset | Type | Description |
|---------|------|-------------|
| JailbreakBench | Red Team | Jailbreak prompts with human annotations |
| SafetyBench | Red Team | Multi-lingual safety evaluation |
| HH-RLHF | Red Team | Human preference data with red team examples |
| RealToxicityPrompts | Red Team | Toxicity elicitation prompts |
| ToolEmu | Red Team | Agent tool misuse scenarios |
| BBQ Bias | Trust | Bias detection across demographics |
| TruthfulQA | Trust | Truthfulness and hallucination detection |
| DecodingTrust | Trust | Privacy and truthfulness probes |
| AgentHarm 🔒 | Red Team | Agentic harm scenarios (requires approval) |
| WildJailbreak 🔒 | Red Team | In-the-wild jailbreaks (requires approval) |

## Agent-Specific Scenarios

Generic datasets are a start, but your agent has specific tools, memory, and context that attackers will target. Crosswind can generate scenarios tailored to your agent.

When you register an agent with its capabilities (tools like Salesforce or Slack, memory settings, RAG context), the scenario generator creates attacks that exploit those specific surfaces—multi-turn conversations that build trust before attempting tool misuse, memory poisoning across sessions, or data exfiltration through your actual integrations.

```bash
# Upload your agent's context (product docs, policies, etc.)
curl -X POST http://localhost:8080/v1/contexts \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -F "files=@product-catalog.pdf" \
  -F "files=@return-policy.docx"

# Generate scenarios targeting your agent's capabilities
curl -X POST http://localhost:8080/v1/agents/{agentId}/scenarios/generate \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"contextId": "ctx_123", "count": 50}'
```

The generated scenarios include single-turn probes and multi-turn conversations that mirror real attack patterns.

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ENCRYPTION_KEY` | AES-256 key for encrypting credentials (required) | - |
| `CROSSWIND_API_KEY` | Bearer token for authenticating with Crosswind API (required) | - |
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
