"""Factory for creating analytics storage backends."""

import structlog

from crosswind.config import settings
from crosswind.storage.base import AnalyticsStorage

logger = structlog.get_logger()


class NullStorage(AnalyticsStorage):
    """No-op storage backend when analytics is disabled."""

    async def connect(self) -> None:
        logger.info("Analytics storage disabled")

    async def close(self) -> None:
        pass

    def is_connected(self) -> bool:
        return False

    def add_eval_detail(self, detail) -> None:  # type: ignore[no-untyped-def]
        pass

    def add_session(self, session) -> None:  # type: ignore[no-untyped-def]
        pass

    def flush(self) -> None:
        pass


async def create_storage() -> AnalyticsStorage:
    """Create and connect the appropriate storage backend.

    Uses the `analytics_backend` config setting to determine which
    backend to use:
    - "duckdb": Embedded DuckDB (default, no server required)
    - "clickhouse": External ClickHouse server
    - "none": Disable analytics storage

    Returns:
        Connected AnalyticsStorage instance
    """
    backend = settings.analytics_backend.lower()
    storage: AnalyticsStorage

    if backend == "none" or backend == "disabled":
        storage = NullStorage()
        await storage.connect()
        return storage

    elif backend == "duckdb":
        from crosswind.storage.duckdb_storage import DuckDBStorage

        storage = DuckDBStorage()
        await storage.connect()
        return storage

    elif backend == "clickhouse":
        from crosswind.storage.clickhouse_storage import ClickHouseStorage

        storage = ClickHouseStorage()
        await storage.connect()
        return storage

    else:
        logger.warning(
            "Unknown analytics backend, using DuckDB",
            backend=backend,
            valid_options=["duckdb", "clickhouse", "none"],
        )
        from crosswind.storage.duckdb_storage import DuckDBStorage

        storage = DuckDBStorage()
        await storage.connect()
        return storage
