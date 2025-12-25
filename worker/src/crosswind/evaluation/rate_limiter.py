"""Token bucket rate limiter with Redis backing."""

import asyncio
import time

import structlog
from redis.asyncio import Redis

logger = structlog.get_logger()


class RateLimitTimeoutError(Exception):
    """Raised when rate limit token cannot be acquired within timeout."""

    pass


class RateLimiter:
    """Token bucket rate limiter backed by Redis.

    Implements a token bucket algorithm where tokens are added at a fixed rate
    and consumed when making requests. This allows for burst capacity up to
    the bucket size while maintaining an average rate.
    """

    def __init__(
        self,
        redis: Redis,
        agent_id: str,
        requests_per_minute: int,
        bucket_size: int | None = None,
    ) -> None:
        """Initialize the rate limiter.

        Args:
            redis: Redis client instance
            agent_id: Agent ID
            requests_per_minute: Target rate in requests per minute
            bucket_size: Maximum burst capacity (defaults to requests_per_minute)
        """
        self.redis = redis
        self.key_prefix = f"ratelimit:{agent_id}"
        self.rpm = requests_per_minute
        self.tokens_per_second = requests_per_minute / 60
        self.bucket_size = bucket_size or requests_per_minute

        # Lua script for atomic token bucket operation
        self._acquire_script = """
        local key = KEYS[1]
        local now = tonumber(ARGV[1])
        local tokens_per_second = tonumber(ARGV[2])
        local bucket_size = tonumber(ARGV[3])

        local bucket = redis.call('HMGET', key, 'tokens', 'last_update')
        local tokens = tonumber(bucket[1]) or bucket_size
        local last_update = tonumber(bucket[2]) or now

        -- Add tokens based on time elapsed
        local elapsed = now - last_update
        tokens = math.min(bucket_size, tokens + (elapsed * tokens_per_second))

        if tokens >= 1 then
            tokens = tokens - 1
            redis.call('HMSET', key, 'tokens', tokens, 'last_update', now)
            redis.call('EXPIRE', key, 120)
            return 1
        else
            redis.call('HMSET', key, 'tokens', tokens, 'last_update', now)
            redis.call('EXPIRE', key, 120)
            return 0
        end
        """

    async def acquire(self, timeout: float = 60.0) -> bool:
        """Acquire a rate limit token.

        Blocks until a token is available or timeout is reached.

        Args:
            timeout: Maximum time to wait in seconds

        Returns:
            True if token acquired

        Raises:
            RateLimitTimeoutError: If timeout is reached
        """
        start_time = time.monotonic()
        key = f"{self.key_prefix}:bucket"

        while True:
            now = time.time()
            result: int = await self.redis.eval(  # type: ignore[misc]
                self._acquire_script,
                1,
                key,
                str(now),
                str(self.tokens_per_second),
                str(self.bucket_size),
            )

            if result == 1:
                return True

            elapsed = time.monotonic() - start_time
            if elapsed >= timeout:
                raise RateLimitTimeoutError(
                    f"Could not acquire rate limit token within {timeout}s"
                )

            # Calculate wait time based on token regeneration rate
            wait_time = min(1.0 / self.tokens_per_second, timeout - elapsed)
            await asyncio.sleep(max(0.1, wait_time))

    async def get_available_tokens(self) -> float:
        """Get the current number of available tokens.

        Returns:
            Number of tokens available
        """
        key = f"{self.key_prefix}:bucket"
        result: bytes | None = await self.redis.hget(key, "tokens")  # type: ignore[misc]
        if result is None:
            return float(self.bucket_size)
        return float(result)

    async def reset(self) -> None:
        """Reset the rate limiter to full capacity."""
        key = f"{self.key_prefix}:bucket"
        await self.redis.delete(key)
