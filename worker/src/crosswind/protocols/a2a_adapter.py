"""A2A (Agent-to-Agent) protocol adapter.

Implements the Google A2A protocol for agent-to-agent communication.
See: https://github.com/google/A2A
"""

import time
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any
from uuid import uuid4

import httpx
import structlog

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
        """Extract service URL from agent card.

        Checks both the direct 'url' field and 'interfaces' array.
        """
        # First check direct url field (newer A2A spec)
        if self.url:
            return self.url

        # Fall back to interfaces array
        for interface in self.interfaces:
            if interface.get("type") in ("http", "json-rpc"):
                return interface.get("url", "")

        # Fallback to first interface
        if self.interfaces:
            return self.interfaces[0].get("url", "")

        return ""


class A2AAdapter(ProtocolAdapter):
    """Adapter for A2A (Agent-to-Agent) protocol.

    Flow:
    1. Fetch agent card from agent_card_url
    2. Extract endpoint URL from agent card
    3. Send JSON-RPC 2.0 messages to that endpoint
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

        # Extract endpoint from agent card
        base_url = self.agent_card.get_url()
        if not base_url:
            raise ValueError("Agent card missing 'url' or 'interfaces' with url")

        self._endpoint = base_url.rstrip("/")

        logger.info(
            "Agent card loaded",
            name=self.agent_card.name,
            version=self.agent_card.version,
            endpoint=self._endpoint,
        )

    async def cleanup(self) -> None:
        """Clean up HTTP client."""
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
        """Send a message via A2A protocol."""
        await self._ensure_agent_card()

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
            "Sending A2A message",
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
                "A2A error response",
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

    def _is_task_response(self, response_data: dict[str, Any]) -> bool:
        """Check if response is a task (async) vs direct message."""
        result = response_data.get("result", {})
        return result.get("kind") == "task"

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
        """Poll for task completion for async tasks."""
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

    async def send_message_streaming(
        self,
        request: ConversationRequest,
    ) -> AsyncIterator[str]:
        """Send a message and stream response tokens via SSE.

        A2A supports streaming via message/stream method if agent capabilities
        indicate support. Falls back to non-streaming otherwise.
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
            "Sending A2A streaming message",
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
                            import json

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
            logger.warning("A2A streaming failed, falling back", error=str(e))
            response = await self.send_message(request)
            yield response.content

    async def close_session(self, session_id: str) -> None:
        """Close a conversation session.

        A2A doesn't have explicit session close - contextId is just an identifier.
        """
        logger.debug("A2A session close (no-op)", session_id=session_id)

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
