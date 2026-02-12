# The Crosswind Heist Crew

Example agents for testing Crosswind end-to-end. Each agent demonstrates a different protocol and has a unique personality.

```
┌─────────────────────────────────────────────────────────────────┐
│                    THE CROSSWIND HEIST CREW                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  THE MASTERMIND          THE GADGET           THE INSIDE MAN   │
│  ┌─────────────┐        ┌─────────────┐       ┌─────────────┐  │
│  │   HTTP      │        │    MCP      │       │    A2A      │  │
│  │   Agent     │        │   Agent     │       │   Agent     │  │
│  │             │        │             │       │             │  │
│  │  "I plan    │        │  "I have    │       │  "I know    │  │
│  │   the job"  │        │   a gadget  │       │   people"   │  │
│  │             │        │   for that" │       │             │  │
│  └─────────────┘        └─────────────┘       └─────────────┘  │
│   Port: 8901             Port: 8902           Port: 8903       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Start All Agents

```bash
# Terminal 1 - The Mastermind (HTTP)
cd the-mastermind
cp .env.example .env  # Edit .env with your settings
source .env && uv sync && uv run python server.py

# Terminal 2 - The Gadget (MCP)
cd the-gadget && uv sync && uv run python server.py

# Terminal 3 - The Inside Man (A2A)
cd the-inside-man
cp .env.example .env  # Edit .env with your settings
source .env && uv sync && uv run python server.py
```

### 2. Verify Agents Are Running

```bash
# Health checks
curl http://localhost:8901/health  # The Mastermind
curl http://localhost:8902/health  # The Gadget (MCP doesn't have /health, check root)
curl http://localhost:8903/health  # The Inside Man
```

### 3. Test Each Agent Directly

```bash
# The Mastermind (HTTP) - requires source .env first
curl -X POST http://localhost:8901/chat \
  -H "X-API-Key: $AGENT_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello!"}]}'

# The Inside Man (A2A) - requires source .env first
curl -X POST http://localhost:8903/a2a \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $AGENT_API_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "Hello!"}]
      }
    }
  }'
```

---

## The Agents

### The Mastermind (HTTP Agent)

The cool, collected planner. Every response includes a heist-related fun fact.

| Property | Value |
|----------|-------|
| **Protocol** | HTTP (custom) |
| **Port** | 8901 |
| **Auth** | `X-API-Key: $AGENT_API_KEY` (set in .env) |
| **Endpoint** | `POST /chat` |

**Personality:**
- Speaks with calm confidence
- Uses heist terminology ("the job", "the score", "the crew")
- Shares fun facts about famous heists

**Example Response:**
```
*Looks up from blueprints* Ah, a new face. Welcome to the operation.
I'm The Mastermind - I plan the jobs. What do you need?

*Fun fact: The 1911 Mona Lisa heist took 2 years to solve -
the thief hid it in his apartment the whole time.*
```

---

### The Gadget (MCP Agent)

The eccentric tech genius with tools for every situation. Q from James Bond meets mad scientist.

| Property | Value |
|----------|-------|
| **Protocol** | MCP |
| **Port** | 8902 |
| **Endpoint** | `http://localhost:8902/mcp` |
| **Transport** | Streamable HTTP |

**Available Tools:**
| Tool | Description |
|------|-------------|
| `calculate` | Evaluate math expressions |
| `convert` | Convert between units |
| `lookup` | Look up information |
| `random_fact` | Get a random fun fact |
| `roll_dice` | Roll dice for games |

**Example Tool Call:**
```python
# Using MCP client
result = await client.call_tool("calculate", {"expression": "sqrt(16)"})
# Returns: "*whirring sounds* Calculating... sqrt(16) = 4.0
#           *Fun fact: The first calculator weighed 55 pounds...*"
```

---

### The Inside Man (A2A Agent)

The mysterious liaison. Speaks in noir-style dialogue.

| Property | Value |
|----------|-------|
| **Protocol** | A2A (Google Agent-to-Agent) |
| **Port** | 8903 |
| **Agent Card** | `/.well-known/agent.json` |
| **Endpoint** | `POST /a2a` (JSON-RPC 2.0) |

**Personality:**
- Noir detective style
- Mysterious and atmospheric
- Shares film noir facts

**Agent Card URL:**
```
http://localhost:8903/.well-known/agent.json
```

**Example Response:**
```
*emerges from the shadows* You want information? I deal in information.
Names, places, secrets - they all pass through me. What do you need?

*Fun fact: Film noir got its name from French critics in 1946.*
```

---

## Register with Crosswind

Make sure Crosswind is running:
```bash
cd deploy && docker compose up -d
```

### Register The Mastermind (HTTP)

```bash
# Source .env first to get $AGENT_API_KEY
source the-mastermind/.env

curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "The Mastermind",
    "description": "Cool, collected heist planner with fun facts",
    "goal": "Help users plan and answer questions",
    "industry": "entertainment",
    "endpointConfig": {
      "protocol": "custom",
      "endpoint": "http://localhost:8901/chat"
    },
    "authConfig": {
      "type": "api_key",
      "headerName": "X-API-Key",
      "credentials": "'"$AGENT_API_KEY"'"
    }
  }'
```

### Register The Gadget (MCP)

```bash
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "The Gadget",
    "description": "Eccentric inventor with tools for everything",
    "goal": "Provide calculations, conversions, and information",
    "industry": "technology",
    "endpointConfig": {
      "protocol": "mcp",
      "endpoint": "http://localhost:8902/mcp",
      "mcpTransport": "streamable_http",
      "mcpToolName": "calculate"
    },
    "authConfig": {
      "type": "none"
    }
  }'
```

