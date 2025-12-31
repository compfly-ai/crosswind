"""Pytest configuration for The Gadget tests."""

import sys
from pathlib import Path

# Add parent directory to path so we can import server
sys.path.insert(0, str(Path(__file__).parent.parent))
