"""Protocol adapters for different agent types."""

from typing import Any

from crosswind.models import AuthConfig
from crosswind.protocols.a2a_adapter import A2AAdapter
from crosswind.protocols.base import ProtocolAdapter
from crosswind.protocols.mcp_adapter import MCPAdapter
from crosswind.protocols.openapi_http import HTTPAgentError, OpenAPIHttpAdapter
from crosswind.utils.crypto import decrypt_credentials


def create_adapter(agent_doc: dict[str, Any]) -> ProtocolAdapter:
    """Factory function to create the appropriate protocol adapter.

    Supported protocols:
    - Platform protocols (use native SDKs):
        - openai: OpenAI Responses/Conversations API
        - azure_openai: Azure OpenAI (Entra auth)
        - langgraph: LangGraph Platform
        - bedrock: AWS Bedrock Agents
        - vertex: Google Vertex AI Agent Engine

    - Generic protocols (custom HTTP adapters):
        - custom: Generic HTTP API
        - custom_ws: Generic WebSocket API

    Args:
        agent_doc: Agent document from MongoDB

    Returns:
        Protocol adapter instance

    Raises:
        ValueError: If protocol is not supported
        NotImplementedError: For protocols not yet implemented
    """
    endpoint_config = agent_doc.get("endpointConfig", {})
    auth_config = agent_doc.get("authConfig", {})
    protocol = endpoint_config.get("protocol", "custom")

    # Decrypt credentials (they're encrypted in MongoDB)
    encrypted_creds = auth_config.get("credentials", "")
    decrypted_creds = decrypt_credentials(encrypted_creds) if encrypted_creds else ""

    auth = AuthConfig(
        type=auth_config.get("type", "bearer"),
        credentials=decrypted_creds,
        header_name=auth_config.get("headerName", "Authorization"),
        header_prefix=auth_config.get("headerPrefix", "Bearer "),
        aws_region=auth_config.get("awsRegion"),
        azure_tenant_id=auth_config.get("azureTenantId"),
    )

    # Platform protocols - not yet implemented in OSS
    if protocol in ("openai", "azure_openai"):
        raise NotImplementedError(
            f"{protocol} protocol requires OpenAI SDK. "
            "Use 'custom' protocol with the appropriate endpoint."
        )

    elif protocol == "langgraph":
        raise NotImplementedError(
            "LangGraph protocol requires LangGraph SDK. "
            "Use 'custom' protocol with the appropriate endpoint."
        )

    elif protocol == "bedrock":
        raise NotImplementedError(
            "Bedrock protocol requires AWS SDK. "
            "Use 'custom' protocol with the appropriate endpoint."
        )

    elif protocol == "vertex":
        raise NotImplementedError(
            "Vertex protocol requires Google Cloud SDK. "
            "Use 'custom' protocol with the appropriate endpoint."
        )

    # Generic protocols - custom HTTP adapters
    elif protocol in ("custom", "openapi_http"):
        from urllib.parse import urlparse, urlunparse

        endpoint = endpoint_config.get("endpoint")
        if not endpoint:
            raise ValueError("Custom protocol requires endpoint")

        # Derive base_url from full endpoint URL
        parsed = urlparse(endpoint)
        base_url = urlunparse((parsed.scheme, parsed.netloc, "", "", "", ""))
        conversation_endpoint = parsed.path or "/chat"

        return OpenAPIHttpAdapter(
            base_url=base_url,
            conversation_endpoint=conversation_endpoint,
            session_endpoint=endpoint_config.get("sessionEndpoint"),
            auth_config=auth,
            spec_url=endpoint_config.get("specUrl"),
            inferred_schema=agent_doc.get("inferredSchema"),
        )

    elif protocol in ("custom_ws", "openapi_ws"):
        raise NotImplementedError(
            "WebSocket protocol not yet implemented in OSS. "
            "Use 'custom' protocol with HTTP endpoint."
        )

    elif protocol == "a2a":
        # Use stored endpoint (populated during registration by Go API discovery)
        a2a_endpoint = endpoint_config.get("a2aEndpoint")
        if not a2a_endpoint:
            raise ValueError(
                "A2A protocol requires a2aEndpoint in endpointConfig. "
                "This is populated during agent registration from the agent card."
            )

        a2a_interface_type = endpoint_config.get("a2aInterfaceType", "http")

        return A2AAdapter(
            endpoint=a2a_endpoint,
            interface_type=a2a_interface_type,
            auth_config=auth,
        )

    elif protocol == "mcp":
        endpoint = endpoint_config.get("endpoint")
        if not endpoint:
            raise ValueError("MCP protocol requires endpoint in endpointConfig")

        tool_name = endpoint_config.get("mcpToolName")
        if not tool_name:
            raise ValueError("MCP protocol requires mcpToolName in endpointConfig")

        transport = endpoint_config.get("mcpTransport", "streamable_http")
        message_field = endpoint_config.get("mcpMessageField", "message")

        return MCPAdapter(
            endpoint=endpoint,
            tool_name=tool_name,
            message_field=message_field,
            transport=transport,
            auth_config=auth,
        )

    else:
        raise ValueError(f"Unsupported protocol: {protocol}")


__all__ = [
    "HTTPAgentError",
    "ProtocolAdapter",
    "OpenAPIHttpAdapter",
    "A2AAdapter",
    "MCPAdapter",
    "create_adapter",
]
