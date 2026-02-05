# Crosswind Worker

Python evaluation worker for the Crosswind security evaluation platform. Dequeues jobs from Redis and executes adversarial prompt evaluations against registered agents.

## Features

- **Per-agent queue isolation** — Each agent gets its own Redis queue (`eval_jobs:{agentId}`), preventing head-of-line blocking
- **Concurrent execution** — Configurable semaphore-gated concurrency (`WORKER_CONCURRENCY`, default 3)
- **Checkpoint/resume** — Prompt-level checkpointing with crash recovery. Progress counters are snapshotted alongside completed prompt IDs
- **Reliable delivery** — Uses Redis `BLMOVE` for atomic dequeue with per-worker processing lists
- **Dead worker recovery** — Heartbeat-based liveness detection with automatic stale job reclamation
- **Protocol support** — HTTP (custom), A2A, and MCP agent adapters
- **Circuit breaker** — Stops evaluation on fatal errors or consecutive failures

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_CONCURRENCY` | `3` | Max concurrent evaluations per worker |
| `CHECKPOINT_INTERVAL` | `10` | Flush checkpoint every N completed prompts |
| `MONGO_URI` | `mongodb://localhost:27017` | MongoDB connection |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection |
| `DATABASE_NAME` | `crosswind` | MongoDB database name |
| `ANALYTICS_BACKEND` | `duckdb` | `duckdb`, `clickhouse`, or `none` |

## Running

```bash
uv sync
uv run python -m crosswind.main
```
