"""A2A (Agent-to-Agent) protocol adapter.

Implements the Google A2A protocol for agent-to-agent communication.
See: https://github.com/google/A2A
"""

import json
import time
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any
from uuid import uuid4

import httpx
import structlog
import websockets
from websockets.client import WebSocketClientProtocol

from crosswind.models import AuthConfig, ConversationRequest, ConversationResponse
from crosswind.protocols.base import ProtocolAdapter

logger = structlog.get_logger()


@dataclass
class AgentCard:
    """A2A Agent Card metadata."""

    id: str
    name: str
    description: str
    version: str
    protocol_version: str
    provider: dict[str, Any]
    capabilities: dict[str, Any]
    skills: list[dict[str, Any]]
    interfaces: list[dict[str, Any]]
    url: str | None = None  # Direct URL field (newer A2A spec)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "AgentCard":
        """Create AgentCard from dictionary."""
        return cls(
            id=data.get("id", ""),
            name=data.get("name", ""),
            description=data.get("description", ""),
            version=data.get("version", ""),
            protocol_version=data.get("protocolVersion", ""),
            provider=data.get("provider", {}),
            capabilities=data.get("capabilities", {}),
            skills=data.get("skills", []),
            interfaces=data.get("interfaces", []),
            url=data.get("url"),
        )

    def get_url(self) -> str:
        """Extract service URL from agent card (legacy method).

        Checks both the direct 'url' field and 'interfaces' array.
        """
        _, url = self.get_interface()
        return url

    def get_interface(self) -> tuple[str, str]:
        """Extract preferred interface (type, url) from agent card.

        Priority: websocket > http > json-rpc > first available > direct url

        Returns:
            Tuple of (interface_type, url). interface_type is one of:
            "websocket", "http", or the raw type from the agent card.
        """
        # Check for WebSocket first (preferred for bidirectional communication)
        for interface in self.interfaces:
            if interface.get("type") == "websocket":
                return ("websocket", interface.get("url", ""))

        # Fall back to HTTP/JSON-RPC
        for interface in self.interfaces:
            if interface.get("type") in ("http", "json-rpc"):
                return ("http", interface.get("url", ""))

        # Use first interface if available
        if self.interfaces:
            iface = self.interfaces[0]
            iface_type = iface.get("type", "http")
            # Normalize json-rpc to http
            if iface_type == "json-rpc":
                iface_type = "http"
            return (iface_type, iface.get("url", ""))

        # Direct URL field (newer A2A spec) - assume HTTP
        if self.url:
            return ("http", self.url)

        return ("http", "")


