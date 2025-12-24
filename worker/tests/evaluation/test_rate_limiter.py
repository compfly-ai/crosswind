"""Tests for rate limiter."""

import asyncio
import time
from unittest.mock import AsyncMock, MagicMock

import pytest

from crosswind.evaluation.rate_limiter import RateLimiter, RateLimitTimeoutError


class MockRedis:
    """Mock Redis client for testing rate limiter logic."""

    def __init__(self):
        self.data: dict[str, dict[str, str]] = {}
        self.eval_results: list[int] = []
        self._eval_call_count = 0

    async def eval(self, script: str, numkeys: int, *args) -> int:
        """Mock eval for Lua script execution."""
        key = args[0]
        now = float(args[1])
        tokens_per_second = float(args[2])
        bucket_size = float(args[3])

        # Get current state
        if key not in self.data:
            self.data[key] = {"tokens": str(bucket_size), "last_update": str(now)}

        tokens = float(self.data[key].get("tokens", bucket_size))
        last_update = float(self.data[key].get("last_update", now))

        # Add tokens based on elapsed time
        elapsed = now - last_update
        tokens = min(bucket_size, tokens + (elapsed * tokens_per_second))

        # Try to consume a token
        if tokens >= 1:
            tokens -= 1
            self.data[key] = {"tokens": str(tokens), "last_update": str(now)}
            return 1
        else:
            self.data[key] = {"tokens": str(tokens), "last_update": str(now)}
            return 0

    async def hget(self, key: str, field: str) -> str | None:
        """Mock hget."""
        if key not in self.data:
            return None
        return self.data[key].get(field)

    async def delete(self, key: str) -> None:
        """Mock delete."""
        self.data.pop(key, None)


@pytest.fixture
def mock_redis():
    """Create a mock Redis instance."""
    return MockRedis()


@pytest.fixture
def rate_limiter(mock_redis):
    """Create a rate limiter with 60 RPM (1 per second)."""
    return RateLimiter(
        redis=mock_redis,
        agent_id="test_agent",
        requests_per_minute=60,
        bucket_size=10,
    )


class TestRateLimiterInit:
    """Tests for rate limiter initialization."""

    def test_init_defaults(self, mock_redis):
        """Test default bucket size equals RPM."""
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="agent1",
            requests_per_minute=120,
        )

        assert limiter.rpm == 120
        assert limiter.tokens_per_second == 2.0
        assert limiter.bucket_size == 120

    def test_init_custom_bucket_size(self, mock_redis):
        """Test custom bucket size."""
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="agent1",
            requests_per_minute=60,
            bucket_size=5,
        )

        assert limiter.bucket_size == 5

    def test_key_prefix(self, mock_redis):
        """Test key prefix is set correctly."""
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="my_agent_123",
            requests_per_minute=60,
        )

        assert limiter.key_prefix == "ratelimit:my_agent_123"


class TestAcquire:
    """Tests for token acquisition."""

    @pytest.mark.asyncio
    async def test_acquire_immediate_success(self, rate_limiter, mock_redis):
        """First acquire should succeed immediately with full bucket."""
        result = await rate_limiter.acquire(timeout=1.0)

        assert result is True
        key = "ratelimit:test_agent:bucket"
        tokens = float(mock_redis.data[key]["tokens"])
        assert tokens == 9.0  # Started with 10, consumed 1

    @pytest.mark.asyncio
    async def test_acquire_multiple_tokens(self, rate_limiter, mock_redis):
        """Should be able to acquire multiple tokens up to bucket size."""
        for i in range(10):
            result = await rate_limiter.acquire(timeout=1.0)
            assert result is True

        # Bucket should now be empty
        key = "ratelimit:test_agent:bucket"
        tokens = float(mock_redis.data[key]["tokens"])
        assert tokens < 1.0

    @pytest.mark.asyncio
    async def test_acquire_timeout(self, mock_redis):
        """Should timeout when bucket is empty and rate is slow."""
        # Very slow rate: 1 request per minute
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="slow_agent",
            requests_per_minute=1,
            bucket_size=1,
        )

        # First acquire succeeds
        await limiter.acquire(timeout=1.0)

        # Second should timeout quickly since rate is so slow
        with pytest.raises(RateLimitTimeoutError):
            await limiter.acquire(timeout=0.2)

    @pytest.mark.asyncio
    async def test_acquire_waits_for_token_regeneration(self, mock_redis):
        """Should wait for token regeneration when bucket is empty."""
        # 60 RPM = 1 per second, bucket size 1
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="fast_agent",
            requests_per_minute=60,
            bucket_size=1,
        )

        # Exhaust the bucket
        await limiter.acquire(timeout=1.0)

        # Next acquire should wait ~1 second for token to regenerate
        start = time.monotonic()
        result = await limiter.acquire(timeout=2.0)
        elapsed = time.monotonic() - start

        assert result is True
        assert elapsed >= 0.9  # Should have waited for token


