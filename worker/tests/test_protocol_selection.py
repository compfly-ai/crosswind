"""Tests for protocol adapter selection.

Tests the `create_adapter()` factory function that selects the appropriate
protocol adapter based on agent configuration.
"""

import pytest
from unittest.mock import patch, MagicMock

from crosswind.protocols import create_adapter, A2AAdapter, OpenAPIHttpAdapter


class TestProtocolSelection:
    """Test protocol adapter factory selection logic."""

    def test_custom_protocol_returns_http_adapter(self):
        """Custom protocol should return OpenAPIHttpAdapter."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom",
                "endpoint": "http://localhost:8000/chat",
            },
            "authConfig": {
                "type": "bearer",
                "credentials": "",  # Empty = no encryption needed
            },
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)

    def test_openapi_http_protocol_returns_http_adapter(self):
        """openapi_http protocol should return OpenAPIHttpAdapter."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "openapi_http",
                "endpoint": "http://localhost:8000/api/chat",
            },
            "authConfig": {
                "type": "none",
            },
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)

    def test_a2a_protocol_returns_a2a_adapter(self):
        """A2A protocol should return A2AAdapter."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "a2a",
                "agentCardUrl": "http://localhost:8903/.well-known/agent.json",
            },
            "authConfig": {
                "type": "api_key",
                "credentials": "",
                "headerName": "X-API-Key",
            },
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, A2AAdapter)
        assert adapter.agent_card_url == "http://localhost:8903/.well-known/agent.json"

    def test_a2a_protocol_missing_agent_card_url_raises(self):
        """A2A protocol without agentCardUrl should raise ValueError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "a2a",
                # Missing agentCardUrl!
            },
            "authConfig": {},
        }

        with pytest.raises(ValueError, match="agentCardUrl"):
            create_adapter(agent_doc)

    def test_custom_protocol_missing_endpoint_raises(self):
        """Custom protocol without endpoint should raise ValueError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom",
                # Missing endpoint!
            },
            "authConfig": {},
        }

        with pytest.raises(ValueError, match="endpoint"):
            create_adapter(agent_doc)

    def test_mcp_protocol_requires_tool_name(self):
        """MCP protocol should raise ValueError without mcpToolName."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "mcp",
                "endpoint": "http://localhost:8902/mcp",
                # Missing mcpToolName!
            },
            "authConfig": {},
        }

        with pytest.raises(ValueError, match="mcpToolName"):
            create_adapter(agent_doc)

    def test_openai_protocol_not_implemented(self):
        """OpenAI protocol should raise NotImplementedError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "openai",
                "endpoint": "https://api.openai.com/v1",
            },
            "authConfig": {},
        }

        with pytest.raises(NotImplementedError, match="openai"):
            create_adapter(agent_doc)

    def test_langgraph_protocol_not_implemented(self):
        """LangGraph protocol should raise NotImplementedError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "langgraph",
                "endpoint": "http://localhost:8000",
            },
            "authConfig": {},
        }

        with pytest.raises(NotImplementedError, match="LangGraph"):
            create_adapter(agent_doc)

    def test_bedrock_protocol_not_implemented(self):
        """Bedrock protocol should raise NotImplementedError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "bedrock",
            },
            "authConfig": {},
        }

        with pytest.raises(NotImplementedError, match="Bedrock"):
            create_adapter(agent_doc)

    def test_vertex_protocol_not_implemented(self):
        """Vertex protocol should raise NotImplementedError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "vertex",
            },
            "authConfig": {},
        }

        with pytest.raises(NotImplementedError, match="Vertex"):
            create_adapter(agent_doc)

    def test_websocket_protocol_not_implemented(self):
        """WebSocket protocol should raise NotImplementedError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom_ws",
                "endpoint": "ws://localhost:8000/ws",
            },
            "authConfig": {},
        }

        with pytest.raises(NotImplementedError, match="WebSocket"):
            create_adapter(agent_doc)

    def test_unsupported_protocol_raises(self):
        """Unsupported protocol should raise ValueError."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "unknown_protocol",
            },
            "authConfig": {},
        }

        with pytest.raises(ValueError, match="Unsupported protocol"):
            create_adapter(agent_doc)

    def test_default_protocol_is_custom(self):
        """Missing protocol should default to custom."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                # No protocol specified
                "endpoint": "http://localhost:8000/chat",
            },
            "authConfig": {},
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)


class TestProtocolSelectionAuthConfig:
    """Test auth config handling in protocol selection."""

    def test_bearer_auth_passed_to_adapter(self):
        """Bearer auth config should be passed to adapter."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "a2a",
                "agentCardUrl": "http://localhost:8903/.well-known/agent.json",
            },
            "authConfig": {
                "type": "bearer",
                "credentials": "",  # Would be encrypted in real usage
            },
        }

        adapter = create_adapter(agent_doc)

        assert adapter.auth_config.type == "bearer"

    def test_api_key_auth_with_custom_header(self):
        """API key auth with custom header should be passed to adapter."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "a2a",
                "agentCardUrl": "http://localhost:8903/.well-known/agent.json",
            },
            "authConfig": {
                "type": "api_key",
                "credentials": "",
                "headerName": "X-Custom-API-Key",
            },
        }

        adapter = create_adapter(agent_doc)

        assert adapter.auth_config.type == "api_key"
        assert adapter.auth_config.header_name == "X-Custom-API-Key"

    def test_no_auth_config_uses_defaults(self):
        """Missing auth config should use defaults."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "a2a",
                "agentCardUrl": "http://localhost:8903/.well-known/agent.json",
            },
            # No authConfig!
        }

        adapter = create_adapter(agent_doc)

        assert adapter.auth_config.type == "bearer"  # default
        assert adapter.auth_config.credentials == ""


class TestProtocolSelectionEndpointParsing:
    """Test endpoint URL parsing for HTTP adapters."""

    def test_endpoint_path_extracted_correctly(self):
        """Endpoint path should be extracted from full URL."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom",
                "endpoint": "http://localhost:8000/api/v1/chat",
            },
            "authConfig": {},
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)
        # The adapter should have base_url and conversation_endpoint separated

    def test_endpoint_with_port(self):
        """Endpoint with port should be parsed correctly."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom",
                "endpoint": "http://localhost:9999/chat",
            },
            "authConfig": {},
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)

    def test_https_endpoint(self):
        """HTTPS endpoint should be handled correctly."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom",
                "endpoint": "https://api.example.com/v1/chat",
            },
            "authConfig": {},
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)

    def test_session_endpoint_passed_to_adapter(self):
        """Session endpoint should be passed to HTTP adapter."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom",
                "endpoint": "http://localhost:8000/chat",
                "sessionEndpoint": "/session",
            },
            "authConfig": {},
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)

    def test_inferred_schema_passed_to_adapter(self):
        """Inferred schema should be passed to HTTP adapter."""
        agent_doc = {
            "agentId": "test-agent",
            "endpointConfig": {
                "protocol": "custom",
                "endpoint": "http://localhost:8000/chat",
            },
            "authConfig": {},
            "inferredSchema": {
                "messageField": "prompt",
                "responseContentField": "response",
            },
        }

        adapter = create_adapter(agent_doc)

        assert isinstance(adapter, OpenAPIHttpAdapter)
