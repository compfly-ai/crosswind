"""MCP Evaluation tests.

Tests MCP behavior during evaluations:
- Prompt mapping to tool arguments via message_field
- Response content extraction from MCP results
- Multi-turn conversation handling
- Error handling

For adapter tests (auth headers), see test_mcp_adapter.py.
For registration tests (agent_doc → adapter), see test_protocol_selection.py.
"""

import pytest
from unittest.mock import AsyncMock, MagicMock

from crosswind.models import ConversationRequest, Message
from crosswind.protocols.mcp_adapter import MCPAdapter


# =============================================================================
# Content Extraction
# =============================================================================


class TestContentExtraction:
    """Test _extract_content() extracts text from MCP tool results."""

    @pytest.fixture
    def adapter(self):
        return MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )

    def test_joins_multiple_text_items_with_newlines(self, adapter):
        """Multiple content items should be joined with newlines."""
        item1 = MagicMock()
        item1.text = "First paragraph"
        item2 = MagicMock()
        item2.text = "Second paragraph"
        result = MagicMock()
        result.content = [item1, item2]

        content = adapter._extract_content(result)

        assert content == "First paragraph\nSecond paragraph"

    def test_falls_back_to_str_when_no_text_attribute(self, adapter):
        """Should stringify result when content items lack text attribute."""
        result = MagicMock()
        result.content = []

        content = adapter._extract_content(result)

        # Falls back to str(result)
        assert content is not None
        assert len(content) > 0


# =============================================================================
# Prompt to Tool Argument Mapping
# =============================================================================


class TestPromptMapping:
    """Test that prompts are correctly mapped to tool arguments."""

    @pytest.fixture
    def mock_result(self):
        content = MagicMock()
        content.text = "Response"
        result = MagicMock()
        result.content = [content]
        return result

    @pytest.mark.asyncio
    async def test_uses_configured_message_field(self, mock_result):
        """Prompt should be passed using the configured message_field name."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="search",
            message_field="query",  # Custom field
        )
        mock_session = MagicMock()
        mock_session.call_tool = AsyncMock(return_value=mock_result)
        adapter._session = mock_session
        adapter._initialized = True

        request = ConversationRequest(
            messages=[Message(role="user", content="find order 12345")],
            session_id="test",
        )

        await adapter.send_message(request)

        call_args = mock_session.call_tool.call_args
        assert call_args.kwargs["name"] == "search"
        assert call_args.kwargs["arguments"] == {"query": "find order 12345"}

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_uses_last_message_for_multiturn(self, mock_result):
        """Multi-turn conversations should use only the last message."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )
        mock_session = MagicMock()
        mock_session.call_tool = AsyncMock(return_value=mock_result)
        adapter._session = mock_session
        adapter._initialized = True

        request = ConversationRequest(
            messages=[
                Message(role="user", content="First message"),
                Message(role="assistant", content="First response"),
                Message(role="user", content="Follow-up question"),
            ],
            session_id="multi-turn",
        )

        await adapter.send_message(request)

        call_args = mock_session.call_tool.call_args
        assert call_args.kwargs["arguments"]["message"] == "Follow-up question"

        await adapter.cleanup()


# =============================================================================
# Error Handling
# =============================================================================


class TestErrorHandling:
    """Test error handling during tool calls."""

    @pytest.mark.asyncio
    async def test_tool_call_exception_propagates(self):
        """Exceptions from tool calls should propagate to caller."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )
        mock_session = MagicMock()
        mock_session.call_tool = AsyncMock(
            side_effect=Exception("MCP server unavailable")
        )
        adapter._session = mock_session
        adapter._initialized = True

        request = ConversationRequest(
            messages=[Message(role="user", content="Hello")],
            session_id="test",
        )

        with pytest.raises(Exception, match="MCP server unavailable"):
            await adapter.send_message(request)

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_empty_response_returns_fallback_content(self):
        """Empty tool response should return non-empty fallback content."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
        )
        mock_result = MagicMock()
        mock_result.content = []
        mock_session = MagicMock()
        mock_session.call_tool = AsyncMock(return_value=mock_result)
        adapter._session = mock_session
        adapter._initialized = True

        request = ConversationRequest(
            messages=[Message(role="user", content="Hello")],
            session_id="test",
        )

        response = await adapter.send_message(request)

        # Should have fallback content (str(result)), not empty
        assert response.content is not None

        await adapter.cleanup()


# =============================================================================
# Integration Tests (require running MCP server)
# =============================================================================


@pytest.mark.integration
class TestMCPIntegration:
    """Integration tests requiring a running MCP server.

    Run with: pytest -m integration
    Requires MCP test server running on localhost:9000
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
    async def test_full_handshake_and_tool_call(self, check_server):
        """Test complete MCP flow: handshake → initialize → tool call."""
        adapter = MCPAdapter(
            endpoint="http://localhost:9000/mcp",
            tool_name="chat",
            message_field="message",
            transport="streamable_http",
        )

        try:
            request = ConversationRequest(
                messages=[Message(role="user", content="Hello!")],
                session_id="integration-test",
            )

            response = await adapter.send_message(request)

            assert response.content is not None
            assert len(response.content) > 0
            assert response.latency_ms > 0
        finally:
            await adapter.cleanup()
