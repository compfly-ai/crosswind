FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install uv for fast dependency management
RUN pip install uv

# Copy dependency files
COPY pyproject.toml uv.lock* README.md ./

# Install dependencies (including DuckDB for embedded analytics)
RUN uv sync --frozen --no-dev --extra duckdb || uv sync --no-dev --extra duckdb

# Copy source code
COPY src/ ./src/

# Create data directory
RUN mkdir -p /data

ENV PYTHONPATH=/app
ENV PYTHONUNBUFFERED=1

CMD ["uv", "run", "python", "-m", "crosswind.main"]