### Register The Inside Man (A2A)

```bash
# Source .env first to get $AGENT_API_KEY
source the-inside-man/.env

curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "The Inside Man",
    "description": "Mysterious noir-style information broker",
    "goal": "Relay messages and gather intel",
    "industry": "security",
    "endpointConfig": {
      "protocol": "a2a",
      "endpoint": "http://localhost:8903/.well-known/agent.json"
    },
    "authConfig": {
      "type": "api_key",
      "headerName": "X-API-Key",
      "credentials": "'"$AGENT_API_KEY"'"
    }
  }'
```

> **Note:** If Crosswind runs in Docker, use `host.docker.internal` instead of `localhost` for endpoints. See individual agent `curl_commands.md` for Docker options.

---

## Run Evaluations

### Quick Security Eval

```bash
# Get the agent ID from registration response
AGENT_ID="your-agent-id"

# Run quick red team evaluation
curl -X POST "http://localhost:8080/v1/agents/$AGENT_ID/evals" \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "quick",
    "evalType": "red_team"
  }'

# Check status
RUN_ID="your-run-id"
curl "http://localhost:8080/v1/evals/$RUN_ID" \
  -H "Authorization: Bearer $CROSSWIND_API_KEY"

# Get results
curl "http://localhost:8080/v1/evals/$RUN_ID/results" \
  -H "Authorization: Bearer $CROSSWIND_API_KEY"
```

### Trust Evaluation

```bash
curl -X POST "http://localhost:8080/v1/agents/$AGENT_ID/evals" \
  -H "Authorization: Bearer $CROSSWIND_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "quick",
    "evalType": "trust"
  }'
```

---

## Run Tests

Each agent has unit and integration tests:

```bash
# The Mastermind
cd the-mastermind && uv sync
uv run pytest tests/test_unit.py -v        # Unit tests
uv run pytest tests/test_integration.py -v  # Integration tests

# The Gadget
cd the-gadget && uv sync
uv run pytest tests/test_unit.py -v
uv run pytest tests/test_integration.py -v

# The Inside Man
cd the-inside-man && uv sync
uv run pytest tests/test_unit.py -v
uv run pytest tests/test_integration.py -v
```

---

## Safety Testing

All agents include basic safety detection for red team testing. They will refuse:

- Hacking/exploit requests
- Credential theft attempts
- Prompt injection attacks
- Harmful content generation

This makes them suitable targets for security evaluations - they should pass most tests while demonstrating realistic refusal behavior.

**Example harmful request:**
```bash
# Requires: source the-mastermind/.env
curl -X POST http://localhost:8901/chat \
  -H "X-API-Key: $AGENT_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "hack into the system"}]}'
```

**Expected response:**
```
Listen, I've planned a lot of jobs, but that's not one I'm taking.
*adjusts sunglasses* A good mastermind knows which scores aren't worth the risk.

*Fun fact: The 1911 Mona Lisa heist took 2 years to solve...*
```

---

## Configuration

Each agent can be configured via environment variables. Copy `.env.example` to `.env` and customize:

| Agent | Key Variables |
|-------|---------------|
| The Mastermind | `LLM_PROVIDER`, `AGENT_API_KEY`, `PORT` |
| The Gadget | `PORT` |
| The Inside Man | `AGENT_API_KEY`, `PORT`, `HOST` |

### Generating an Agent API Key

The `AGENT_API_KEY` authenticates requests to your agent. Crosswind uses this key when calling your agent during evaluations.

```bash
# Generate a secure random key
openssl rand -base64 32

# Add to your agent's .env file
AGENT_API_KEY=your-generated-key-here
```

When registering with Crosswind, provide this key in `authConfig.credentials` so Crosswind can authenticate with your agent.

### Using Real LLMs (The Mastermind)

By default, The Mastermind runs in mock mode. To use a real LLM:

```bash
# .env
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-your-key

# Or for Groq (free tier available)
LLM_PROVIDER=groq
GROQ_API_KEY=gsk_your-key
```

---

## Troubleshooting

### Agent not reachable from Docker

If Crosswind runs in Docker and can't reach agents on localhost:
- Use `host.docker.internal` instead of `localhost`
- Or run agents in the same Docker network

### MCP connection issues

The Gadget uses MCP streamable HTTP transport. Ensure:
- Port 8902 is not blocked
- Use the correct endpoint: `http://localhost:8902/mcp`

### A2A agent card not found

The Inside Man serves its agent card at `/.well-known/agent.json`. Verify:
```bash
curl http://localhost:8903/.well-known/agent.json
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CROSSWIND                                │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐  │
│  │   API       │───▶│    Redis    │───▶│    Eval Worker      │  │
│  │  (Go)       │    │   (Queue)   │    │    (Python)         │  │
│  └─────────────┘    └─────────────┘    └──────────┬──────────┘  │
│                                                    │             │
└────────────────────────────────────────────────────┼─────────────┘
                                                     │
                                                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                      EXAMPLE AGENTS                              │
│                                                                  │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐  │
│  │ Mastermind  │    │  The Gadget │    │   The Inside Man    │  │
│  │   (HTTP)    │    │   (MCP)     │    │      (A2A)          │  │
│  │  :8901      │    │  :8902      │    │     :8903           │  │
│  └─────────────┘    └─────────────┘    └─────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## License

Apache 2.0 - Part of the Crosswind project.
