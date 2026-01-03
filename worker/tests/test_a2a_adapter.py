"""A2A adapter Registration tests.

Tests the Registration side: Can we create and configure this protocol adapter?
- Interface selection logic (WS vs HTTP)
- Authentication header building
- AgentCard discovery and parsing
- Health check / endpoint reachability
- Endpoint validation

For Evaluation tests (protocol usage during eval), see test_eval_runner_a2a.py.
"""

import httpx
import pytest
from unittest.mock import MagicMock, patch

from crosswind.models import AuthConfig
from crosswind.protocols.a2a_adapter import A2AAdapter, AgentCard


# =============================================================================
# Interface Selection (Decision Logic)
# =============================================================================


class TestInterfaceSelection:
    """Test adapter selects correct interface based on agent card.

    A2A agents can expose multiple interfaces. The adapter prioritizes
    HTTP over WebSocket because:
    - HTTP is simpler (stateless, request-response)
    - Sufficient for evaluation (prompt → response)
    - WebSocket only needed for streaming/real-time (not eval)
    """

    def test_http_priority_over_websocket(self):
        """Given both HTTP and WS, adapter should select HTTP.

        HTTP is preferred for eval because it's simpler and sufficient.
        """
        card = AgentCard.from_dict({
            "name": "Test",
            "description": "Test",
            "interfaces": [
                {"type": "websocket", "url": "ws://example.com/ws"},
                {"type": "http", "url": "http://example.com/http"},
            ],
        })

        iface_type, url = card.get_interface()

        assert iface_type == "http"
        assert url == "http://example.com/http"

    def test_websocket_fallback_when_no_http(self):
        """Given only WebSocket, adapter should use WebSocket."""
        card = AgentCard.from_dict({
            "name": "Test",
            "description": "Test",
            "interfaces": [{"type": "websocket", "url": "ws://example.com/ws"}],
        })

        iface_type, url = card.get_interface()

        assert iface_type == "websocket"
        assert url == "ws://example.com/ws"

    def test_json_rpc_normalized_to_http(self):
        """json-rpc interface type should be treated as HTTP.

        Some A2A servers advertise 'json-rpc' as interface type.
        This should be normalized to HTTP for transport.
        """
        card = AgentCard.from_dict({
            "name": "Test",
            "description": "Test",
            "interfaces": [{"type": "json-rpc", "url": "http://example.com/rpc"}],
        })

        iface_type, url = card.get_interface()

        assert iface_type == "http"

    def test_url_field_fallback(self):
        """When no interfaces array, should use url field.

        Legacy A2A cards may use a top-level 'url' field instead of interfaces.
        """
        card = AgentCard.from_dict({
            "name": "Test",
            "description": "Test",
            "interfaces": [],
            "url": "http://example.com/a2a",
        })

        iface_type, url = card.get_interface()

        assert url == "http://example.com/a2a"

    def test_no_endpoint_returns_empty(self):
        """When no interfaces and no url, should return empty string."""
        card = AgentCard.from_dict({
            "name": "Test",
            "description": "Test",
            "interfaces": [],
        })

        _, url = card.get_interface()

        assert url == ""


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
            agent_card_url="http://example.com/.well-known/agent.json",
            auth_config=AuthConfig(type="bearer", credentials="my-token"),
        )

        headers = adapter._auth_headers()

        assert headers == {"Authorization": "Bearer my-token"}

    def test_api_key_custom_header(self):
        """API key auth should use specified header name.

        Some APIs use X-API-Key, others use custom headers.
        """
        adapter = A2AAdapter(
            agent_card_url="http://example.com/.well-known/agent.json",
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
            agent_card_url="http://example.com/.well-known/agent.json",
            auth_config=AuthConfig(type="basic", credentials="user:pass"),
        )

        headers = adapter._auth_headers()

        expected = base64.b64encode(b"user:pass").decode()
        assert headers == {"Authorization": f"Basic {expected}"}

    def test_no_auth(self):
        """No auth should produce empty headers."""
        adapter = A2AAdapter(
            agent_card_url="http://example.com/.well-known/agent.json",
            auth_config=AuthConfig(type="none"),
        )

        headers = adapter._auth_headers()

        assert headers == {}

    def test_unknown_auth_type_empty_headers(self):
        """Unknown auth type should produce empty headers (fail open)."""
        adapter = A2AAdapter(
            agent_card_url="http://example.com/.well-known/agent.json",
            auth_config=AuthConfig(type="oauth2", credentials="token"),
        )

        headers = adapter._auth_headers()

        assert headers == {}


