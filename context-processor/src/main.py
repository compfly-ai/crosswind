"""Main entry point for the OSS context processor."""

import asyncio

from crosswind_context.context.processor import main

if __name__ == "__main__":
    asyncio.run(main())
