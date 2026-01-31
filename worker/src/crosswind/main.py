"""Main entry point for the evaluation worker."""

import asyncio
import json
import logging
import os
import random
import signal
import sys
import uuid
from datetime import UTC, datetime
from typing import Any

import structlog
from motor.motor_asyncio import AsyncIOMotorClient
from redis.asyncio import Redis

from crosswind.config import settings
from crosswind.evaluation.runner import EvalRunner
from crosswind.protocols import create_adapter
from crosswind.storage import AnalyticsStorage, create_storage

LOG_LEVELS = {
    "DEBUG": logging.DEBUG,
    "INFO": logging.INFO,
    "WARNING": logging.WARNING,
    "ERROR": logging.ERROR,
}

logger = structlog.get_logger()

HEARTBEAT_INTERVAL = 10
HEARTBEAT_TTL = 30
RECLAIM_INTERVAL = 60
AGENT_LOCK_TTL = 3600
DEQUEUE_TIMEOUT = 0.1


class Worker:
    """Evaluation worker with per-agent queue isolation, reliable delivery, and concurrent eval support."""

    def __init__(self) -> None:
        self.running = False
        self.worker_id = os.environ.get("HOSTNAME", str(uuid.uuid4()))
        self.mongo_client: AsyncIOMotorClient[Any] | None = None
        self.redis_client: Redis | None = None
        self.storage: AnalyticsStorage | None = None
        self.eval_semaphore = asyncio.Semaphore(settings.worker_concurrency)
        self._active_tasks: set[asyncio.Task[None]] = set()

    async def setup(self) -> None:
        """Initialize connections and reclaim stale jobs from a previous crash."""
        logger.info("Setting up worker connections", worker_id=self.worker_id)

        self.mongo_client = AsyncIOMotorClient(settings.mongo_uri)
        self.db = self.mongo_client[settings.database_name]
        self.redis_client = Redis.from_url(
            settings.redis_url, socket_timeout=5.0, socket_connect_timeout=5.0,
        )

        await self.mongo_client.admin.command("ping")
        await self.redis_client.ping()  # type: ignore[misc]

        self.storage = await create_storage()

        await self._reclaim_own_stale_jobs()

        logger.info("Worker connections established", worker_id=self.worker_id)

    async def cleanup(self) -> None:
        """Clean up connections."""
        logger.info("Cleaning up worker connections")

        try:
            if self.storage:
                await self.storage.close()
        except Exception as e:
            logger.error("Error closing storage", error=str(e))
        try:
            if self.mongo_client:
                self.mongo_client.close()
        except Exception as e:
            logger.error("Error closing MongoDB", error=str(e))
        try:
            if self.redis_client:
                await self.redis_client.aclose()
        except Exception as e:
            logger.error("Error closing Redis", error=str(e))

    async def _reclaim_own_stale_jobs(self) -> None:
        """On startup, re-queue any jobs left in our processing list from a previous crash."""
        if not self.redis_client:
            raise RuntimeError("Redis client not initialized")
        processing_key = f"eval_processing:{self.worker_id}"
        stale_jobs = await self.redis_client.lrange(processing_key, 0, -1)

        for job_raw in stale_jobs:
            job_data = json.loads(job_raw)
            agent_id = job_data["agentId"]
            await self.redis_client.lpush(f"eval_jobs:{agent_id}", job_raw)
            await self.redis_client.sadd("eval_agents", agent_id)

            # Only delete the lock if we were the holder
            lock_holder = await self.redis_client.get(f"eval_lock:{agent_id}")
            if lock_holder and lock_holder.decode() == self.worker_id:
                await self.redis_client.delete(f"eval_lock:{agent_id}")

        await self.redis_client.delete(processing_key)
        if stale_jobs:
            logger.info("Reclaimed stale jobs on startup", count=len(stale_jobs))

    async def _heartbeat_loop(self) -> None:
        """Background task that publishes liveness to Redis."""
        if not self.redis_client:
            raise RuntimeError("Redis client not initialized")
        while self.running:
            try:
                await self.redis_client.set(
                    f"eval_heartbeat:{self.worker_id}", "alive", ex=HEARTBEAT_TTL
                )
            except Exception as e:
                logger.error("Heartbeat write failed", error=str(e))
            await asyncio.sleep(HEARTBEAT_INTERVAL)

    async def _reclaim_loop(self) -> None:
        """Background task that reclaims jobs from dead workers."""
        if not self.redis_client:
            raise RuntimeError("Redis client not initialized")
        while self.running:
            await asyncio.sleep(RECLAIM_INTERVAL)
            try:
                await self._reclaim_dead_worker_jobs()
            except Exception as e:
                logger.error("Reclaim sweep failed", error=str(e))

    async def _reclaim_dead_worker_jobs(self) -> None:
        """Scan for dead workers and re-queue their in-flight jobs."""
        if not self.redis_client:
            raise RuntimeError("Redis client not initialized")

        acquired = await self.redis_client.set(
            "eval_reclaim_lock", self.worker_id, nx=True, ex=RECLAIM_INTERVAL
        )
        if not acquired:
            return

        cursor: int | bytes = 0
        while True:
            cursor, keys = await self.redis_client.scan(
                cursor=int(cursor), match="eval_processing:*", count=100
            )
            for key in keys:
                key_str = key.decode() if isinstance(key, bytes) else key
                dead_worker_id = key_str.split(":")[-1]
                if dead_worker_id == self.worker_id:
                    continue

                alive = await self.redis_client.exists(f"eval_heartbeat:{dead_worker_id}")
                if alive:
                    continue

                while True:
                    job_raw = await self.redis_client.rpop(key_str)
                    if not job_raw:
                        break
                    job_data = json.loads(job_raw)
                    agent_id = job_data["agentId"]
                    await self.redis_client.lpush(f"eval_jobs:{agent_id}", job_raw)
                    await self.redis_client.sadd("eval_agents", agent_id)

                    lock_holder = await self.redis_client.get(f"eval_lock:{agent_id}")
                    if lock_holder and lock_holder.decode() == dead_worker_id:
                        await self.redis_client.delete(f"eval_lock:{agent_id}")

                    logger.info("Reclaimed job from dead worker",
                                dead_worker=dead_worker_id, agent_id=agent_id)

                # Clean up empty processing key
                await self.redis_client.delete(key_str)

            if not cursor:
                break

    async def _dequeue_job(self) -> tuple[dict[str, Any], bytes] | None:
        """Try to dequeue a job from any agent queue, respecting per-agent locks.

        Only attempts dequeue when the semaphore has capacity, to avoid
        holding agent locks while waiting for a concurrency slot.
        """
        if not self.redis_client:
            raise RuntimeError("Redis client not initialized")

        # Don't dequeue if all concurrency slots are occupied
        if self.eval_semaphore._value <= 0:  # noqa: SLF001
            return None

        agent_ids = await self.redis_client.smembers("eval_agents")
        agent_list = [
            a.decode() if isinstance(a, bytes) else a
            for a in agent_ids
        ]
        random.shuffle(agent_list)

        for agent_id in agent_list:
            locked = await self.redis_client.set(
                f"eval_lock:{agent_id}", self.worker_id, nx=True, ex=AGENT_LOCK_TTL
            )
            if not locked:
                continue

            job_raw = await self.redis_client.execute_command(
                "BLMOVE",
                f"eval_jobs:{agent_id}",
                f"eval_processing:{self.worker_id}",
                "LEFT", "RIGHT",
                DEQUEUE_TIMEOUT,
            )
            if job_raw:
                job_data = json.loads(job_raw)
                return job_data, job_raw

            # Empty queue — release lock. Check queue length before removing
            # from eval_agents to avoid racing with a concurrent enqueue.
            await self.redis_client.delete(f"eval_lock:{agent_id}")
            queue_len = await self.redis_client.llen(f"eval_jobs:{agent_id}")
            if queue_len == 0:
                await self.redis_client.srem("eval_agents", agent_id)

        return None

    async def _release_agent(self, agent_id: str, job_raw: bytes) -> None:
        """Release the per-agent lock and remove the job from the processing list."""
        if not self.redis_client:
            raise RuntimeError("Redis client not initialized")
        await self.redis_client.delete(f"eval_lock:{agent_id}")
        await self.redis_client.lrem(f"eval_processing:{self.worker_id}", 1, job_raw)

    async def process_job(self, job_data: dict[str, Any]) -> None:
        """Process a single evaluation job."""
        run_id = job_data.get("runId")
        agent_id = job_data.get("agentId")
        mode = job_data.get("mode")
        eval_type = job_data.get("evalType", "red_team")
        scenario_set_ids = job_data.get("scenarioSetIds", [])
        include_built_in_datasets = job_data.get("includeBuiltInDatasets", False)

        if not run_id or not agent_id or not mode:
            raise ValueError(f"Missing required job fields: runId={run_id}, agentId={agent_id}, mode={mode}")

        log = logger.bind(
            run_id=run_id, agent_id=agent_id, mode=mode,
            eval_type=eval_type, worker_id=self.worker_id,
        )

        # Check if cancelled before starting
        eval_run = await self.db.evalRuns.find_one({"runId": run_id})
        if eval_run and eval_run.get("status") == "cancelled":
            log.info("Skipping cancelled eval")
            return

        log.info("Processing evaluation job")

        try:
            await self.db.evalRuns.update_one(
                {"runId": run_id},
                {"$set": {"status": "running", "startedAt": datetime.now(UTC)}},
            )

            agent = await self.db.agents.find_one({"agentId": agent_id})
            if not agent:
                raise ValueError(f"Agent not found: {agent_id}")

            if not self.redis_client:
                raise RuntimeError("Redis client not initialized")

            adapter = create_adapter(agent)

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
            try:
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
            except Exception as db_err:
                log.error("Failed to mark eval as failed in MongoDB", error=str(db_err))
            raise

    async def run(self) -> None:
        """Main worker loop with concurrent eval support."""
        self.running = True
        logger.info("Worker started", worker_id=self.worker_id,
                     concurrency=settings.worker_concurrency)

        heartbeat_task = asyncio.create_task(self._heartbeat_loop())
        reclaim_task = asyncio.create_task(self._reclaim_loop())

        try:
            while self.running:
                try:
                    result = await self._dequeue_job()
                    if result is None:
                        await asyncio.sleep(1)
                        continue

                    job_data, job_raw = result
                    task = asyncio.create_task(
                        self._run_eval(job_data, job_raw)
                    )
                    self._active_tasks.add(task)
                    task.add_done_callback(self._active_tasks.discard)

                except asyncio.CancelledError:
                    logger.info("Worker cancelled")
                    break
                except Exception as e:
                    logger.error("Error in dequeue loop", error=str(e))
                    await asyncio.sleep(settings.retry_delay_seconds)
        finally:
            heartbeat_task.cancel()
            reclaim_task.cancel()

            # Wait for in-flight evals to finish gracefully
            if self._active_tasks:
                logger.info("Waiting for in-flight evals to complete",
                            count=len(self._active_tasks))
                await asyncio.gather(*self._active_tasks, return_exceptions=True)

    async def _run_eval(self, job_data: dict[str, Any], job_raw: bytes) -> None:
        """Run a single eval within the concurrency semaphore."""
        agent_id = job_data.get("agentId", "")
        async with self.eval_semaphore:
            try:
                await self.process_job(job_data)
            except Exception as e:
                logger.error("Eval task failed", error=str(e),
                             run_id=job_data.get("runId"))
            finally:
                await self._release_agent(agent_id, job_raw)

    def stop(self) -> None:
        """Signal the worker to stop."""
        self.running = False


async def main() -> None:
    """Main entry point."""
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

    loop = asyncio.get_running_loop()

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
