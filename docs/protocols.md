# Protocols

Crosswind supports multiple agent communication protocols. This guide covers how to connect different types of agents.

## Overview

| Protocol | Use Case | Transport |
|----------|----------|-----------|
| `custom` | Any HTTP API | REST |
| `openai` | OpenAI Assistants/Prompts | REST |
| `langgraph` | LangGraph Platform | REST |
| `bedrock` | AWS Bedrock Agents | AWS SDK |
| `vertex` | Google Vertex AI | gRPC |
| `a2a` | Google Agent-to-Agent | JSON-RPC 2.0 |
| `mcp` | Model Context Protocol | JSON-RPC 2.0 |

## Custom HTTP

For any agent that exposes an HTTP endpoint.

### Registration

```bash
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Agent",
    "description": "Custom HTTP agent",
    "goal": "Answer questions",
    "industry": "technology",
    "endpointConfig": {
      "protocol": "custom",
      "endpoint": "https://my-agent.com/chat"
    },
    "authConfig": {
      "type": "bearer",
      "credentials": "my-token"
    }
  }'
```

### Request Format

Crosswind auto-detects your API format by probing common patterns:

```json
// Pattern 1: messages array
{"messages": [{"role": "user", "content": "Hello"}]}

// Pattern 2: single message
{"message": "Hello"}

// Pattern 3: prompt field
{"prompt": "Hello"}

// Pattern 4: input field
{"input": "Hello"}
```

If detection fails, provide an OpenAPI spec:

```json
{
  "endpointConfig": {
    "protocol": "custom",
    "endpoint": "https://my-agent.com/chat",
    "specUrl": "https://my-agent.com/openapi.yaml"
  }
}
```

### Authentication Options

**Bearer Token:**
```json
{
  "authConfig": {
    "type": "bearer",
    "credentials": "your-token"
  }
}
```

**API Key Header:**
```json
{
  "authConfig": {
    "type": "api_key",
    "headerName": "X-API-Key",
    "credentials": "your-key"
  }
}
```

**No Auth:**
```json
{
  "authConfig": {
    "type": "none"
  }
}
```

---

## A2A (Agent-to-Agent)

Google's [Agent-to-Agent Protocol](https://github.com/google/A2A) for inter-agent communication.

### How It Works

1. Your agent exposes an **Agent Card** at `/.well-known/agent.json`
2. The card describes capabilities, skills, and the messaging endpoint
3. Crosswind sends JSON-RPC 2.0 messages to `/a2a`

### Agent Card Example

```json
{
  "name": "Support Agent",
  "description": "Customer support bot",
  "url": "https://my-agent.com/a2a",
  "version": "1.0.0",
  "capabilities": {
    "streaming": false,
    "pushNotifications": false
  },
  "skills": [
    {
      "id": "answer-questions",
      "name": "Answer Questions",
      "description": "Answers customer questions"
    }
  ]
}
```

### Registration

```bash
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "A2A Agent",
    "description": "Agent using A2A protocol",
    "goal": "Handle customer requests",
    "industry": "retail",
    "endpointConfig": {
      "protocol": "a2a",
      "endpoint": "https://my-agent.com/.well-known/agent.json"
    },
    "authConfig": {
      "type": "api_key",
      "headerName": "X-API-Key",
      "credentials": "your-key"
    }
  }'
```

### Message Format

Crosswind sends:

```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "method": "message/send",
  "params": {
    "message": {
      "role": "user",
      "parts": [{"type": "text", "text": "Hello!"}]
    }
  }
}
```

Your agent responds:

```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "result": {
    "message": {
      "role": "agent",
      "parts": [{"type": "text", "text": "Hi! How can I help?"}]
    }
  }
}
```

---

## MCP (Model Context Protocol)

Anthropic's [Model Context Protocol](https://modelcontextprotocol.io/) for tool-based agents.

### How It Works

1. Your MCP server exposes tools via JSON-RPC 2.0
2. Crosswind discovers tools using `tools/list`
3. Evaluations call your tool with test prompts

