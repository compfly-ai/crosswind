"""Context document processor for OSS.

This module polls MongoDB for context documents in 'processing' status,
downloads files from storage, extracts text, and updates the database.

Supports running multiple instances - uses atomic MongoDB operations to
prevent duplicate processing.

Storage backends:
- Local filesystem (default): Set STORAGE_PROVIDER=local
- Google Cloud Storage (optional): Set STORAGE_PROVIDER=gcs

Usage:
    # Run as standalone processor
    python -m src.main

    # Or import and use
    from src.context.processor import ContextProcessor
    processor = ContextProcessor(mongo_db)
    await processor.process_pending_contexts()
"""

import asyncio
import logging
import os
import socket
from datetime import datetime, timedelta
from typing import Optional

from .extractor import MAX_CHARS_TOTAL, TextExtractor
from crosswind_context.storage import create_storage

logger = logging.getLogger(__name__)

# Worker identification for distributed locking
WORKER_ID = f"{socket.gethostname()}-{os.getpid()}"


class ContextProcessor:
    """Processes context documents to extract text content.

    Supports multiple concurrent instances via atomic MongoDB claims.
    OSS version - no multi-tenancy (no orgId filtering).
    """

    def __init__(
        self,
        mongo_db,
        poll_interval: int = 30,
        claim_timeout_minutes: int = 10,
        storage_provider: str | None = None,
    ):
        """Initialize the context processor.

        Args:
            mongo_db: MongoDB database instance
            poll_interval: Seconds between polls for new contexts
            claim_timeout_minutes: Minutes before a claimed context can be reclaimed
            storage_provider: Storage backend ("local" or "gcs"). Defaults to STORAGE_PROVIDER env var.
        """
        self.db = mongo_db
        self.contexts_collection = mongo_db["contexts"]
        self.storage = create_storage(provider=storage_provider)
        self.poll_interval = poll_interval
        self.claim_timeout = timedelta(minutes=claim_timeout_minutes)
        self.extractor = TextExtractor()

        logger.info("Context processor initialized (OSS mode)")

    async def claim_context(self) -> Optional[dict]:
        """Atomically claim a context for processing.

        Uses findOneAndUpdate to prevent race conditions when multiple
        workers are running.

        Returns:
            The claimed context document, or None if no work available.
        """
        now = datetime.utcnow()
        claim_expiry = now - self.claim_timeout

        # Find and claim a context that is either:
        # 1. In 'processing' status and not claimed
        # 2. In 'processing' status with an expired claim (worker died)
        result = await self.contexts_collection.find_one_and_update(
            {
                "status": "processing",
                "$or": [
                    {"claimedBy": {"$exists": False}},
                    {"claimedBy": None},
                    {"claimedAt": {"$lt": claim_expiry}},
                ],
            },
            {
                "$set": {
                    "claimedBy": WORKER_ID,
                    "claimedAt": now,
                }
            },
            return_document=True,
        )

        if result:
            logger.info(
                f"Claimed context {result.get('contextId')} for processing",
                extra={"worker_id": WORKER_ID},
            )

        return result

    async def release_claim(self, context_id: str) -> None:
        """Release the claim on a context (after successful processing)."""
        await self.contexts_collection.update_one(
            {"contextId": context_id},
            {"$unset": {"claimedBy": "", "claimedAt": ""}},
        )

    async def process_pending_contexts(self) -> int:
        """Process all pending contexts.

        Uses atomic claiming to support multiple workers.

        Returns:
            Number of contexts processed
        """
        processed = 0

        while True:
            # Try to claim a context
            context = await self.claim_context()
            if context is None:
                break  # No more work available

            try:
                await self.process_context(context)
                processed += 1
            except Exception as e:
                logger.error(
                    f"Failed to process context {context.get('contextId')}: {e}",
                    exc_info=True,
                )
                # Mark as failed and release claim
                await self.contexts_collection.update_one(
                    {"contextId": context["contextId"]},
                    {
                        "$set": {
                            "status": "failed",
                            "error": str(e),
                            "updatedAt": datetime.utcnow(),
                        },
                        "$unset": {"claimedBy": "", "claimedAt": ""},
                    },
                )

        return processed

    async def process_context(self, context: dict) -> None:
        """Process a single context document.

        Downloads files from storage, extracts text, and updates MongoDB.
        """
        context_id = context["contextId"]
        logger.info(f"Processing context {context_id}")

        files = context.get("files", [])
        total_chars = 0
        ready_count = 0
        failed_count = 0

        for i, file_info in enumerate(files):
            if file_info.get("status") not in ("processing", "uploading"):
                # Skip already processed or failed files
                if file_info.get("status") == "ready":
                    ready_count += 1
                continue

            file_name = file_info["name"]
            # Storage path - works for both local and GCS
            storage_path = file_info.get("storagePath") or file_info.get("gcsObjectName")
            content_type = file_info["contentType"]

            try:
                # Download from storage (local filesystem or GCS)
                content = self.storage.download(storage_path)

                # Extract text
                extracted_text, metadata = self.extractor.extract(content, content_type)

                if "error" in metadata:
                    raise Exception(metadata["error"])

                # Truncate if we're approaching total limit
                remaining_chars = MAX_CHARS_TOTAL - total_chars
                if len(extracted_text) > remaining_chars:
                    extracted_text = extracted_text[:remaining_chars]
                    extracted_text += "\n\n[... truncated due to context size limits ...]"

                char_count = len(extracted_text)
                total_chars += char_count

                # Update file in MongoDB
                update_fields = {
                    f"files.{i}.status": "ready",
                    f"files.{i}.extractedText": extracted_text,
                    f"files.{i}.extractedChars": char_count,
                    "updatedAt": datetime.utcnow(),
                }

                if "page_count" in metadata:
                    update_fields[f"files.{i}.pageCount"] = metadata["page_count"]
                if "row_count" in metadata:
                    update_fields[f"files.{i}.rowCount"] = metadata["row_count"]

                await self.contexts_collection.update_one(
                    {"contextId": context_id},
                    {"$set": update_fields},
                )

                ready_count += 1
                logger.info(
                    f"Extracted {char_count} chars from {file_name} in context {context_id}"
                )

            except Exception as e:
                logger.error(f"Failed to process file {file_name}: {e}")
                failed_count += 1

                await self.contexts_collection.update_one(
                    {"contextId": context_id},
                    {
                        "$set": {
                            f"files.{i}.status": "failed",
                            f"files.{i}.error": str(e),
                            "updatedAt": datetime.utcnow(),
                        }
                    },
                )

        # Update context status
        if failed_count == len(files):
            final_status = "failed"
            error_msg = "All files failed to process"
        else:
            final_status = "ready"
            error_msg = None

        summary = {
            "totalFiles": len(files),
            "readyFiles": ready_count,
            "failedFiles": failed_count,
            "extractedTokens": total_chars // 4,  # Rough token estimate
            "totalSize": sum(f.get("size", 0) for f in files),
        }

        update = {
            "status": final_status,
            "summary": summary,
            "updatedAt": datetime.utcnow(),
        }
        if error_msg:
            update["error"] = error_msg

        # Update status and release claim atomically
        await self.contexts_collection.update_one(
            {"contextId": context_id},
            {
                "$set": update,
                "$unset": {"claimedBy": "", "claimedAt": ""},
            },
        )

        logger.info(
            f"Context {context_id} processing complete: "
            f"status={final_status}, ready={ready_count}, failed={failed_count}"
        )

    async def run(self) -> None:
        """Run the processor in a loop."""
        logger.info(
            f"Starting context processor, polling every {self.poll_interval}s"
        )

        while True:
            try:
                processed = await self.process_pending_contexts()
                if processed > 0:
                    logger.info(f"Processed {processed} contexts")
            except Exception as e:
                logger.error(f"Error in processing loop: {e}", exc_info=True)

            await asyncio.sleep(self.poll_interval)


async def main():
    """Main entry point for standalone processor."""
    from motor.motor_asyncio import AsyncIOMotorClient

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    )

    mongo_uri = os.getenv("MONGO_URI", "mongodb://localhost:27017")
    db_name = os.getenv("DATABASE_NAME", "agent_eval")

    client = AsyncIOMotorClient(mongo_uri)
    db = client[db_name]

    processor = ContextProcessor(db)
    await processor.run()


if __name__ == "__main__":
    asyncio.run(main())
