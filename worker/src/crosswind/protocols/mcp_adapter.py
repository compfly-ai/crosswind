"""MCP (Model Context Protocol) adapter for tool execution during evaluations.

Implements the Anthropic MCP protocol for calling tools on MCP servers.
See: https://modelcontextprotocol.io/
"""

import time
from collections.abc import AsyncIterator
from typing import Any
from uuid import uuid4

import httpx
import structlog
from mcp import ClientSession
from mcp.client.sse import sse_client
from mcp.client.streamable_http import streamable_http_client

from crosswind.models import AuthConfig, ConversationRequest, ConversationResponse
from crosswind.protocols.base import ProtocolAdapter

logger = structlog.get_logger()


class MCPAdapter(ProtocolAdapter):
    """Adapter for MCP (Model Context Protocol).

    Flow:
    1. Initialize connection via JSON-RPC
    2. Call the configured tool with the prompt mapped to message_field
    3. Return the tool result as the response

    Supports both SSE and streamable_http transports.
    """

    def __init__(
        self,
        endpoint: str,
        tool_name: str,
        message_field: str,
        transport: str = "streamable_http",
        auth_config: AuthConfig | None = None,
        timeout: float = 120.0,
    ) -> None:
        """Initialize the MCP adapter.

        Args:
            endpoint: MCP server endpoint URL
            tool_name: Name of the tool to call
            message_field: Primary text input field in the tool's inputSchema
            transport: Transport type ("sse" or "streamable_http")
            auth_config: Authentication configuration
            timeout: Request timeout in seconds
        """
        self.endpoint = endpoint
        self.tool_name = tool_name
        self.message_field = message_field
        self.transport = transport
        self.auth_config = auth_config or AuthConfig()
        self.timeout = timeout
        self._initialized = False
        self._session: Any = None
        self._client_ctx: Any = None
        self._http_client: httpx.AsyncClient | None = None

    async def _ensure_session(self) -> None:
        """Ensure we have an active MCP session."""
        if self._initialized:
            return

        logger.debug(
            "Initializing MCP session",
            endpoint=self.endpoint,
            transport=self.transport,
            tool=self.tool_name,
        )

        if self.transport == "sse":
            # sse_client takes headers directly
            self._client_ctx = sse_client(
                url=self.endpoint,
                headers=self._auth_headers() or None,
                timeout=self.timeout,
            )
        else:
            # streamable_http_client takes http_client
            self._http_client = httpx.AsyncClient(
                timeout=self.timeout,
                headers=self._auth_headers(),
            )
            self._client_ctx = streamable_http_client(
                url=self.endpoint,
                http_client=self._http_client,
            )

        # MCP SDK returns (read_stream, write_stream) for SSE
        # and (read_stream, write_stream, get_session_id) for streamable_http
        streams = await self._client_ctx.__aenter__()
        if len(streams) == 2:
            read, write = streams
        else:
            read, write, _get_session_id = streams
        self._session = ClientSession(read, write)
        await self._session.__aenter__()
        await self._session.initialize()
        self._initialized = True

        logger.info(
            "MCP session initialized",
            endpoint=self.endpoint,
            tool=self.tool_name,
        )

    async def cleanup(self) -> None:
        """Clean up resources."""
        if self._session:
            try:
                await self._session.__aexit__(None, None, None)
            except Exception as e:
                logger.warning("Error closing MCP session", error=str(e))
            self._session = None

        if self._client_ctx:
            try:
                await self._client_ctx.__aexit__(None, None, None)
            except Exception as e:
                logger.warning("Error closing MCP client context", error=str(e))
            self._client_ctx = None

        if self._http_client:
            try:
                await self._http_client.aclose()
            except Exception as e:
                logger.warning("Error closing HTTP client", error=str(e))
            self._http_client = None

        self._initialized = False

    def _auth_headers(self) -> dict[str, str]:
        """Build authentication headers."""
        if not self.auth_config.credentials:
            return {}

        if self.auth_config.type == "bearer":
            return {"Authorization": f"Bearer {self.auth_config.credentials}"}
        elif self.auth_config.type == "api_key":
            header_name = self.auth_config.header_name or "X-API-Key"
            return {header_name: self.auth_config.credentials}
        elif self.auth_config.type == "basic":
            import base64

            encoded = base64.b64encode(self.auth_config.credentials.encode()).decode()
            return {"Authorization": f"Basic {encoded}"}
        else:
            return {}

    async def create_session(self) -> str:
        """Create a new conversation session.

        MCP doesn't have built-in session management, so we generate a client-side UUID.
        """
        return str(uuid4())

    async def send_message(self, request: ConversationRequest) -> ConversationResponse:
        """Send a message by calling the configured MCP tool.

        Args:
            request: The conversation request containing the prompt

        Returns:
            The tool result as a ConversationResponse
        """
        await self._ensure_session()
        start_time = time.monotonic()

        # Extract the prompt text from the request
        latest_message = request.messages[-1].content if request.messages else ""

        # Build tool arguments using the message field
        arguments = {self.message_field: latest_message}

        logger.debug(
            "Calling MCP tool",
            tool=self.tool_name,
            message_field=self.message_field,
            endpoint=self.endpoint,
        )

        # Call the tool
        result = await self._session.call_tool(
            name=self.tool_name,
            arguments=arguments,
        )

        latency_ms = int((time.monotonic() - start_time) * 1000)

        # Extract text content from result
        content = self._extract_content(result)

        logger.debug(
            "MCP tool call complete",
            tool=self.tool_name,
            latency_ms=latency_ms,
            content_length=len(content),
        )

        return ConversationResponse(
            session_id=request.session_id or str(uuid4()),
            content=content,
            latency_ms=latency_ms,
            raw_response={"tool": self.tool_name, "result": str(result)},
        )

    def _extract_content(self, result: Any) -> str:
        """Extract text content from MCP tool result."""
        if hasattr(result, "content") and result.content:
            texts = []
            for item in result.content:
                if hasattr(item, "text"):
                    texts.append(item.text)
            return "\n".join(texts) if texts else str(result)
        return str(result)

    async def send_message_streaming(
        self,
        request: ConversationRequest,
    ) -> AsyncIterator[str]:
        """Send a message and stream response tokens.

        MCP tool calls are not typically streaming, so this falls back to
        non-streaming and yields the complete result.

        Yields:
            The complete response as a single chunk
        """
        response = await self.send_message(request)
        yield response.content

    async def close_session(self, session_id: str) -> None:
        """Close a conversation session.

        MCP doesn't have explicit session management, so this is a no-op.
        """
        logger.debug("MCP session close requested (no-op)", session_id=session_id)

    async def health_check(self) -> bool:
        """Check if the MCP server is reachable.

        Attempts to initialize the session which includes the initialize handshake.
        """
        try:
            await self._ensure_session()
            return True
        except Exception as e:
            logger.warning("MCP health check failed", error=str(e))
            return False