# =============================================================================
# AgentCard Discovery
# =============================================================================


class TestAgentCardDiscovery:
    """Test fetching and parsing agent cards from /.well-known/agent.json.

    A2A agents expose their capabilities via agent cards. The adapter
    must fetch, parse, and validate these cards during registration.
    """

    @pytest.mark.asyncio
    async def test_fetches_agent_card_on_first_access(self):
        """Should fetch agent card when _ensure_agent_card is called."""
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        mock_card = {
            "name": "Test Agent",
            "description": "A test agent",
            "interfaces": [{"type": "http", "url": "http://localhost:9000/a2a"}],
        }

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_card,
                raise_for_status=lambda: None,
            )

            await adapter._ensure_agent_card()

            mock_get.assert_called_once()
            assert adapter.agent_card is not None
            assert adapter.agent_card.name == "Test Agent"

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_connection_error_during_discovery(self):
        """Should raise when agent card fetch fails."""
        adapter = A2AAdapter(
            agent_card_url="http://nonexistent.local/.well-known/agent.json",
            timeout=1.0,
        )

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.side_effect = httpx.ConnectError("Connection refused")

            with pytest.raises(httpx.ConnectError):
                await adapter._ensure_agent_card()

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_missing_endpoint_raises_validation_error(self):
        """Should raise ValueError when agent card has no endpoint.

        An agent card without interfaces or url is invalid for communication.
        """
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        bad_card = {"name": "Bad", "description": "No endpoint", "interfaces": []}

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.return_value = MagicMock(
                status_code=200,
                json=lambda: bad_card,
                raise_for_status=lambda: None,
            )

            with pytest.raises(ValueError, match="missing"):
                await adapter._ensure_agent_card()

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_detects_interface_type_from_card(self):
        """Should set _interface_type based on agent card."""
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        mock_card = {
            "name": "WS Agent",
            "description": "Test",
            "interfaces": [
                {"type": "websocket", "url": "ws://localhost:9000/ws"},
            ],
        }

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_card,
                raise_for_status=lambda: None,
            )

            await adapter._ensure_agent_card()

            assert adapter._interface_type == "websocket"
            assert "ws://" in adapter._endpoint

        await adapter.cleanup()


# =============================================================================
# Health Check
# =============================================================================


class TestHealthCheck:
    """Test health check behavior for registration validation.

    Before registering an agent, we verify the endpoint is reachable.
    """

    @pytest.mark.asyncio
    async def test_healthy_endpoint(self):
        """Should return True when agent card is reachable."""
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.return_value = MagicMock(status_code=200)

            result = await adapter.health_check()

            assert result is True

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_unreachable_endpoint(self):
        """Should return False when connection fails."""
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.side_effect = httpx.ConnectError("Connection refused")

            result = await adapter.health_check()

            assert result is False

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_timeout_returns_unhealthy(self):
        """Should return False on timeout."""
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.side_effect = httpx.TimeoutException("Timeout")

            result = await adapter.health_check()

            assert result is False

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_404_returns_unhealthy(self):
        """Should return False when agent card returns 404."""
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.return_value = MagicMock(status_code=404)

            result = await adapter.health_check()

            assert result is False

        await adapter.cleanup()


# =============================================================================
# Endpoint Property Validation
# =============================================================================


class TestEndpointValidation:
    """Test endpoint property access validation."""

    @pytest.mark.asyncio
    async def test_endpoint_access_before_card_loaded(self):
        """Should raise when accessing endpoint before agent card loaded.

        This prevents using the adapter before discovery is complete.
        """
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        with pytest.raises(RuntimeError, match="not loaded"):
            _ = adapter.endpoint

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_endpoint_accessible_after_discovery(self):
        """Should return endpoint after agent card is loaded."""
        adapter = A2AAdapter(agent_card_url="http://localhost:9000/.well-known/agent.json")

        mock_card = {
            "name": "Test",
            "description": "Test",
            "interfaces": [{"type": "http", "url": "http://localhost:9000/a2a"}],
        }

        with patch.object(adapter.client, "get") as mock_get:
            mock_get.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_card,
                raise_for_status=lambda: None,
            )

            await adapter._ensure_agent_card()

            assert adapter.endpoint == "http://localhost:9000/a2a"

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

        adapter = A2AAdapter(agent_card_url="http://localhost:8907/.well-known/agent.json")

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
