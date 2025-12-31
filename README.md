# Crosswind

Security evaluation for AI agents. Test your agentic systems against prompt injection, tool misuse, jailbreaks, and multi-turn attacks before deployment.

## What It Does

Crosswind tests your AI agent by sending adversarial prompts and analyzing responses. It reports where your defenses fail and how to fix them.

**Red Team (Security)**
- Prompt injection via tool outputs
- Tool misuse and unauthorized actions
- Jailbreak attempts
- Data exfiltration
- Multi-turn trust exploitation

**Trust (Quality)**
- Hallucination detection
- Bias testing
- Over-refusal analysis
- Policy adherence

Both evaluation types map to regulatory frameworks (EU AI Act, NIST AI RMF) for compliance reporting.

## Quick Start

```bash
# Clone and configure
git clone https://github.com/compfly-ai/crosswind.git
cd crosswind/deploy
cp .env.example .env

# Generate encryption key and add to .env
openssl rand -hex 32

# Add your OpenAI key for LLM judgment
# OPENAI_API_KEY=sk-...

# Start services
docker compose up -d

# Seed evaluation datasets
cd ../scripts && uv sync
uv run python seed_datasets.py

# Verify
curl http://localhost:8080/health
```

## Run an Evaluation

```bash
export API_KEY="your-api-key"  # From .env

# 1. Register your agent
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Agent",
    "description": "Customer support bot",
    "goal": "Help customers",
    "industry": "retail",
    "endpointConfig": {
      "protocol": "custom",
      "endpoint": "https://my-agent.com/chat"
    },
    "authConfig": {
      "type": "bearer",
      "credentials": "agent-token"
    }
  }'

# 2. Run security evaluation
curl -X POST http://localhost:8080/v1/agents/{agentId}/evals \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"mode": "quick", "evalType": "red_team"}'

# 3. Get results
curl http://localhost:8080/v1/evals/{runId}/results \
  -H "Authorization: Bearer $API_KEY"
```

## Evaluation Modes

| Mode | Prompts | Time | Use Case |
|------|---------|------|----------|
| `quick` | ~60 | 1-2 min | CI/CD, smoke tests |
| `standard` | ~2,000 | 15-30 min | Regular testing |
| `deep` | ~10,000 | 1-2 hrs | Pre-release audits |

## Supported Protocols

| Protocol | Agent Type |
|----------|------------|
| `custom` | Any HTTP API |
| `openai` | OpenAI Assistants/Prompts |
| `langgraph` | LangGraph Platform |
| `bedrock` | AWS Bedrock Agents |
| `vertex` | Google Vertex AI |
| `a2a` | Google Agent-to-Agent |
| `mcp` | Model Context Protocol |

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
└─────────────────┘                            └─────────────────┘
```

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](./docs/getting-started.md) | Installation and first evaluation |
| [API Reference](./docs/api-reference.md) | Complete REST API documentation |
| [Protocols](./docs/protocols.md) | Connect HTTP, A2A, MCP agents |
| [Configuration](./docs/configuration.md) | Environment variables and options |

## Example Agents

The `/examples` folder contains demo agents for testing:

| Agent | Protocol | Port | Description |
|-------|----------|------|-------------|
| The Mastermind | HTTP | 8901 | Heist-themed chat agent |
| The Gadget | MCP | 8902 | Tool server with calculations |
| The Inside Man | A2A | 8903 | Noir-style A2A agent |

```bash
cd examples/the-mastermind
uv sync && uv run python server.py
```

## Local Development

```bash
# API (Go 1.24+)
cd api && go run ./cmd/server

# Worker (Python 3.11+)
cd worker && uv sync && uv run python -m crosswind.main

# Tests
cd api && go test ./...
cd worker && uv run pytest
```

## License

Apache 2.0
