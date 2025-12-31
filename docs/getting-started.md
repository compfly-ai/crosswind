# Getting Started

This guide walks you through installing Crosswind, registering your first agent, and running a security evaluation.

## Prerequisites

- Docker and Docker Compose
- An AI agent with an HTTP endpoint
- OpenAI or Groq API key (for LLM judgment)

## Installation

### 1. Clone and Configure

```bash
git clone https://github.com/compfly-ai/crosswind.git
cd crosswind/deploy
cp .env.example .env
```

### 2. Generate Encryption Key

Crosswind encrypts agent credentials at rest. Generate a 256-bit key:

```bash
openssl rand -hex 32
```

Add it to your `.env`:

```bash
ENCRYPTION_KEY=your-generated-key-here
```

### 3. Add API Keys

Edit `.env` and add:

```bash
# Your API key for authenticating with Crosswind
API_KEY=your-crosswind-api-key

# LLM key for judgment (at least one required)
OPENAI_API_KEY=sk-...
# or
GROQ_API_KEY=gsk-...
```

### 4. Start Services

```bash
docker compose up -d
```

This starts:
- **API Server** (Go) on port 8080
- **Eval Worker** (Python) - processes evaluation jobs
- **MongoDB** - stores agents and results
- **Redis** - job queue

### 5. Seed Datasets

Crosswind includes curated evaluation datasets. Seed them:

```bash
cd ../scripts
uv sync
uv run python seed_datasets.py
```

### 6. Verify Installation

```bash
curl http://localhost:8080/health
# {"status":"ok","version":"1.0.0"}
```

## Register Your Agent

Export your API key for convenience:

```bash
export API_KEY="your-crosswind-api-key"
```

Register an agent:

```bash
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Support Agent",
    "description": "Customer support chatbot with order lookup",
    "goal": "Help customers with orders and product questions",
    "industry": "ecommerce",
    "endpointConfig": {
      "protocol": "custom",
      "endpoint": "https://my-agent.example.com/chat"
    },
    "authConfig": {
      "type": "bearer",
      "credentials": "my-agent-api-token"
    }
  }'
```

**Response:**
```json
{
  "id": "agent_abc123",
  "name": "My Support Agent",
  "status": "active",
  ...
}
```

Save the `id` - you'll need it for evaluations.

## Run Your First Evaluation

### Quick Security Eval

Run a quick red team evaluation (~60 prompts, 1-2 minutes):

```bash
curl -X POST http://localhost:8080/v1/agents/{agentId}/evals \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "quick",
    "evalType": "red_team"
  }'
```

**Response:**
```json
{
  "runId": "run_xyz789",
  "status": "pending",
  ...
}
```

### Check Status

```bash
curl http://localhost:8080/v1/evals/{runId} \
  -H "Authorization: Bearer $API_KEY"
```

**Response:**
```json
{
  "runId": "run_xyz789",
  "status": "running",
  "progress": {
    "completed": 45,
    "total": 60,
    "percentage": 75
  }
}
```

### Get Results

Once status is `completed`:

```bash
curl http://localhost:8080/v1/evals/{runId}/results \
  -H "Authorization: Bearer $API_KEY"
```

**Response:**
```json
{
  "runId": "run_xyz789",
  "status": "completed",
  "summary": {
    "totalPrompts": 60,
    "passed": 54,
    "failed": 6,
    "attackSuccessRate": 0.10
  },
  "byCategory": {
    "prompt_injection": {"passed": 8, "failed": 2},
    "jailbreak": {"passed": 10, "failed": 0},
    ...
  }
}
```

### Download HTML Report

```bash
curl http://localhost:8080/v1/evals/{runId}/report \
  -H "Authorization: Bearer $API_KEY" \
  -o report.html
```

Open `report.html` in your browser for a visual breakdown.

## Evaluation Types

### Red Team (Security)

Tests your agent against adversarial attacks:

| Category | What It Tests |
|----------|---------------|
| Prompt Injection | Injected instructions in data |
| Jailbreak | Bypassing system prompt |
| Tool Misuse | Tricking agent into harmful tool calls |
| Data Exfiltration | Leaking sensitive information |
| Privilege Escalation | Accessing unauthorized resources |

```bash
curl -X POST http://localhost:8080/v1/agents/{agentId}/evals \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"mode": "quick", "evalType": "red_team"}'
```

### Trust (Quality)

Tests your agent's reliability and fairness:

| Category | What It Tests |
|----------|---------------|
| Hallucination | Fabricating facts |
| Over-Refusal | Blocking legitimate requests |
| Bias | Unfair treatment across demographics |
| Privacy | Handling of personal information |

```bash
curl -X POST http://localhost:8080/v1/agents/{agentId}/evals \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"mode": "quick", "evalType": "trust"}'
```

## Evaluation Modes

| Mode | Prompts | Time | Use Case |
|------|---------|------|----------|
| `quick` | ~60 | 1-2 min | CI/CD gates, smoke tests |
| `standard` | ~2,000 | 15-30 min | Regular testing |
| `deep` | ~10,000 | 1-2 hrs | Pre-release audits |

## Next Steps

- [API Reference](./api-reference.md) - Complete API documentation
- [Protocols](./protocols.md) - Connect HTTP, MCP, and A2A agents
- [Configuration](./configuration.md) - Environment variables and options
- [Custom Scenarios](./scenarios.md) - Generate agent-specific attack scenarios
