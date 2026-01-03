"""A2A adapter tests.

Tests the A2A protocol adapter functionality:
- Authentication header building
- Health check / endpoint reachability
- HTTP and WebSocket message sending

Discovery mode has been removed - the Go API handles agent card discovery
during registration and provides the endpoint directly to the worker.

For Evaluation tests (protocol usage during eval), see test_eval_runner_a2a.py.
"""

import httpx
import pytest
from unittest.mock import MagicMock, patch

from crosswind.models import AuthConfig
from crosswind.protocols.a2a_adapter import A2AAdapter


# =============================================================================
# Constructor Validation
# =============================================================================


class TestConstructorValidation:
    """Test adapter constructor validation."""

    def test_requires_endpoint(self):
        """Should raise ValueError when endpoint is empty."""
        with pytest.raises(ValueError, match="endpoint is required"):
            A2AAdapter(endpoint="")

    def test_accepts_valid_endpoint(self):
        """Should accept valid endpoint URL."""
        adapter = A2AAdapter(endpoint="http://example.com/a2a")
        assert adapter.endpoint == "http://example.com/a2a"

    def test_default_interface_type_is_http(self):
        """Default interface type should be HTTP."""
        adapter = A2AAdapter(endpoint="http://example.com/a2a")
        assert adapter.interface_type == "http"

    def test_accepts_websocket_interface_type(self):
        """Should accept websocket interface type."""
        adapter = A2AAdapter(
            endpoint="ws://example.com/a2a",
            interface_type="websocket",
        )
        assert adapter.interface_type == "websocket"


# =============================================================================
# Authentication (Header Building)
# =============================================================================


class TestAuthentication:
    """Test adapter builds correct auth headers for different auth types.

    Different A2A servers require different authentication methods.
    The adapter must build the correct headers based on AuthConfig.
    """

    def test_bearer_auth(self):
        """Bearer auth should produce 'Authorization: Bearer <token>'."""
        adapter = A2AAdapter(
            endpoint="http://example.com/a2a",
            auth_config=AuthConfig(type="bearer", credentials="my-token"),
        )

        headers = adapter._auth_headers()

        assert headers == {"Authorization": "Bearer my-token"}

    def test_api_key_custom_header(self):
        """API key auth should use specified header name.

        Some APIs use X-API-Key, others use custom headers.
        """
        adapter = A2AAdapter(
            endpoint="http://example.com/a2a",
            auth_config=AuthConfig(
                type="api_key",
                credentials="secret",
                header_name="X-API-Key",
            ),
        )

        headers = adapter._auth_headers()

        assert headers == {"X-API-Key": "secret"}

    def test_basic_auth(self):
        """Basic auth should base64 encode credentials."""
        import base64

        adapter = A2AAdapter(
            endpoint="http://example.com/a2a",
            auth_config=AuthConfig(type="basic", credentials="user:pass"),
        )

        headers = adapter._auth_headers()

        expected = base64.b64encode(b"user:pass").decode()
        assert headers == {"Authorization": f"Basic {expected}"}

    def test_no_auth(self):
        """No auth should produce empty headers."""
        adapter = A2AAdapter(
            endpoint="http://example.com/a2a",
            auth_config=AuthConfig(type="none"),
        )

        headers = adapter._auth_headers()

        assert headers == {}

    def test_unknown_auth_type_empty_headers(self):
        """Unknown auth type should produce empty headers (fail open)."""
        adapter = A2AAdapter(
            endpoint="http://example.com/a2a",
            auth_config=AuthConfig(type="oauth2", credentials="token"),
        )

        headers = adapter._auth_headers()

        assert headers == {}


# =============================================================================
# Health Check
# =============================================================================


class TestHealthCheck:
    """Test health check behavior for endpoint validation."""

    @pytest.mark.asyncio
    async def test_healthy_http_endpoint(self):
        """Should return True when HTTP endpoint is reachable."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a")

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.return_value = MagicMock(status_code=200)

            result = await adapter.health_check()

            assert result is True

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_unreachable_endpoint(self):
        """Should return False when connection fails."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a")

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.side_effect = httpx.ConnectError("Connection refused")

            result = await adapter.health_check()

            assert result is False

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_timeout_returns_unhealthy(self):
        """Should return False on timeout."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a")

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.side_effect = httpx.TimeoutException("Timeout")

            result = await adapter.health_check()

            assert result is False

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_websocket_interface_assumes_healthy(self):
        """WebSocket interface should assume healthy (can't easily check)."""
        adapter = A2AAdapter(
            endpoint="ws://localhost:9000/a2a",
            interface_type="websocket",
        )

        result = await adapter.health_check()

        # WebSocket health check returns True without actual connection
        assert result is True

        await adapter.cleanup()


# =============================================================================
# HTTP Message Sending
# =============================================================================


class TestHTTPMessageSending:
    """Test HTTP message sending functionality."""

    @pytest.mark.asyncio
    async def test_sends_jsonrpc_request(self):
        """Should send JSON-RPC 2.0 formatted request."""
        from crosswind.models import ConversationRequest, Message

        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a")

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: {
                    "jsonrpc": "2.0",
                    "id": "test",
                    "result": {
                        "kind": "message",
                        "parts": [{"kind": "text", "text": "Hello back!"}],
                    },
                },
            )

            response = await adapter.send_message(
                ConversationRequest(
                    messages=[Message(role="user", content="Hello")],
                    session_id="test-session",
                )
            )

            # Verify JSON-RPC structure was sent
            call_args = mock_post.call_args
            sent_json = call_args.kwargs["json"]
            assert sent_json["jsonrpc"] == "2.0"
            assert sent_json["method"] == "message/send"
            assert "params" in sent_json

            # Verify response was extracted
            assert response.content == "Hello back!"
            assert response.session_id == "test-session"

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_http_error_raises_exception(self):
        """Should raise exception on HTTP error status."""
        from crosswind.models import ConversationRequest, Message

        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a")

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(status_code=500)

            with pytest.raises(Exception, match="status 500"):
                await adapter.send_message(
                    ConversationRequest(
                        messages=[Message(role="user", content="Hello")],
                        session_id="test-session",
                    )
                )

        await adapter.cleanup()


# =============================================================================
# WebSocket Smoke Test (Integration)
# =============================================================================


class TestWebSocketSmoke:
    """Single integration test to verify real WebSocket communication.

    This catches issues that unit tests with mocks cannot:
    - WebSocket library API changes
    - Connection/handshake bugs
    - JSON-RPC serialization over WebSocket
    """

    @pytest.mark.usefixtures("a2a_websocket_server")
    @pytest.mark.asyncio
    async def test_websocket_end_to_end(self):
        """Verify real WebSocket send/receive works."""
        from crosswind.models import ConversationRequest, Message

        # The fixture starts a server on port 8907
        adapter = A2AAdapter(
            endpoint="ws://localhost:8907/ws",
            interface_type="websocket",
        )

        try:
            response = await adapter.send_message(
                ConversationRequest(
                    messages=[Message(role="user", content="Hello WebSocket")],
                    session_id="smoke-test",
                )
            )

            assert "Echo: Hello WebSocket" in response.content
            assert response.session_id == "smoke-test"
        finally:
            await adapter.cleanup()
