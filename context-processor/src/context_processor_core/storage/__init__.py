"""Storage abstraction for context processor.

Supports local filesystem (default) and GCS (optional).
"""

from .base import FileStorage
from .factory import create_storage

__all__ = ["FileStorage", "create_storage"]
