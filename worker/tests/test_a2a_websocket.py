"""Test A2A adapter WebSocket support."""

import asyncio
import os

import pytest

from crosswind.protocols.a2a_adapter import A2AAdapter, AgentCard
from crosswind.models import ConversationRequest, Message


# Test agent card URL (WebSocket-enabled test agent)
TEST_AGENT_CARD_URL = "http://localhost:8905/.well-known/agent.json"

# Skip integration tests in CI (no server available)
skip_in_ci = pytest.mark.skipif(
    os.environ.get("CI") == "true" or os.environ.get("GITHUB_ACTIONS") == "true",
    reason="Integration tests require a running A2A server"
)


class TestAgentCardInterfaceDetection:
    """Test AgentCard interface detection."""

    def test_websocket_priority(self):
        """WebSocket should be preferred over HTTP."""
        card = AgentCard.from_dict({
            "id": "test",
            "name": "Test",
            "description": "Test",
            "version": "1.0.0",
            "protocolVersion": "0.2.0",
            "provider": {},
            "capabilities": {},
            "skills": [],
            "interfaces": [
                {"type": "http", "url": "http://example.com"},
                {"type": "websocket", "url": "ws://example.com/ws"},
            ],
        })

        iface_type, url = card.get_interface()
        assert iface_type == "websocket"
        assert url == "ws://example.com/ws"

    def test_http_fallback(self):
        """Should fall back to HTTP if no WebSocket."""
        card = AgentCard.from_dict({
            "id": "test",
            "name": "Test",
            "description": "Test",
            "version": "1.0.0",
            "protocolVersion": "0.2.0",
            "provider": {},
            "capabilities": {},
            "skills": [],
            "interfaces": [
                {"type": "http", "url": "http://example.com"},
            ],
        })

        iface_type, url = card.get_interface()
        assert iface_type == "http"
        assert url == "http://example.com"

    def test_json_rpc_treated_as_http(self):
        """json-rpc should be normalized to http."""
        card = AgentCard.from_dict({
            "id": "test",
            "name": "Test",
            "description": "Test",
            "version": "1.0.0",
            "protocolVersion": "0.2.0",
            "provider": {},
            "capabilities": {},
            "skills": [],
            "interfaces": [
                {"type": "json-rpc", "url": "http://example.com/rpc"},
            ],
        })

        iface_type, url = card.get_interface()
        assert iface_type == "http"
        assert url == "http://example.com/rpc"


@skip_in_ci
class TestA2AWebSocketIntegration:
    """Integration tests for A2A WebSocket support.

    Requires the WebSocket test agent to be running on port 8905/8906.
    """

    async def test_detects_websocket_interface(self):
        """Adapter should detect WebSocket from agent card."""
        adapter = A2AAdapter(agent_card_url=TEST_AGENT_CARD_URL)

        try:
            await adapter._ensure_agent_card()

            assert adapter.agent_card is not None
            assert adapter._interface_type == "websocket"
            assert "ws://" in adapter._endpoint
            print(f"Detected interface: {adapter._interface_type} at {adapter._endpoint}")
        finally:
            await adapter.cleanup()

    async def test_send_message_via_websocket(self):
        """Should send and receive messages via WebSocket."""
        adapter = A2AAdapter(agent_card_url=TEST_AGENT_CARD_URL)

        try:
            request = ConversationRequest(
                messages=[Message(role="user", content="Hello!")],
                session_id="test-session-1",
            )

            response = await adapter.send_message(request)

            assert response.content is not None
            assert len(response.content) > 0
            assert response.session_id == "test-session-1"
            print(f"Response: {response.content}")
        finally:
            await adapter.cleanup()

    async def test_multiple_messages_same_session(self):
        """Should reuse WebSocket connection for same session."""
        adapter = A2AAdapter(agent_card_url=TEST_AGENT_CARD_URL)

        try:
            session_id = "test-session-2"

            # First message
            request1 = ConversationRequest(
                messages=[Message(role="user", content="Tell me a joke")],
                session_id=session_id,
            )
            response1 = await adapter.send_message(request1)
            print(f"First response: {response1.content}")

            # Second message - should reuse connection
            request2 = ConversationRequest(
                messages=[Message(role="user", content="What time is it?")],
                session_id=session_id,
            )
            response2 = await adapter.send_message(request2)
            print(f"Second response: {response2.content}")

            # Verify both succeeded
            assert "joke" in response1.content.lower() or any(
                word in response1.content.lower()
                for word in ["why", "what do", "gummy", "noodle"]
            )
            assert "time" in response2.content.lower() or ":" in response2.content

            # Verify connection was created
            assert session_id in adapter._ws_connections
        finally:
            await adapter.cleanup()

    async def test_close_session_closes_websocket(self):
        """Closing session should close WebSocket connection."""
        adapter = A2AAdapter(agent_card_url=TEST_AGENT_CARD_URL)

        try:
            session_id = "test-session-3"

            request = ConversationRequest(
                messages=[Message(role="user", content="Hello")],
                session_id=session_id,
            )
            await adapter.send_message(request)

            # Connection should exist
            assert session_id in adapter._ws_connections

            # Close session
            await adapter.close_session(session_id)

            # Connection should be removed
            assert session_id not in adapter._ws_connections
        finally:
            await adapter.cleanup()

    async def test_cleanup_closes_all_websockets(self):
        """Cleanup should close all WebSocket connections."""
        adapter = A2AAdapter(agent_card_url=TEST_AGENT_CARD_URL)

        try:
            # Create multiple sessions
            for i in range(3):
                request = ConversationRequest(
                    messages=[Message(role="user", content="Hello")],
                    session_id=f"cleanup-test-{i}",
                )
                await adapter.send_message(request)

            # Should have 3 connections
            assert len(adapter._ws_connections) == 3
        finally:
            await adapter.cleanup()

        # All connections should be closed
        assert len(adapter._ws_connections) == 0


if __name__ == "__main__":
    # Run a quick manual test
    async def main():
        print("Testing A2A WebSocket support...")
        print("=" * 50)

        adapter = A2AAdapter(agent_card_url=TEST_AGENT_CARD_URL)

        try:
            # Fetch agent card
            await adapter._ensure_agent_card()
            print(f"Agent: {adapter.agent_card.name}")
            print(f"Interface type: {adapter._interface_type}")
            print(f"Endpoint: {adapter._endpoint}")
            print()

            # Send a message
            request = ConversationRequest(
                messages=[Message(role="user", content="Tell me a joke!")],
                session_id="manual-test",
            )

            response = await adapter.send_message(request)
            print(f"Response: {response.content}")
            print(f"Latency: {response.latency_ms}ms")
            print()

            # Test multiple messages on same connection
            print("Testing second message on same connection...")
            request2 = ConversationRequest(
                messages=[Message(role="user", content="What time is it?")],
                session_id="manual-test",
            )
            response2 = await adapter.send_message(request2)
            print(f"Response 2: {response2.content}")
            print(f"Latency: {response2.latency_ms}ms")
            print()

            # Verify connection was reused
            print(f"Connections created: {len(adapter._ws_connections)}")
            assert len(adapter._ws_connections) == 1, "Should reuse same connection"

            # Test close session
            print("Testing session close...")
            await adapter.close_session("manual-test")
            assert "manual-test" not in adapter._ws_connections
            print("Session closed successfully")

        finally:
            await adapter.cleanup()

        print()
        print("All tests passed!")

    asyncio.run(main())
