# Repository Guidelines

## Project Structure & Module Organization
- `api/`: Go REST API (`cmd/server` entrypoint) plus `internal/` services, handlers, and repository layers; tests live as `*_test.go`.
- `worker/`: Python eval runner (`src/crosswind`) with datasets and analytics hooks; tests in `tests/`.
- `context-processor/`: Python library for document extraction; storage/parsing modules under `src/`.
- `deploy/`: Docker Compose for API, worker, MongoDB, Redis; optional analytics overlay.
- `scripts/`: Dataset seeding tools; run from here when populating MongoDB.
- `Makefile`: Common tasks; prefer `make` targets to stay aligned with CI.

## Build, Test, and Development Commands
- `make up` / `make down`: Start/stop full stack from `deploy/`.
- `make run-api` / `make run-worker`: Run services locally without containers.
- `make test`: Run Go and Python test suites.
- Go API: `cd api && go run ./cmd/server` (dev server), `go test ./...` (unit/integration), `go vet ./...` (static checks).
- Python worker/context: `cd worker && uv sync` (deps), `uv run pytest` (tests), `uv run ruff check .` and `uv run mypy .` (lint/type).
- Dataset seeding: `cd scripts && uv run python seed_datasets.py --red-team` (minimal), `--all` for full set.

## Coding Style & Naming Conventions
- Go: standard `gofmt`/`goimports`; prefer small packages, request-scoped contexts, and structured logging (`zap`) in handlers/services.
- Python: ruff-enforced style (line length 120, long prompt strings allowed), mypy `--strict`; snake_case for functions/vars, CapWords for classes.
- Tests mirror source structure; name Go tests `TestXxx`, Python tests `test_*`.

## Testing Guidelines
- Go: table-driven tests covering handlers/services/repos; mock external services and Redis/Mongo when possible.
- Python: pytest with asyncio support; prefer fixtures; include representative prompt/tool-call flows.
- Run `make test` before PRs; add targeted tests for new features/bug fixes.

## Commit & Pull Request Guidelines
- Commits: short imperative summaries (e.g., `fix worker lint`, `add dataset seed option`); keep related changes together.
- PRs: describe behavior change, risks, and test coverage; link issues when applicable; include screenshots or sample responses for API/CLI changes.
- Keep diffs minimal; align with existing package boundaries and avoid drive-by refactors without tests.

## Security & Configuration Tips
- Secrets live in `.env` under `deploy/`; never commit keys—use the provided example as a template.
- Most datasets require `HUGGINGFACE_TOKEN`; seed only the sets you need in shared environments.
- When exposing API endpoints, ensure `ENCRYPTION_KEY` and `API_KEY` are set and avoid logging sensitive payloads.
