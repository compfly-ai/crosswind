"""Main entry point for the OSS context processor."""

import asyncio

from context_processor_core.context.processor import main

if __name__ == "__main__":
    asyncio.run(main())
