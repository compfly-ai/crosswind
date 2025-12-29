.PHONY: help build up down logs ps test clean seed seed-all

# Default target
help:
	@echo "Crosswind OSS - Available targets:"
	@echo ""
	@echo "  up        - Start all services (API, Worker, MongoDB, Redis)"
	@echo "  down      - Stop all services"
	@echo "  logs      - View logs (all services)"
	@echo "  ps        - Show running containers"
	@echo "  build     - Build Docker images"
	@echo "  test      - Run tests"
	@echo "  clean     - Remove containers and volumes"
	@echo ""
	@echo "Dataset Seeding:"
	@echo "  seed      - Seed red-team datasets from HuggingFace into MongoDB"
	@echo "  seed-all  - Seed all OSS-safe datasets into MongoDB"
	@echo "  seed-dry  - Dry run - show what would be seeded"
	@echo ""
	@echo "Development:"
	@echo "  build-api    - Build API locally"
	@echo "  build-worker - Build Worker locally"
	@echo "  run-api      - Run API locally"
	@echo "  run-worker   - Run Worker locally"

# Docker Compose targets
up:
	cd deploy && docker compose up -d

up-full:
	cd deploy && docker compose -f docker-compose.yml -f docker-compose.analytics.yml up -d

down:
	cd deploy && docker compose down

logs:
	cd deploy && docker compose logs -f

ps:
	cd deploy && docker compose ps

build:
	cd deploy && docker compose build

clean:
	cd deploy && docker compose down -v --rmi local

# Development targets
build-api:
	cd api && go build -o bin/agent-eval-api ./cmd/server

build-worker:
	cd worker && uv sync

run-api:
	cd api && go run ./cmd/server

run-worker:
	cd worker && uv run python -m src.main

test: test-api test-worker

test-api:
	cd api && go test ./...

test-worker:
	cd worker && uv run pytest

# Health check
health:
	@curl -s http://localhost:8080/health | jq .
	@curl -s http://localhost:8080/ready | jq .

# Dataset seeding
# Default: Seeds curated agentic datasets (quick_agentic, quick_trust_agentic) - no external dependencies
seed:
	cd scripts && uv run python seed_datasets.py

# Seeds all OSS-safe datasets (requires HuggingFace downloads)
seed-all:
	cd scripts && uv run python seed_datasets.py --all

seed-dry:
	cd scripts && uv run python seed_datasets.py --dry-run
