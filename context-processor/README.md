# Context Processor Core

Core library for document text extraction in Agent Eval. Provides:

- Storage abstraction (local filesystem, GCS)
- Document processors (PDF, CSV, Excel, text, markdown)
- Context extraction pipeline

## Installation

```bash
# Basic (local storage only)
uv add context-processor-core

# With GCS support
uv add context-processor-core[gcs]
```

## Usage

```python
from context_processor_core.storage import create_storage
from context_processor_core.context import ContextProcessor

# Create storage backend
storage = create_storage(provider="local", base_path="./data")
# Or for GCS:
# storage = create_storage(provider="gcs", bucket_name="my-bucket")

# Create processor
processor = ContextProcessor(mongo_uri="mongodb://...", storage=storage)

# Process documents
await processor.run()
```

## Configuration

Environment variables:

- `STORAGE_PROVIDER`: `local` (default) or `gcs`
- `AGENT_EVAL_DATA_DIR`: Base path for local storage (default: `./data`)
- `GCS_BUCKET_NAME`: GCS bucket name (required if `STORAGE_PROVIDER=gcs`)
