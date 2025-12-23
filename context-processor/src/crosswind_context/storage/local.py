"""Local filesystem storage implementation."""

import logging
from pathlib import Path

from .base import FileStorage

logger = logging.getLogger(__name__)


class LocalFileStorage(FileStorage):
    """File storage using local filesystem.

    This is the default storage backend for OSS deployments.
    Files are read from a base directory, typically a mounted volume
    shared with the API server.

    Path structure:
        {base_path}/contexts/{context_id}/{filename}
    """

    def __init__(self, base_path: str):
        """Initialize local file storage.

        Args:
            base_path: Base directory for file storage.
                      Typically set via AGENT_EVAL_DATA_DIR env var.
        """
        self.base_path = Path(base_path)
        logger.info(f"Initialized local file storage at {self.base_path}")

    def _resolve_path(self, object_name: str) -> Path:
        """Resolve object name to full filesystem path.

        Args:
            object_name: Relative path from base directory.

        Returns:
            Absolute path to the file.
        """
        # Prevent path traversal attacks
        resolved = (self.base_path / object_name).resolve()
        if not str(resolved).startswith(str(self.base_path.resolve())):
            raise ValueError(f"Invalid path: {object_name}")
        return resolved

    def download(self, object_name: str) -> bytes:
        """Read a file from the local filesystem.

        Args:
            object_name: Relative path from base directory.

        Returns:
            File contents as bytes.

        Raises:
            FileNotFoundError: If file doesn't exist.
        """
        path = self._resolve_path(object_name)

        if not path.exists():
            raise FileNotFoundError(f"File not found: {object_name}")

        logger.debug(f"Reading file from {path}")
        return path.read_bytes()

    def exists(self, object_name: str) -> bool:
        """Check if a file exists on the local filesystem.

        Args:
            object_name: Relative path from base directory.

        Returns:
            True if file exists, False otherwise.
        """
        try:
            path = self._resolve_path(object_name)
            return path.exists() and path.is_file()
        except ValueError:
            return False

    def list_files(self, prefix: str) -> list[str]:
        """List files matching a prefix.

        Args:
            prefix: Directory prefix to search in.

        Returns:
            List of relative file paths.
        """
        try:
            base = self._resolve_path(prefix)
            if not base.exists():
                return []

            files = []
            for path in base.rglob("*"):
                if path.is_file():
                    # Return path relative to base_path
                    rel_path = path.relative_to(self.base_path)
                    files.append(str(rel_path))
            return files
        except ValueError:
            return []