class A2AAdapter(ProtocolAdapter):
    """Adapter for A2A (Agent-to-Agent) protocol.

    Flow:
    1. Fetch agent card from agent_card_url
    2. Extract endpoint URL and interface type from agent card
    3. Send JSON-RPC 2.0 messages via HTTP or WebSocket

    Supports both HTTP and WebSocket interfaces as declared in the agent card.
    """

    def __init__(
        self,
        agent_card_url: str,
        auth_config: AuthConfig | None = None,
        timeout: float = 120.0,
    ) -> None:
        """Initialize the A2A adapter.

        Args:
            agent_card_url: URL to the agent card
                (e.g., https://agent.example.com/.well-known/agent.json)
            auth_config: Authentication configuration
            timeout: Request timeout in seconds
        """
        self.agent_card_url = agent_card_url
        self.auth_config = auth_config or AuthConfig()
        self.timeout = timeout
        self.client = httpx.AsyncClient(timeout=timeout)
        self.agent_card: AgentCard | None = None
        self._endpoint: str | None = None
        self._interface_type: str = "http"  # "http" or "websocket"
        self._ws_connections: dict[str, WebSocketClientProtocol] = {}

    async def _get_ws_connection(self, session_id: str) -> WebSocketClientProtocol:
        """Get or create WebSocket connection for a session.

        Each session maintains its own WebSocket connection for isolation.
        """
        if session_id not in self._ws_connections:
            logger.debug(
                "Creating WebSocket connection",
                endpoint=self._endpoint,
                session_id=session_id,
            )
            ws = await websockets.connect(
                self._endpoint,
                additional_headers=self._auth_headers(),
            )
            self._ws_connections[session_id] = ws
        return self._ws_connections[session_id]

    async def _close_ws_connection(self, session_id: str) -> None:
        """Close WebSocket connection for a session."""
        if session_id in self._ws_connections:
            ws = self._ws_connections.pop(session_id)
            try:
                await ws.close()
                logger.debug("WebSocket connection closed", session_id=session_id)
            except Exception as e:
                logger.warning(
                    "Error closing WebSocket connection",
                    session_id=session_id,
                    error=str(e),
                )

    async def _ensure_agent_card(self) -> None:
        """Fetch and cache agent card if not already loaded."""
        if self.agent_card is not None:
            return

        logger.debug("Fetching agent card", url=self.agent_card_url)

        response = await self.client.get(
            self.agent_card_url,
            headers=self._auth_headers(),
        )
        response.raise_for_status()

        data = response.json()
        self.agent_card = AgentCard.from_dict(data)

        # Extract interface type and endpoint from agent card
        self._interface_type, base_url = self.agent_card.get_interface()
        if not base_url:
            raise ValueError("Agent card missing 'url' or 'interfaces' with url")

        self._endpoint = base_url.rstrip("/")

        logger.info(
            "Agent card loaded",
            name=self.agent_card.name,
            version=self.agent_card.version,
            interface_type=self._interface_type,
            endpoint=self._endpoint,
        )

    async def cleanup(self) -> None:
        """Clean up all connections (HTTP client and WebSocket connections)."""
        # Close all WebSocket connections
        for session_id in list(self._ws_connections.keys()):
            await self._close_ws_connection(session_id)

        # Close HTTP client
        await self.client.aclose()

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

    def _build_jsonrpc_request(
        self,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        """Build a JSON-RPC 2.0 request."""
        return {
            "jsonrpc": "2.0",
            "id": str(uuid4()),
            "method": method,
            "params": params,
        }

    async def create_session(self) -> str:
        """Create a new conversation session.

        A2A uses contextId for session management, generated client-side.
        """
        return str(uuid4())

    async def send_message(self, request: ConversationRequest) -> ConversationResponse:
        """Send a message via A2A protocol.

        Routes to HTTP or WebSocket based on the agent card interface type.
        """
        await self._ensure_agent_card()

        if self._interface_type == "websocket":
            return await self._send_message_ws(request)
        else:
            return await self._send_message_http(request)

    async def _send_message_http(self, request: ConversationRequest) -> ConversationResponse:
        """Send a message via HTTP POST."""
        start_time = time.monotonic()
        latest_message = request.messages[-1].content

        # Build A2A message/send request
        jsonrpc_request = self._build_jsonrpc_request(
            method="message/send",
            params={
                "message": {
                    "role": "user",
                    "parts": [{"kind": "text", "text": latest_message}],
                    "messageId": str(uuid4()),
                },
                "configuration": {
                    "contextId": request.session_id,
                },
            },
        )

        logger.debug(
            "Sending A2A message via HTTP",
            endpoint=self._endpoint,
            session_id=request.session_id,
        )

        response = await self.client.post(
            self._endpoint,
            json=jsonrpc_request,
            headers={
                **self._auth_headers(),
                "Content-Type": "application/json",
            },
            timeout=request.timeout_seconds,
        )

        latency_ms = int((time.monotonic() - start_time) * 1000)

        if response.status_code != 200:
            logger.warning(
                "A2A HTTP error response",
                status_code=response.status_code,
                endpoint=self._endpoint,
            )
            raise Exception(f"A2A request failed with status {response.status_code}")

        response_data = response.json()

        # Extract content from response
        content = self._extract_content(response_data)

        # Handle task-based responses (async tasks)
        if self._is_task_response(response_data):
            content = await self._poll_task_completion(
                response_data, request.timeout_seconds
            )

        return ConversationResponse(
            session_id=request.session_id or str(uuid4()),
            content=content,
            latency_ms=latency_ms,
            raw_response=response_data,
        )

    async def _send_message_ws(self, request: ConversationRequest) -> ConversationResponse:
        """Send a message via WebSocket."""
        start_time = time.monotonic()
        latest_message = request.messages[-1].content
        session_id = request.session_id or str(uuid4())

        ws = await self._get_ws_connection(session_id)

        # Build A2A message/send request
        jsonrpc_request = self._build_jsonrpc_request(
            method="message/send",
            params={
                "message": {
                    "role": "user",
                    "parts": [{"kind": "text", "text": latest_message}],
                    "messageId": str(uuid4()),
                },
                "configuration": {
                    "contextId": session_id,
                },
            },
        )

        logger.debug(
            "Sending A2A message via WebSocket",
            endpoint=self._endpoint,
            session_id=session_id,
        )

        # Send JSON-RPC request over WebSocket
        await ws.send(json.dumps(jsonrpc_request))

        # Receive response
        response_text = await ws.recv()
        latency_ms = int((time.monotonic() - start_time) * 1000)

        response_data = json.loads(response_text)

        # Extract content from response
        content = self._extract_content(response_data)

        # Handle task-based responses (async tasks) - poll via WebSocket
        if self._is_task_response(response_data):
            content = await self._poll_task_completion_ws(
                ws, response_data, request.timeout_seconds
            )

        return ConversationResponse(
            session_id=session_id,
            content=content,
            latency_ms=latency_ms,
            raw_response=response_data,
        )

    def _is_task_response(self, response_data: dict[str, Any]) -> bool:
        """Check if response is a task (async) vs direct message."""
        result = response_data.get("result", {})
        return bool(result.get("kind") == "task")

    def _extract_content(self, response_data: dict[str, Any]) -> str:
        """Extract text content from A2A response."""
        result = response_data.get("result", {})

        # Direct message response
        if result.get("kind") == "message":
            parts = result.get("parts", [])
            return self._extract_text_from_parts(parts)

        # Task response - extract from artifacts or status
        if result.get("kind") == "task":
            artifacts = result.get("artifacts", [])
            for artifact in artifacts:
                parts = artifact.get("parts", [])
                text = self._extract_text_from_parts(parts)
                if text:
                    return text

            state = result.get("state", "")
            return f"[Task {result.get('taskId', 'unknown')}: {state}]"

        # Error response
        if "error" in response_data:
            error = response_data["error"]
            return f"[Error: {error.get('message', 'Unknown error')}]"

        return str(result)

    def _extract_text_from_parts(self, parts: list[dict[str, Any]]) -> str:
        """Extract text from message parts."""
        texts = []
        for part in parts:
            if part.get("kind") == "text":
                texts.append(part.get("text", ""))
        return "\n".join(texts)

    async def _poll_task_completion(
        self,
        initial_response: dict[str, Any],
        timeout_seconds: float,
    ) -> str:
        """Poll for task completion for async tasks via HTTP."""
        import asyncio

        result = initial_response.get("result", {})
        task_id = result.get("taskId")

        if not task_id:
            return self._extract_content(initial_response)

        start_time = time.monotonic()
        poll_interval = 1.0

        while (time.monotonic() - start_time) < timeout_seconds:
            jsonrpc_request = self._build_jsonrpc_request(
                method="tasks/get",
                params={"taskId": task_id},
            )

            response = await self.client.post(
                self._endpoint,
                json=jsonrpc_request,
                headers={
                    **self._auth_headers(),
                    "Content-Type": "application/json",
                },
            )

            if response.status_code != 200:
                logger.warning("Failed to poll task", task_id=task_id)
                break

            task_data = response.json()
            task_result = task_data.get("result", {})
            state = task_result.get("state", "")

            if state in ("completed", "failed", "canceled", "rejected"):
                return self._extract_content(task_data)

            if state == "input-required":
                return "[Agent requires additional input]"

            await asyncio.sleep(poll_interval)
            poll_interval = min(poll_interval * 1.5, 5.0)

        return f"[Task {task_id} timed out]"

    async def _poll_task_completion_ws(
        self,
        ws: WebSocketClientProtocol,
        initial_response: dict[str, Any],
        timeout_seconds: float,
    ) -> str:
        """Poll for task completion via WebSocket for async tasks."""
        import asyncio

        result = initial_response.get("result", {})
        task_id = result.get("taskId")

        if not task_id:
            return self._extract_content(initial_response)

        start_time = time.monotonic()
        poll_interval = 1.0

        while (time.monotonic() - start_time) < timeout_seconds:
            jsonrpc_request = self._build_jsonrpc_request(
                method="tasks/get",
                params={"taskId": task_id},
            )

            await ws.send(json.dumps(jsonrpc_request))
            response_text = await ws.recv()

            try:
                task_data = json.loads(response_text)
                task_result = task_data.get("result", {})
                state = task_result.get("state", "")

                if state in ("completed", "failed", "canceled", "rejected"):
                    return self._extract_content(task_data)

                if state == "input-required":
                    return "[Agent requires additional input]"

            except json.JSONDecodeError:
                logger.warning("Failed to parse task poll response", task_id=task_id)
                break

            await asyncio.sleep(poll_interval)
            poll_interval = min(poll_interval * 1.5, 5.0)

        return f"[Task {task_id} timed out]"

    async def send_message_streaming(
        self,
        request: ConversationRequest,
    ) -> AsyncIterator[str]:
        """Send a message and stream response tokens.

        Routes to HTTP (SSE) or WebSocket based on the agent card interface type.
        Falls back to non-streaming if agent doesn't support streaming.
        """
        await self._ensure_agent_card()

        # Check if agent supports streaming
        if not (self.agent_card and self.agent_card.capabilities.get("streaming")):
            logger.debug(
                "Agent does not support streaming, falling back to non-streaming"
            )
            response = await self.send_message(request)
            yield response.content
            return

        if self._interface_type == "websocket":
            async for chunk in self._send_message_streaming_ws(request):
                yield chunk
        else:
            async for chunk in self._send_message_streaming_http(request):
                yield chunk

    async def _send_message_streaming_http(
        self,
        request: ConversationRequest,
    ) -> AsyncIterator[str]:
        """Stream response via HTTP SSE."""
        latest_message = request.messages[-1].content

        # Build A2A message/stream request (SSE)
        jsonrpc_request = self._build_jsonrpc_request(
            method="message/stream",
            params={
                "message": {
                    "role": "user",
                    "parts": [{"kind": "text", "text": latest_message}],
                    "messageId": str(uuid4()),
                },
                "configuration": {
                    "contextId": request.session_id,
                },
            },
        )

        logger.debug(
            "Sending A2A streaming message via HTTP SSE",
            endpoint=self._endpoint,
            session_id=request.session_id,
        )

        try:
            async with self.client.stream(
                "POST",
                self._endpoint,
                json=jsonrpc_request,
                headers={
                    **self._auth_headers(),
                    "Content-Type": "application/json",
                    "Accept": "text/event-stream",
                },
                timeout=request.timeout_seconds,
            ) as response:
                response.raise_for_status()

                async for line in response.aiter_lines():
                    if not line or line.startswith(":"):
                        # Empty line or comment, skip
                        continue

                    if line.startswith("data: "):
                        data = line[6:]
                        if data == "[DONE]":
                            break

                        try:
                            event = json.loads(data)
                            result = event.get("result", {})

                            # Extract text from streaming message parts
                            if result.get("kind") == "message":
                                parts = result.get("parts", [])
                                text = self._extract_text_from_parts(parts)
                                if text:
                                    yield text
                            # Handle artifact chunks
                            elif result.get("kind") == "artifact-chunk":
                                parts = result.get("parts", [])
                                text = self._extract_text_from_parts(parts)
                                if text:
                                    yield text
                        except json.JSONDecodeError:
                            logger.warning("Failed to parse SSE data", data=data)
                            continue

        except httpx.HTTPStatusError as e:
            logger.warning("A2A HTTP streaming failed, falling back", error=str(e))
            response = await self.send_message(request)
            yield response.content

    async def _send_message_streaming_ws(
        self,
        request: ConversationRequest,
    ) -> AsyncIterator[str]:
        """Stream response via WebSocket."""
        latest_message = request.messages[-1].content
        session_id = request.session_id or str(uuid4())

        ws = await self._get_ws_connection(session_id)

        # Build A2A message/stream request
        jsonrpc_request = self._build_jsonrpc_request(
            method="message/stream",
            params={
                "message": {
                    "role": "user",
                    "parts": [{"kind": "text", "text": latest_message}],
                    "messageId": str(uuid4()),
                },
                "configuration": {
                    "contextId": session_id,
                },
            },
        )

        logger.debug(
            "Sending A2A streaming message via WebSocket",
            endpoint=self._endpoint,
            session_id=session_id,
        )

        # Send the streaming request
        await ws.send(json.dumps(jsonrpc_request))

        # Receive streaming messages
        try:
            async for message in ws:
                try:
                    data = json.loads(message)
                    result = data.get("result", {})

                    # Check for completion or error
                    if result.get("kind") == "status":
                        state = result.get("state", "")
                        if state in ("completed", "failed", "canceled", "rejected"):
                            break
                        if state == "input-required":
                            yield "[Agent requires additional input]"
                            break

                    # Extract text from streaming message parts
                    if result.get("kind") == "message":
                        parts = result.get("parts", [])
                        text = self._extract_text_from_parts(parts)
                        if text:
                            yield text

                    # Handle artifact chunks
                    elif result.get("kind") == "artifact-chunk":
                        parts = result.get("parts", [])
                        text = self._extract_text_from_parts(parts)
                        if text:
                            yield text

                    # Check for end marker in JSON-RPC response
                    if "error" in data:
                        error = data["error"]
                        yield f"[Error: {error.get('message', 'Unknown error')}]"
                        break

                except json.JSONDecodeError:
                    logger.warning("Failed to parse WebSocket message", message=message)
                    continue

        except websockets.exceptions.ConnectionClosed:
            logger.warning("WebSocket connection closed during streaming")
        except Exception as e:
            logger.warning("WebSocket streaming error", error=str(e))
            # Fall back to non-streaming
            response = await self._send_message_ws(request)
            yield response.content

    async def close_session(self, session_id: str) -> None:
        """Close a conversation session.

        For HTTP: A2A doesn't have explicit session close - contextId is just an identifier.
        For WebSocket: Closes the WebSocket connection for this session.
        """
        await self._close_ws_connection(session_id)
        logger.debug("A2A session closed", session_id=session_id)

    async def health_check(self) -> bool:
        """Check if the agent is reachable by fetching agent card."""
        try:
            response = await self.client.get(
                self.agent_card_url,
                timeout=10.0,
            )
            return response.status_code == 200
        except Exception:
            return False
