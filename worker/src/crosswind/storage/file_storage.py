"""File storage for saving report files."""

from abc import ABC, abstractmethod
from pathlib import Path

import structlog

from crosswind.config import settings

logger = structlog.get_logger()


class FileStorage(ABC):
    """Abstract base class for file storage."""

    @abstractmethod
    async def upload(self, path: str, content: bytes, content_type: str) -> None:
        """Upload content to storage."""
        pass

    @abstractmethod
    async def download(self, path: str) -> bytes:
        """Download content from storage."""
        pass

    @abstractmethod
    async def exists(self, path: str) -> bool:
        """Check if a file exists."""
        pass


class LocalFileStorage(FileStorage):
    """Local filesystem storage implementation."""

    def __init__(self, base_path: str | None = None) -> None:
        self.base_path = Path(base_path or settings.data_dir)
        self.base_path.mkdir(parents=True, exist_ok=True)
        logger.info("Initialized local file storage", base_path=str(self.base_path))

    def _full_path(self, path: str) -> Path:
        """Get full path, preventing directory traversal."""
        clean_path = path.lstrip("/").lstrip(".")
        return self.base_path / clean_path

    async def upload(self, path: str, content: bytes, content_type: str) -> None:
        """Upload content to local filesystem."""
        full_path = self._full_path(path)
        full_path.parent.mkdir(parents=True, exist_ok=True)

        full_path.write_bytes(content)
        logger.debug("Uploaded file", path=path, size=len(content))

    async def download(self, path: str) -> bytes:
        """Download content from local filesystem."""
        full_path = self._full_path(path)

        if not full_path.exists():
            raise FileNotFoundError(f"File not found: {path}")

        return full_path.read_bytes()

    async def exists(self, path: str) -> bool:
        """Check if a file exists."""
        full_path = self._full_path(path)
        return full_path.exists()


def create_file_storage() -> FileStorage:
    """Create file storage based on configuration."""
    provider = settings.storage_provider

    if provider == "local":
        return LocalFileStorage()
    elif provider == "gcs":
        # GCS support can be added later
        raise NotImplementedError("GCS file storage not yet implemented in worker")
    else:
        raise ValueError(f"Unknown storage provider: {provider}")
