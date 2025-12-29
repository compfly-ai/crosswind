"""Main entry point for the evaluation worker."""

import asyncio
import logging
import signal
import sys
from datetime import UTC, datetime
from typing import Any

import structlog
from motor.motor_asyncio import AsyncIOMotorClient
from redis.asyncio import Redis

from crosswind.config import settings
from crosswind.evaluation.runner import EvalRunner
from crosswind.protocols import create_adapter
from crosswind.storage import AnalyticsStorage, create_storage

# Map string log levels to numeric values
LOG_LEVELS = {
    "DEBUG": logging.DEBUG,      # 10
    "INFO": logging.INFO,        # 20
    "WARNING": logging.WARNING,  # 30
    "ERROR": logging.ERROR,      # 40
}

logger = structlog.get_logger()


class Worker:
    """Main worker class that processes evaluation jobs."""

    def __init__(self) -> None:
        self.running = False
        self.mongo_client: AsyncIOMotorClient[Any] | None = None
        self.redis_client: Redis | None = None
        self.storage: AnalyticsStorage | None = None

    async def setup(self) -> None:
        """Initialize connections to databases and services."""
        logger.info("Setting up worker connections")

        # Connect to MongoDB
        self.mongo_client = AsyncIOMotorClient(settings.mongo_uri)
        self.db = self.mongo_client[settings.database_name]

        # Connect to Redis
        self.redis_client = Redis.from_url(settings.redis_url)

        # Verify connections
        await self.mongo_client.admin.command("ping")
        await self.redis_client.ping()

        # Connect to analytics storage
        self.storage = await create_storage()

        logger.info("Worker connections established")

    async def cleanup(self) -> None:
        """Clean up connections."""
        logger.info("Cleaning up worker connections")

        if self.storage:
            await self.storage.close()

        if self.mongo_client:
            self.mongo_client.close()

        if self.redis_client:
            await self.redis_client.aclose()

    async def process_job(self, job_data: dict[str, Any]) -> None:
        """Process a single evaluation job."""
        run_id = job_data.get("runId")
        agent_id = job_data.get("agentId")
        mode = job_data.get("mode")
        eval_type = job_data.get("evalType", "red_team")
        scenario_set_ids = job_data.get("scenarioSetIds", [])
        include_built_in_datasets = job_data.get("includeBuiltInDatasets", False)

        log = logger.bind(
            run_id=run_id,
            agent_id=agent_id,
            mode=mode,
            eval_type=eval_type,
            scenario_set_ids=scenario_set_ids,
            include_built_in_datasets=include_built_in_datasets,
        )
        log.info("Processing evaluation job")

        try:
            # Update status to running
            await self.db.evalRuns.update_one(
                {"runId": run_id},
                {"$set": {"status": "running", "startedAt": datetime.now(UTC)}},
            )

            # Get agent configuration by agentId
            agent = await self.db.agents.find_one({"agentId": agent_id})
            if not agent:
                raise ValueError(f"Agent not found: {agent_id}")

            # Create protocol adapter
            adapter = create_adapter(agent)

            # Ensure required values are present
            if not self.redis_client:
                raise RuntimeError("Redis client not initialized")
            if not run_id:
                raise ValueError("run_id is required")
            if not mode:
                raise ValueError("mode is required")

            # Create and run evaluation
            runner = EvalRunner(
                adapter=adapter,
                db=self.db,
                redis=self.redis_client,
                storage=self.storage,
                agent=agent,
                run_id=str(run_id),
                mode=str(mode),
                eval_type=eval_type,
                scenario_set_ids=scenario_set_ids,
                include_built_in_datasets=include_built_in_datasets,
            )

            await runner.run()

            log.info("Evaluation job completed successfully")

        except Exception as e:
            log.error("Evaluation job failed", error=str(e))
            await self.db.evalRuns.update_one(
                {"runId": run_id},
                {
                    "$set": {"status": "failed"},
                    "$push": {
                        "errors": {
                            "timestamp": datetime.now(UTC),
                            "type": "worker_error",
                            "message": str(e),
                        }
                    },
                },
            )
            raise

    async def run(self) -> None:
        """Main worker loop."""
        self.running = True
        logger.info("Worker started, waiting for jobs")

        while self.running:
            try:
                if not self.redis_client:
                    raise RuntimeError("Redis client not initialized")

                # Block waiting for a job (BRPOP)
                result = await self.redis_client.brpop(["eval_jobs"], timeout=5)

                if result is None:
                    continue

                _, job_json = result
                import json

                job_data = json.loads(job_json)

                await self.process_job(job_data)

            except asyncio.CancelledError:
                logger.info("Worker cancelled")
                break
            except Exception as e:
                logger.error("Error processing job", error=str(e))
                await asyncio.sleep(settings.retry_delay_seconds)

    def stop(self) -> None:
        """Signal the worker to stop."""
        self.running = False


async def main() -> None:
    """Main entry point."""
    # Configure structured logging with configurable level
    log_level = LOG_LEVELS.get(settings.log_level.upper(), logging.INFO)

    structlog.configure(
        processors=[
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.add_log_level,
            structlog.processors.StackInfoRenderer(),
            structlog.dev.ConsoleRenderer() if sys.stdout.isatty() else structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(log_level),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
    )

    structlog.get_logger().info("Worker starting", log_level=settings.log_level)

    worker = Worker()

    # Set up signal handlers
    loop = asyncio.get_event_loop()

    def signal_handler() -> None:
        logger.info("Received shutdown signal")
        worker.stop()

    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, signal_handler)

    try:
        await worker.setup()
        await worker.run()
    finally:
        await worker.cleanup()


if __name__ == "__main__":
    asyncio.run(main())