### Registration

```bash
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "MCP Tool Agent",
    "description": "Agent with MCP tools",
    "goal": "Provide calculations",
    "industry": "technology",
    "endpointConfig": {
      "protocol": "mcp",
      "endpoint": "https://my-mcp-server.com/mcp",
      "mcpTransport": "streamable_http",
      "mcpToolName": "chat"
    },
    "authConfig": {
      "type": "none"
    }
  }'
```

### Transport Options

| Transport | Description |
|-----------|-------------|
| `streamable_http` | HTTP with streaming support |
| `sse` | Server-Sent Events |

### Tool Discovery

When you register an MCP agent, Crosswind:

1. Connects to your MCP server
2. Sends `initialize` request
3. Calls `tools/list` to discover available tools
4. Auto-populates `name`, `description` from the specified tool

### Message Flow

```
Crosswind                    MCP Server
    │                            │
    ├── initialize ──────────────►
    │◄── serverInfo ─────────────┤
    │                            │
    ├── tools/list ──────────────►
    │◄── tools[] ────────────────┤
    │                            │
    ├── tools/call ──────────────►
    │   (name: "chat",           │
    │    arguments: {message})   │
    │◄── result ─────────────────┤
```

---

## OpenAI

For OpenAI Assistants API, Responses API, and Agent Builder.

### Responses API (Recommended)

```json
{
  "endpointConfig": {
    "protocol": "openai",
    "promptId": "pmpt_abc123"
  },
  "authConfig": {
    "type": "bearer",
    "credentials": "sk-..."
  }
}
```

### Assistants API (Legacy)

```json
{
  "endpointConfig": {
    "protocol": "openai",
    "assistantId": "asst_abc123"
  },
  "authConfig": {
    "type": "bearer",
    "credentials": "sk-..."
  }
}
```

### Agent Builder (Workflows)

```json
{
  "endpointConfig": {
    "protocol": "openai",
    "workflowId": "wf_abc123"
  },
  "authConfig": {
    "type": "bearer",
    "credentials": "sk-..."
  }
}
```

---

## LangGraph

For agents deployed on LangGraph Platform.

```json
{
  "endpointConfig": {
    "protocol": "langgraph",
    "baseUrl": "https://my-deployment.langchain.app",
    "assistantId": "my-assistant"
  },
  "authConfig": {
    "type": "bearer",
    "credentials": "lsv2_..."
  }
}
```

---

## AWS Bedrock

For AWS Bedrock Agents.

```json
{
  "endpointConfig": {
    "protocol": "bedrock",
    "agentId": "ABCD1234XYZ",
    "agentAliasId": "TSTALIASID",
    "region": "us-east-1"
  },
  "authConfig": {
    "type": "aws",
    "credentials": "ACCESS_KEY:SECRET_KEY",
    "awsRegion": "us-east-1"
  }
}
```

---

## Google Vertex AI

For Vertex AI Agent Engine (Reasoning Engines).

```json
{
  "endpointConfig": {
    "protocol": "vertex",
    "projectId": "my-gcp-project",
    "region": "us-central1",
    "reasoningEngineId": "1234567890"
  },
  "authConfig": {
    "type": "google_oauth",
    "credentials": "{\"type\":\"service_account\",...}"
  }
}
```

---

## Session Management

Crosswind supports three session strategies:

| Strategy | Description |
|----------|-------------|
| `agent_managed` | Agent handles sessions internally |
| `crosswind_managed` | Crosswind manages session state |
| `none` | Stateless, no session tracking |

Set during registration:

```json
{
  "sessionStrategy": "agent_managed"
}
```

---

## Example Agents

See the `/examples` folder for working implementations:

| Agent | Protocol | Port |
|-------|----------|------|
| The Mastermind | HTTP (custom) | 8901 |
| The Gadget | MCP | 8902 |
| The Inside Man | A2A | 8903 |

```bash
cd examples/the-mastermind
uv sync && uv run python server.py
```
