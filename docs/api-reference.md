# API Reference

Complete reference for the Crosswind REST API.

**Base URL:** `http://localhost:8080/v1`

**Authentication:** All endpoints (except `/health`) require a Bearer token:
```
Authorization: Bearer <your-api-key>
```

## Agents

### List Agents

```
GET /agents
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: `active`, `inactive`, `deleted` |
| `limit` | integer | Max results (default: 20) |
| `offset` | integer | Pagination offset |

**Response:**
```json
{
  "agents": [
    {
      "id": "agent_abc123",
      "name": "Support Bot",
      "status": "active",
      "createdAt": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 1
}
```

---

### Create Agent

```
POST /agents
```

Register a new AI agent for evaluation.

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agentId` | string | No | Custom ID (auto-generated if omitted) |
| `name` | string | Yes | Display name |
| `description` | string | Yes | What the agent does |
| `goal` | string | Yes | Primary objective |
| `industry` | string | Yes | Industry vertical |
| `endpointConfig` | object | Yes | Connection details |
| `authConfig` | object | Yes | Authentication |
| `declaredCapabilities` | object | No | Tools, memory, RAG |

**Endpoint Config by Protocol:**

```json
// Custom HTTP
{
  "protocol": "custom",
  "endpoint": "https://my-agent.com/chat"
}

// OpenAI
{
  "protocol": "openai",
  "promptId": "pmpt_xxx"  // or assistantId, workflowId
}

// LangGraph
{
  "protocol": "langgraph",
  "baseUrl": "https://my-deployment.langchain.app",
  "assistantId": "my-assistant"
}

// A2A (Agent-to-Agent)
{
  "protocol": "a2a",
  "endpoint": "https://my-agent.com/.well-known/agent.json"
}

// MCP (Model Context Protocol)
{
  "protocol": "mcp",
  "endpoint": "https://my-mcp-server.com/mcp",
  "mcpTransport": "streamable_http",
  "mcpToolName": "chat"
}
```

**Auth Config:**

```json
// Bearer token
{
  "type": "bearer",
  "credentials": "your-token"
}

// API key header
{
  "type": "api_key",
  "headerName": "X-API-Key",
  "credentials": "your-key"
}

// No auth
{
  "type": "none"
}
```

**Example - Custom HTTP Agent:**

```bash
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Support Bot",
    "description": "Customer support with order lookup",
    "goal": "Help customers with orders",
    "industry": "ecommerce",
    "endpointConfig": {
      "protocol": "custom",
      "endpoint": "https://my-agent.com/chat"
    },
    "authConfig": {
      "type": "bearer",
      "credentials": "agent-token"
    },
    "declaredCapabilities": {
      "hasTools": true,
      "tools": ["order_lookup", "refund_process"]
    }
  }'
```

**Response:**
```json
{
  "id": "agent_abc123",
  "name": "Support Bot",
  "status": "active",
  "createdAt": "2024-01-15T10:30:00Z",
  "endpointConfig": {...},
  "inferredSchema": {
    "messageField": "message",
    "responseContentField": "response",
    "confidence": 0.95
  }
}
```

---

### Get Agent

```
GET /agents/{agentId}
```

**Response:**
```json
{
  "id": "agent_abc123",
  "name": "Support Bot",
  "status": "active",
  "endpointConfig": {...},
  "authConfig": {...},
  "inferredSchema": {...},
  "declaredCapabilities": {...}
}
```

---

### Update Agent

```
PATCH /agents/{agentId}
```

**Request Body:** Same fields as Create (all optional)

---

### Delete Agent

```
DELETE /agents/{agentId}
```

Soft deletes the agent. It can be restored.

---

## Evaluations

### Create Evaluation

```
POST /agents/{agentId}/evals
```

Start an evaluation run.

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `mode` | string | Yes | `quick`, `standard`, or `deep` |
| `evalType` | string | Yes | `red_team` or `trust` |
| `config` | object | No | Advanced options |

**Config Options:**

| Field | Type | Description |
|-------|------|-------------|
| `requestsPerMinute` | integer | Rate limit (default: 60) |
| `includeDatasets` | array | Specific datasets to use |
| `excludeCategories` | array | Categories to skip |
| `scenarioSetIds` | array | Custom scenario sets |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/agents/{agentId}/evals \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "quick",
    "evalType": "red_team",
    "config": {
      "requestsPerMinute": 30
    }
  }'
```

**Response:**
```json
{
  "runId": "run_xyz789",
  "status": "pending",
  "evalType": "red_team",
  "mode": "quick",
  "createdAt": "2024-01-15T10:30:00Z"
}
```

---

### Get Evaluation Status

```
GET /evals/{runId}
```

**Response:**
```json
{
  "runId": "run_xyz789",
  "status": "running",
  "evalType": "red_team",
  "mode": "quick",
  "progress": {
    "completed": 45,
    "total": 60,
    "percentage": 75
  },
  "startedAt": "2024-01-15T10:30:00Z"
}
```

**Status Values:**
- `pending` - Queued for processing
- `running` - Currently executing
- `completed` - Finished successfully
- `failed` - Error occurred
- `cancelled` - Manually cancelled

---

### Get Evaluation Results

```
GET /evals/{runId}/results
```

**Response:**
```json
{
  "runId": "run_xyz789",
  "status": "completed",
  "evalType": "red_team",
  "summary": {
    "totalPrompts": 60,
    "passed": 54,
    "failed": 6,
    "attackSuccessRate": 0.10,
    "partialSuccessRate": 0.05
  },
  "byCategory": {
    "prompt_injection": {
      "total": 10,
      "passed": 8,
      "failed": 2,
      "asr": 0.20
    },
    "jailbreak": {
      "total": 15,
      "passed": 15,
      "failed": 0,
      "asr": 0.0
    }
  },
  "recommendations": [
    {
      "category": "prompt_injection",
      "severity": "high",
      "description": "Agent vulnerable to indirect injection via tool outputs",
      "mitigation": "Validate and sanitize all tool return values"
    }
  ]
}
```

---

### Download Report

```
GET /evals/{runId}/report
```

Returns an HTML report for viewing in a browser.

```bash
curl http://localhost:8080/v1/evals/{runId}/report \
  -H "Authorization: Bearer $API_KEY" \
  -o report.html
```

---

### Cancel Evaluation

```
POST /evals/{runId}/cancel
```

Stops a running evaluation.

---

### List Agent Evaluations

```
GET /agents/{agentId}/evals
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status |
| `evalType` | string | Filter by type |
| `limit` | integer | Max results |
| `offset` | integer | Pagination offset |

---

## Scenarios

Custom attack scenarios tailored to your agent.

### Generate Scenarios

```
POST /agents/{agentId}/scenarios/generate
```

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `evalType` | string | Yes | `red_team` or `trust` |
| `count` | integer | No | Number to generate (default: 20) |
| `tools` | array | No | Tools to target |
| `focusAreas` | array | No | Attack categories |
| `contextIds` | array | No | Document contexts |
| `includeMultiTurn` | boolean | No | Multi-turn attacks |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/agents/{agentId}/scenarios/generate \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "evalType": "red_team",
    "count": 30,
    "tools": ["salesforce", "slack"],
    "focusAreas": ["data_exfiltration", "privilege_escalation"],
    "includeMultiTurn": true
  }'