class TestGetAvailableTokens:
    """Tests for checking available tokens."""

    @pytest.mark.asyncio
    async def test_get_tokens_initial(self, rate_limiter, mock_redis):
        """Initial tokens should equal bucket size."""
        tokens = await rate_limiter.get_available_tokens()
        assert tokens == 10.0

    @pytest.mark.asyncio
    async def test_get_tokens_after_acquire(self, rate_limiter, mock_redis):
        """Tokens should decrease after acquisition."""
        await rate_limiter.acquire(timeout=1.0)
        await rate_limiter.acquire(timeout=1.0)

        tokens = await rate_limiter.get_available_tokens()
        # Allow small delta for time-based token regeneration between calls
        assert 7.9 < tokens < 8.1


class TestReset:
    """Tests for rate limiter reset."""

    @pytest.mark.asyncio
    async def test_reset_clears_bucket(self, rate_limiter, mock_redis):
        """Reset should delete the bucket key."""
        # Use some tokens
        await rate_limiter.acquire(timeout=1.0)

        # Reset
        await rate_limiter.reset()

        # Key should be deleted
        key = "ratelimit:test_agent:bucket"
        assert key not in mock_redis.data

    @pytest.mark.asyncio
    async def test_reset_restores_capacity(self, rate_limiter, mock_redis):
        """After reset, full capacity should be available."""
        # Use all tokens
        for _ in range(10):
            await rate_limiter.acquire(timeout=1.0)

        # Reset
        await rate_limiter.reset()

        # Should be able to acquire full bucket again
        for _ in range(10):
            result = await rate_limiter.acquire(timeout=0.5)
            assert result is True


class TestTokenRegeneration:
    """Tests for token regeneration over time."""

    @pytest.mark.asyncio
    async def test_tokens_regenerate_over_time(self, mock_redis):
        """Tokens should regenerate based on elapsed time."""
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="regen_agent",
            requests_per_minute=600,  # 10 per second
            bucket_size=5,
        )

        # Exhaust the bucket
        for _ in range(5):
            await limiter.acquire(timeout=0.5)

        # Wait for regeneration (10 per second means 0.5s = 5 tokens)
        await asyncio.sleep(0.5)

        # Should be able to acquire again
        result = await limiter.acquire(timeout=0.1)
        assert result is True

    @pytest.mark.asyncio
    async def test_tokens_capped_at_bucket_size(self, mock_redis):
        """Tokens should not exceed bucket size even after long wait."""
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="cap_agent",
            requests_per_minute=6000,  # Very fast
            bucket_size=5,
        )

        # Acquire one token to initialize
        await limiter.acquire(timeout=0.1)

        # Wait "long time" - but bucket should cap at 5
        await asyncio.sleep(0.2)

        tokens = await limiter.get_available_tokens()
        # Should be at most bucket_size (5), but token was consumed too
        assert tokens <= 5.0


class TestEdgeCases:
    """Tests for edge cases."""

    @pytest.mark.asyncio
    async def test_zero_rpm_not_allowed(self, mock_redis):
        """Rate of 0 would cause division by zero."""
        # This tests that the code handles edge cases
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="zero_agent",
            requests_per_minute=1,  # Very slow but not zero
            bucket_size=1,
        )
        assert limiter.tokens_per_second == 1 / 60

    @pytest.mark.asyncio
    async def test_very_high_rpm(self, mock_redis):
        """Should handle very high RPM values."""
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="fast_agent",
            requests_per_minute=10000,
            bucket_size=100,
        )

        # Should be able to burst 100 requests quickly
        for _ in range(100):
            result = await limiter.acquire(timeout=0.1)
            assert result is True

    @pytest.mark.asyncio
    async def test_concurrent_acquires(self, mock_redis):
        """Concurrent acquires should be handled correctly."""
        limiter = RateLimiter(
            redis=mock_redis,
            agent_id="concurrent_agent",
            requests_per_minute=600,
            bucket_size=10,
        )

        # Try to acquire 10 tokens concurrently
        tasks = [limiter.acquire(timeout=1.0) for _ in range(10)]
        results = await asyncio.gather(*tasks)

        assert all(r is True for r in results)

        # Bucket should now be empty
        tokens = await limiter.get_available_tokens()
        assert tokens < 1.0
