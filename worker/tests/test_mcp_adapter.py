"""MCP adapter tests.

Tests MCP protocol behavior:
- Transport selection (SSE vs streamable_http)
- Connection failures
- Timeout handling
- Resource cleanup

For eval-specific tests (prompt mapping, content extraction), see test_eval_runner_mcp.py.
For registration tests (agent_doc → adapter), see test_protocol_selection.py.
"""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from crosswind.models import AuthConfig
from crosswind.protocols.mcp_adapter import MCPAdapter


# =============================================================================
# Transport Selection
# =============================================================================


class TestTransportSelection:
    """Test that the correct MCP client is used based on transport config."""

    @pytest.mark.asyncio
    async def test_sse_transport_uses_sse_client(self):
        """SSE transport should use sse_client."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="sse",
        )

        with patch("crosswind.protocols.mcp_adapter.sse_client") as mock_sse, \
             patch("crosswind.protocols.mcp_adapter.streamable_http_client") as mock_http:

            # Setup mock to return async context manager
            mock_ctx = MagicMock()
            mock_ctx.__aenter__ = AsyncMock(return_value=(MagicMock(), MagicMock(), MagicMock()))
            mock_ctx.__aexit__ = AsyncMock()
            mock_sse.return_value = mock_ctx

            # Mock ClientSession
            with patch("crosswind.protocols.mcp_adapter.ClientSession") as mock_session_cls:
                mock_session = MagicMock()
                mock_session.__aenter__ = AsyncMock(return_value=mock_session)
                mock_session.__aexit__ = AsyncMock()
                mock_session.initialize = AsyncMock()
                mock_session_cls.return_value = mock_session

                await adapter._ensure_session()

            mock_sse.assert_called_once()
            mock_http.assert_not_called()

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_streamable_http_transport_uses_http_client(self):
        """Streamable HTTP transport should use streamable_http_client."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="streamable_http",
        )

        with patch("crosswind.protocols.mcp_adapter.sse_client") as mock_sse, \
             patch("crosswind.protocols.mcp_adapter.streamable_http_client") as mock_http:

            mock_ctx = MagicMock()
            mock_ctx.__aenter__ = AsyncMock(return_value=(MagicMock(), MagicMock(), MagicMock()))
            mock_ctx.__aexit__ = AsyncMock()
            mock_http.return_value = mock_ctx

            with patch("crosswind.protocols.mcp_adapter.ClientSession") as mock_session_cls:
                mock_session = MagicMock()
                mock_session.__aenter__ = AsyncMock(return_value=mock_session)
                mock_session.__aexit__ = AsyncMock()
                mock_session.initialize = AsyncMock()
                mock_session_cls.return_value = mock_session

                await adapter._ensure_session()

            mock_http.assert_called_once()
            mock_sse.assert_not_called()

        await adapter.cleanup()


# =============================================================================
# Connection Failures
# =============================================================================