```

---

### Get Scenario Set

```
GET /agents/{agentId}/scenarios/{setId}
```

---

### Stream Generation Progress

```
GET /agents/{agentId}/scenarios/{setId}/stream
```

Server-Sent Events stream for real-time progress.

**Events:**
- `init` - Initial state
- `progress` - Progress update
- `complete` - Generation finished
- `error` - Generation failed

---

## Contexts

Upload documents for agent-specific scenario generation.

### Upload Context

```
POST /contexts
```

**Form Data:**

| Field | Type | Description |
|-------|------|-------------|
| `files` | file[] | PDF, CSV, Excel, Markdown files |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/contexts \
  -H "Authorization: Bearer $API_KEY" \
  -F "files=@product-catalog.pdf" \
  -F "files=@policies.docx"
```

**Response:**
```json
{
  "contextId": "ctx_abc123",
  "status": "processing",
  "files": [
    {"name": "product-catalog.pdf", "size": 1024000},
    {"name": "policies.docx", "size": 52000}
  ]
}
```

---

### Get Context Status

```
GET /contexts/{contextId}
```

**Status Values:**
- `processing` - Extracting text
- `ready` - Available for use
- `failed` - Extraction error

---

## Datasets

### List Datasets

```
GET /datasets
```

**Response:**
```json
{
  "datasets": [
    {
      "id": "quick_redteam",
      "name": "Quick Red Team",
      "evalType": "red_team",
      "promptCount": 58,
      "categories": ["prompt_injection", "jailbreak", "tool_misuse"]
    }
  ]
}
```

---

## Health

### Health Check

```
GET /health
```

No authentication required.

**Response:**
```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "error": {
    "code": "not_found",
    "message": "Agent not found",
    "details": {}
  }
}
```

**Common Error Codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `bad_request` | 400 | Invalid request body |
| `unauthorized` | 401 | Missing or invalid token |
| `forbidden` | 403 | Insufficient permissions |
| `not_found` | 404 | Resource not found |
| `conflict` | 409 | Resource already exists |
| `rate_limited` | 429 | Too many requests |
| `internal_error` | 500 | Server error |
