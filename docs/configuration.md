# Configuration

Complete reference for Crosswind environment variables and configuration options.

## Quick Setup

```bash
cd deploy
cp .env.example .env
```

Edit `.env` with your values, then:

```bash
docker compose up -d
```

## Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `ENCRYPTION_KEY` | AES-256 key for encrypting credentials | `openssl rand -hex 32` |
| `API_KEY` | Bearer token for API authentication | Any secure string |
| `OPENAI_API_KEY` | OpenAI API key for LLM judgment | `sk-...` |

## Optional Variables

### LLM Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GROQ_API_KEY` | No | - | Groq API key (alternative to OpenAI) |

### Database

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MONGO_URI` | No | `mongodb://localhost:27017` | MongoDB connection string |
| `DATABASE_NAME` | No | `agent_eval` | MongoDB database name |
| `REDIS_URL` | No | `redis://localhost:6379` | Redis connection string |

### Analytics (Optional)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ANALYTICS_BACKEND` | No | `duckdb` | Analytics storage: `duckdb`, `clickhouse`, `none` |
| `CLICKHOUSE_HOST` | No | - | ClickHouse host (if using clickhouse) |
| `CLICKHOUSE_DATABASE` | No | `agent_eval` | ClickHouse database |
| `CLICKHOUSE_USER` | No | `default` | ClickHouse user |
| `CLICKHOUSE_PASSWORD` | No | - | ClickHouse password |

### Storage

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `STORAGE_PROVIDER` | No | `local` | Storage backend: `local`, `gcs` |
| `AGENT_EVAL_DATA_DIR` | No | `/data` | Local storage path |
| `GCS_BUCKET` | No | - | GCS bucket (if using gcs) |
| `GOOGLE_APPLICATION_CREDENTIALS` | No | - | GCS service account JSON |

### API Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PORT` | No | `8080` | API server port |
| `ENVIRONMENT` | No | `development` | Environment: `development`, `production` |
| `DOCS_USERNAME` | No | - | Basic auth for /docs |
| `DOCS_PASSWORD` | No | - | Basic auth for /docs |

### Worker

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `WORKER_CONCURRENCY` | No | `4` | Parallel evaluation jobs |
| `DEFAULT_RPM` | No | `60` | Default requests per minute |

## Docker Compose

### Default Setup

```yaml
# deploy/docker-compose.yml
services:
  api:
    ports:
      - "8080:8080"
  worker:
    # Runs in background
  mongo:
    ports:
      - "27017:27017"
  redis:
    ports:
      - "6380:6379"
```

### With ClickHouse Analytics

```bash
docker compose -f docker-compose.yml -f docker-compose.analytics.yml up -d
```

## Local Development

For development without Docker:

### API Server (Go)

```bash
cd api

# Set environment
export MONGO_URI=mongodb://localhost:27017
export DATABASE_NAME=agent_eval
export REDIS_URL=redis://localhost:6379
export ENCRYPTION_KEY=$(openssl rand -hex 32)
export API_KEY=dev-api-key
export OPENAI_API_KEY=sk-...

# Run
go run ./cmd/server
```

### Worker (Python)

```bash
cd worker
uv sync

# Set environment (same as above, or symlink to deploy/.env)
ln -s ../deploy/.env .env

# Run
uv run python -m crosswind.main
```

### Dependencies

Start MongoDB and Redis locally:

```bash
# MongoDB
docker run -d -p 27017:27017 mongo:7.0

# Redis
docker run -d -p 6379:6379 redis:7-alpine
```

## Production Recommendations

### Security

1. Use strong, unique values for `ENCRYPTION_KEY` and `API_KEY`
2. Enable HTTPS via reverse proxy (nginx, Caddy, etc.)
3. Set `DOCS_USERNAME` and `DOCS_PASSWORD` to protect `/docs`
4. Restrict network access to MongoDB and Redis

### Performance

1. Use ClickHouse for analytics at scale
2. Increase `WORKER_CONCURRENCY` based on resources
3. Deploy multiple worker replicas for high throughput

### Example Production `.env`

```bash
# Security
ENCRYPTION_KEY=your-256-bit-hex-key
API_KEY=your-secure-api-key

# LLM
OPENAI_API_KEY=sk-...

# Database (use managed services)
MONGO_URI=mongodb+srv://user:pass@cluster.mongodb.net
DATABASE_NAME=crosswind

# Redis (use managed service)
REDIS_URL=redis://:password@redis-host:6379

# Analytics
ANALYTICS_BACKEND=clickhouse
CLICKHOUSE_HOST=clickhouse.example.com:9440
CLICKHOUSE_DATABASE=crosswind
CLICKHOUSE_USER=default
CLICKHOUSE_PASSWORD=your-password

# API protection
DOCS_USERNAME=admin
DOCS_PASSWORD=secure-docs-password
ENVIRONMENT=production
```

## Troubleshooting

### "ENCRYPTION_KEY is required"

Generate a key:
```bash
openssl rand -hex 32
```

Add to `.env`:
```
ENCRYPTION_KEY=your-generated-key
```

### "Connection refused" to MongoDB/Redis

Ensure services are running:
```bash
docker compose ps
```

Check health:
```bash
docker compose logs mongo
docker compose logs redis
```

### Worker not processing jobs

1. Check Redis connection:
```bash
docker compose exec redis redis-cli ping
```

2. Check worker logs:
```bash
docker compose logs worker
```

### LLM judgment failing

1. Verify API key is set:
```bash
docker compose exec api printenv | grep OPENAI
```

2. Check for rate limits in worker logs
