"""Base storage interface for file operations."""

from abc import ABC, abstractmethod


class FileStorage(ABC):
    """Abstract base class for file storage backends.

    Implementations:
    - LocalFileStorage: Reads from local filesystem (default for OSS)
    - GCSFileStorage: Reads from Google Cloud Storage (optional)
    """

    @abstractmethod
    def download(self, object_name: str) -> bytes:
        """Download a file and return its contents.

        Args:
            object_name: The path/key of the file to download.
                        For local: relative path from base directory
                        For GCS: object name in bucket

        Returns:
            The file contents as bytes.

        Raises:
            FileNotFoundError: If the file does not exist.
            Exception: For other storage errors.
        """
        pass

    @abstractmethod
    def exists(self, object_name: str) -> bool:
        """Check if a file exists.

        Args:
            object_name: The path/key of the file to check.

        Returns:
            True if the file exists, False otherwise.
        """
        pass

    @abstractmethod
    def list_files(self, prefix: str) -> list[str]:
        """List files with a given prefix.

        Args:
            prefix: The prefix to filter files by.

        Returns:
            List of file paths/keys matching the prefix.
        """
        pass