class TestConnectionFailures:
    """Test handling of MCP connection failures."""

    @pytest.mark.asyncio
    async def test_connection_refused_propagates(self):
        """Connection refused should propagate as exception."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9999/mcp",  # Non-existent
            tool_name="chat",
            message_field="message",
        )

        with patch("crosswind.protocols.mcp_adapter.streamable_http_client") as mock_http:
            mock_http.side_effect = ConnectionRefusedError("Connection refused")

            with pytest.raises(ConnectionRefusedError):
                await adapter._ensure_session()

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_initialization_failure_propagates(self):
        """MCP initialize handshake failure should propagate."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        with patch("crosswind.protocols.mcp_adapter.streamable_http_client") as mock_http:
            mock_ctx = MagicMock()
            mock_ctx.__aenter__ = AsyncMock(return_value=(MagicMock(), MagicMock(), MagicMock()))
            mock_ctx.__aexit__ = AsyncMock()
            mock_http.return_value = mock_ctx

            with patch("crosswind.protocols.mcp_adapter.ClientSession") as mock_session_cls:
                mock_session = MagicMock()
                mock_session.__aenter__ = AsyncMock(return_value=mock_session)
                mock_session.__aexit__ = AsyncMock()
                mock_session.initialize = AsyncMock(side_effect=Exception("Handshake failed"))
                mock_session_cls.return_value = mock_session

                with pytest.raises(Exception, match="Handshake failed"):
                    await adapter._ensure_session()

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_health_check_returns_false_on_connection_failure(self):
        """Health check should return False when connection fails."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9999/mcp",
            tool_name="chat",
            message_field="message",
        )

        with patch("crosswind.protocols.mcp_adapter.streamable_http_client") as mock_http:
            mock_http.side_effect = ConnectionRefusedError("Connection refused")

            result = await adapter.health_check()

            assert result is False

        await adapter.cleanup()


# =============================================================================
# Resource Cleanup
# =============================================================================


class TestResourceCleanup:
    """Test that resources are properly released."""

    @pytest.mark.asyncio
    async def test_cleanup_closes_session(self):
        """Cleanup should close the MCP session."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        # Setup mocks
        mock_session = MagicMock()
        mock_session.__aexit__ = AsyncMock()
        adapter._session = mock_session
        adapter._initialized = True

        await adapter.cleanup()

        mock_session.__aexit__.assert_called_once()
        assert adapter._session is None
        assert adapter._initialized is False

    @pytest.mark.asyncio
    async def test_cleanup_closes_http_client(self):
        """Cleanup should close the HTTP client for streamable_http transport."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="streamable_http",
        )

        mock_http_client = MagicMock()
        mock_http_client.aclose = AsyncMock()
        adapter._http_client = mock_http_client
        adapter._initialized = True

        await adapter.cleanup()

        mock_http_client.aclose.assert_called_once()
        assert adapter._http_client is None

    @pytest.mark.asyncio
    async def test_cleanup_closes_client_context(self):
        """Cleanup should exit the client context manager."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        mock_ctx = MagicMock()
        mock_ctx.__aexit__ = AsyncMock()
        adapter._client_ctx = mock_ctx
        adapter._initialized = True

        await adapter.cleanup()

        mock_ctx.__aexit__.assert_called_once()
        assert adapter._client_ctx is None

    @pytest.mark.asyncio
    async def test_cleanup_handles_already_closed(self):
        """Cleanup should handle already-closed resources gracefully."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        # Not initialized, nothing to clean up
        await adapter.cleanup()  # Should not raise
        await adapter.cleanup()  # Multiple calls should be safe


# =============================================================================
# Auth Header Building
# =============================================================================


class TestAuthHeaders:
    """Test authentication header building for MCP requests."""

    def test_bearer_auth_header(self):
        """Bearer auth should produce Authorization: Bearer header."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(type="bearer", credentials="my-token"),
        )

        headers = adapter._auth_headers()

        assert headers == {"Authorization": "Bearer my-token"}

    def test_api_key_with_custom_header(self):
        """API key auth should use configured header name."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(
                type="api_key",
                credentials="secret",
                header_name="X-API-Key",
            ),
        )

        headers = adapter._auth_headers()

        assert headers == {"X-API-Key": "secret"}

    def test_empty_credentials_returns_empty(self):
        """Empty credentials should return empty headers."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(type="bearer", credentials=""),
        )

        headers = adapter._auth_headers()

        assert headers == {}


# =============================================================================
# Integration Tests
# =============================================================================


@pytest.mark.integration
class TestMCPIntegration:
    """Integration tests requiring a running MCP server.

    Run with: pytest -m integration
    """

    @pytest.fixture
    def check_server(self):
        """Skip if MCP server not running."""
        import socket

        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        result = sock.connect_ex(("localhost", 9000))
        sock.close()
        if result != 0:
            pytest.skip("MCP test server not running on localhost:9000")

    @pytest.mark.asyncio
    async def test_full_connection_lifecycle(self, check_server):
        """Test connect → call tool → cleanup lifecycle."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="streamable_http",
        )

        try:
            # Health check establishes connection
            healthy = await adapter.health_check()
            assert healthy is True

            # Connection should be established
            assert adapter._initialized is True
            assert adapter._session is not None
        finally:
            await adapter.cleanup()

            # Resources should be released
            assert adapter._session is None
            assert adapter._initialized is False
