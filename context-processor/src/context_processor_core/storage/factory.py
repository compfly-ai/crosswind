"""Storage factory for creating storage backends."""

import logging
import os

from .base import FileStorage
from .local import LocalFileStorage

logger = logging.getLogger(__name__)


def create_storage(
    provider: str | None = None,
    base_path: str | None = None,
    bucket_name: str | None = None,
) -> FileStorage:
    """Create a storage backend based on configuration.

    Args:
        provider: Storage provider ("local" or "gcs").
                 Defaults to STORAGE_PROVIDER env var, or "local".
        base_path: Base path for local storage.
                  Defaults to AGENT_EVAL_DATA_DIR env var, or "./data".
        bucket_name: GCS bucket name (required if provider="gcs").
                    Defaults to GCS_BUCKET_NAME env var.

    Returns:
        Configured FileStorage implementation.

    Raises:
        ValueError: If provider is unknown.
        ImportError: If GCS is requested but google-cloud-storage not installed.
        ValueError: If GCS is requested but bucket_name not provided.
    """
    # Resolve provider
    if provider is None:
        provider = os.getenv("STORAGE_PROVIDER", "local")

    provider = provider.lower()
    logger.info(f"Creating storage backend: {provider}")

    if provider == "local":
        if base_path is None:
            base_path = os.getenv("AGENT_EVAL_DATA_DIR", "./data")
        return LocalFileStorage(base_path)

    elif provider == "gcs":
        if bucket_name is None:
            bucket_name = os.getenv("GCS_BUCKET_NAME")
        if not bucket_name:
            raise ValueError(
                "GCS storage requires bucket name. "
                "Set GCS_BUCKET_NAME environment variable."
            )

        # Lazy import to avoid requiring GCS package for local storage
        try:
            from .gcs import GCSFileStorage
        except ImportError:
            raise ImportError(
                "GCS storage requires google-cloud-storage. "
                "Install with: uv sync --extra gcs"
            )

        return GCSFileStorage(bucket_name)

    else:
        raise ValueError(
            f"Unknown storage provider: {provider}. "
            "Supported: 'local', 'gcs'"
        )
