"""Google Cloud Storage implementation.

This module is only loaded when STORAGE_PROVIDER=gcs.
Requires the 'gcs' optional dependency: uv sync --extra gcs
"""

import logging

from .base import FileStorage

logger = logging.getLogger(__name__)


class GCSFileStorage(FileStorage):
    """File storage using Google Cloud Storage.

    Optional for OSS users who want to use GCS.
    Requires google-cloud-storage package to be installed.
    """

    def __init__(self, bucket_name: str):
        """Initialize GCS storage.

        Args:
            bucket_name: Name of the GCS bucket.

        Raises:
            ImportError: If google-cloud-storage is not installed.
        """
        try:
            from google.cloud import storage as gcs
        except ImportError:
            raise ImportError(
                "GCS storage requires google-cloud-storage. "
                "Install with: uv sync --extra gcs"
            )

        self.bucket_name = bucket_name
        self.client = gcs.Client()
        self.bucket = self.client.bucket(bucket_name)
        logger.info(f"Initialized GCS storage with bucket {bucket_name}")

    def download(self, object_name: str) -> bytes:
        """Download a file from GCS.

        Args:
            object_name: Object key in the bucket.

        Returns:
            File contents as bytes.

        Raises:
            FileNotFoundError: If object doesn't exist.
        """
        blob = self.bucket.blob(object_name)

        if not blob.exists():
            raise FileNotFoundError(f"Object not found in GCS: {object_name}")

        logger.debug(f"Downloading from GCS: {object_name}")
        return blob.download_as_bytes()

    def exists(self, object_name: str) -> bool:
        """Check if an object exists in GCS.

        Args:
            object_name: Object key in the bucket.

        Returns:
            True if object exists, False otherwise.
        """
        blob = self.bucket.blob(object_name)
        return blob.exists()

    def list_files(self, prefix: str) -> list[str]:
        """List objects matching a prefix in GCS.

        Args:
            prefix: Object key prefix to filter by.

        Returns:
            List of object keys matching the prefix.
        """
        blobs = self.client.list_blobs(self.bucket_name, prefix=prefix)
        return [blob.name for blob in blobs]
