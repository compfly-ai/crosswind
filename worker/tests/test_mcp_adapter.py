"""Unit tests for MCP protocol adapter."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from crosswind.protocols.mcp_adapter import MCPAdapter
from crosswind.models import AuthConfig, ConversationRequest, Message


class TestMCPAdapterUnit:
    """Unit tests for MCPAdapter without requiring a real MCP server."""

    def test_init_defaults(self):
        """Test adapter initialization with default values."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        assert adapter.endpoint == "http://localhost:9000/mcp"
        assert adapter.tool_name == "chat"
        assert adapter.message_field == "message"
        assert adapter.transport == "streamable_http"
        assert adapter.timeout == 120.0
        assert adapter._initialized is False
        assert adapter._session is None

    def test_init_with_sse_transport(self):
        """Test adapter initialization with SSE transport."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="search",
            message_field="query",
            transport="sse",
        )

        assert adapter.transport == "sse"

    def test_init_with_auth(self):
        """Test adapter initialization with authentication."""
        auth = AuthConfig(type="bearer", credentials="test-token")
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=auth,
        )

        assert adapter.auth_config.type == "bearer"
        assert adapter.auth_config.credentials == "test-token"

    def test_auth_headers_bearer(self):
        """Test bearer token auth header generation."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(type="bearer", credentials="my-token"),
        )

        headers = adapter._auth_headers()
        assert headers == {"Authorization": "Bearer my-token"}

    def test_auth_headers_api_key(self):
        """Test API key auth header generation.

        Note: AuthConfig defaults header_name to "Authorization".
        For X-API-Key header, use header_name="X-API-Key" explicitly.
        """
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(type="api_key", credentials="my-key", header_name="X-API-Key"),
        )

        headers = adapter._auth_headers()
        assert headers == {"X-API-Key": "my-key"}

    def test_auth_headers_api_key_custom_header(self):
        """Test API key with custom header name."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(
                type="api_key",
                credentials="my-key",
                header_name="Custom-Auth"
            ),
        )

        headers = adapter._auth_headers()
        assert headers == {"Custom-Auth": "my-key"}

    def test_auth_headers_basic(self):
        """Test basic auth header generation."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(type="basic", credentials="user:pass"),
        )

        headers = adapter._auth_headers()
        assert "Authorization" in headers
        assert headers["Authorization"].startswith("Basic ")

    def test_auth_headers_none(self):
        """Test no auth returns empty headers."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            auth_config=AuthConfig(type="none"),
        )

        headers = adapter._auth_headers()
        assert headers == {}

    def test_extract_content_with_text(self):
        """Test content extraction from MCP result with text."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        # Mock result with content items that have text
        mock_item1 = MagicMock()
        mock_item1.text = "Hello"
        mock_item2 = MagicMock()
        mock_item2.text = "World"

        mock_result = MagicMock()
        mock_result.content = [mock_item1, mock_item2]

        content = adapter._extract_content(mock_result)
        assert content == "Hello\nWorld"

    def test_extract_content_no_text(self):
        """Test content extraction falls back to str()."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        # Mock result without text attribute
        mock_item = MagicMock(spec=[])  # No text attribute

        mock_result = MagicMock()
        mock_result.content = [mock_item]

        content = adapter._extract_content(mock_result)
        # Should fall back to str(result)
        assert content is not None

    def test_extract_content_empty(self):
        """Test content extraction with empty content."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        mock_result = MagicMock()
        mock_result.content = []

        content = adapter._extract_content(mock_result)
        # Should fall back to str(result) when no texts
        assert content is not None

    async def test_create_session_returns_uuid(self):
        """Test session creation returns a valid UUID."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        session_id = await adapter.create_session()
        assert session_id is not None
        assert len(session_id) == 36  # UUID format
        assert "-" in session_id

    async def test_close_session_is_noop(self):
        """Test session close is a no-op for MCP."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        # Should not raise
        await adapter.close_session("test-session-id")


class TestMCPAdapterMessageMapping:
    """Test message field mapping logic."""

    def test_message_field_used_in_arguments(self):
        """Verify message field is used when building tool arguments."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="search",
            message_field="query",  # Custom message field
        )

        # The adapter should use "query" as the key
        assert adapter.message_field == "query"

    def test_different_message_fields(self):
        """Test adapter with various message field names."""
        fields = ["message", "text", "prompt", "query", "input", "content"]

        for field in fields:
            adapter = MCPAdapter(
                endpoint="http://localhost:9000/mcp",
                tool_name="chat",
                message_field=field,
            )
            assert adapter.message_field == field


class TestMCPAdapterCleanup:
    """Test cleanup behavior."""

    async def test_cleanup_when_not_initialized(self):
        """Test cleanup when never initialized."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        # Should not raise even when not initialized
        await adapter.cleanup()
        assert adapter._initialized is False
        assert adapter._session is None

    async def test_cleanup_clears_state(self):
        """Test cleanup clears internal state."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

        # Set some mock state
        adapter._initialized = True
        adapter._session = MagicMock()
        adapter._session.__aexit__ = AsyncMock()
        adapter._client_ctx = MagicMock()
        adapter._client_ctx.__aexit__ = AsyncMock()
        adapter._http_client = MagicMock()
        adapter._http_client.aclose = AsyncMock()

        await adapter.cleanup()

        assert adapter._initialized is False
        assert adapter._session is None
        assert adapter._client_ctx is None
        assert adapter._http_client is None


@pytest.mark.usefixtures("mcp_test_server")
class TestMCPAdapterIntegration:
    """Integration tests requiring a running MCP server.

    Uses the mcp_test_server fixture defined in conftest.py.
    """

    async def test_send_message(self):
        """Test sending a message to MCP server."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="streamable_http",
        )

        try:
            request = ConversationRequest(
                messages=[Message(role="user", content="Hello!")],
                session_id="test-session",
            )

            response = await adapter.send_message(request)

            assert response.content is not None
            assert len(response.content) > 0
            assert response.session_id == "test-session"
            assert response.latency_ms >= 0
        finally:
            await adapter.cleanup()

    async def test_health_check(self):
        """Test health check connects to server."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="streamable_http",
        )

        try:
            is_healthy = await adapter.health_check()
            assert is_healthy is True
        finally:
            await adapter.cleanup()

    async def test_streaming_fallback(self):
        """Test streaming falls back to non-streaming."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="streamable_http",
        )

        try:
            request = ConversationRequest(
                messages=[Message(role="user", content="Test streaming")],
                session_id="stream-test",
            )

            chunks = []
            async for chunk in adapter.send_message_streaming(request):
                chunks.append(chunk)

            # Should return single chunk (non-streaming fallback)
            assert len(chunks) == 1
            assert len(chunks[0]) > 0
        finally:
            await adapter.cleanup()


# Fixture for MCP test server
@pytest.fixture(scope="session")
def mcp_test_server():
    """Start MCP test server for integration tests.

    Uses the test server from test_mcp_discovery.py.
    Server must be started manually with:
        uv run python tests/test_mcp_discovery.py --server
    """
    import socket

    # Check if server is already running
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    result = sock.connect_ex(("localhost", 9000))
    sock.close()

    if result != 0:
        pytest.skip("MCP test server not running. Start with: uv run python tests/test_mcp_discovery.py --server")

    yield

    # No cleanup needed - server is managed externally
