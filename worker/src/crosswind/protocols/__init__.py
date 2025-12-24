"""Protocol adapters for different agent types."""

from typing import Any

from crosswind.models import AuthConfig
from crosswind.protocols.a2a_adapter import A2AAdapter
from crosswind.protocols.base import ProtocolAdapter
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
        agent_card_url = endpoint_config.get("agentCardUrl")
        if not agent_card_url:
            raise ValueError("A2A protocol requires agentCardUrl in endpointConfig")

        return A2AAdapter(
            agent_card_url=agent_card_url,
            auth_config=auth,
        )

    elif protocol == "mcp":
        raise NotImplementedError("MCP protocol support coming in V2")

    else:
        raise ValueError(f"Unsupported protocol: {protocol}")


__all__ = [
    "HTTPAgentError",
    "ProtocolAdapter",
    "OpenAPIHttpAdapter",
    "A2AAdapter",
    "create_adapter",
]
