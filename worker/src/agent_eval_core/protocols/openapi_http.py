"""OpenAPI HTTP protocol adapter."""

import time
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any
from uuid import uuid4

import httpx
import structlog

from agent_eval_core.models import AuthConfig, ConversationRequest, ConversationResponse, Message
from agent_eval_core.protocols.base import ProtocolAdapter

logger = structlog.get_logger()


class HTTPAgentError(Exception):
    """Exception raised when the agent returns a non-200 HTTP status code."""

    def __init__(self, status_code: int, message: str, url: str) -> None:
        self.status_code = status_code
        self.message = message
        self.url = url
        super().__init__(message)

    def is_auth_error(self) -> bool:
        """Check if this is an authentication error (401/403)."""
        return self.status_code in (401, 403)

    def is_server_error(self) -> bool:
        """Check if this is a server error (5xx)."""
        return 500 <= self.status_code < 600

    def is_rate_limit(self) -> bool:
        """Check if this is a rate limit error (429)."""
        return self.status_code == 429


# API Style constants
API_STYLE_CHAT_STATELESS = "chat_stateless"
API_STYLE_SINGLE_MESSAGE = "single_message"
API_STYLE_THREAD_BASED = "thread_based"
API_STYLE_TASK_BASED = "task_based"
API_STYLE_LANGSERVE = "langserve"
API_STYLE_FLOWISE = "flowise"
API_STYLE_DIFY = "dify"
API_STYLE_HAYSTACK = "haystack"
API_STYLE_BOTPRESS = "botpress"


@dataclass
class InferredSchema:
    """API schema inferred by LLM analysis."""

    api_style: str = API_STYLE_SINGLE_MESSAGE
    request_method: str = "POST"
    request_content_type: str = "application/json"
    message_field: str = "message"
    session_id_field: str | None = None
    history_field: str | None = None
    additional_fields: dict[str, Any] | None = None
    response_content_field: str = "response"
    response_error_field: str | None = None
    streaming_supported: bool = False
    session_id_in_response: str | None = None
    session_id_in_header: str | None = None
    session_create_method: str = "none"

    @classmethod
    def from_dict(cls, data: dict[str, Any] | None) -> "InferredSchema | None":
        """Create from dictionary (MongoDB document)."""
        if not data:
            return None

        api_style = data.get("apiStyle")
        if not api_style:
            message_field = data.get("messageField", "message")
            history_field = data.get("historyField")
            if message_field == "messages" or (history_field and message_field == history_field):
                api_style = API_STYLE_CHAT_STATELESS
            else:
                api_style = API_STYLE_SINGLE_MESSAGE

        return cls(
            api_style=api_style,
            request_method=data.get("requestMethod", "POST"),
            request_content_type=data.get("requestContentType", "application/json"),
            message_field=data.get("messageField", "message"),
            session_id_field=data.get("sessionIdField"),
            history_field=data.get("historyField"),
            additional_fields=data.get("additionalFields"),
            response_content_field=data.get("responseContentField", "response"),
            response_error_field=data.get("responseErrorField"),
            streaming_supported=data.get("streamingSupported", False),
            session_id_in_response=data.get("sessionIdInResponse"),
            session_id_in_header=data.get("sessionIdInHeader"),
            session_create_method=data.get("sessionCreateMethod", "none"),
        )


class OpenAPIHttpAdapter(ProtocolAdapter):
    """Adapter for OpenAPI-defined HTTP agents."""

    def __init__(
        self,
        base_url: str,
        conversation_endpoint: str,
        session_endpoint: str | None = None,
        auth_config: AuthConfig | None = None,
        spec_url: str | None = None,
        inferred_schema: dict[str, Any] | None = None,
        timeout: float = 120.0,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.conversation_endpoint = conversation_endpoint
        self.session_endpoint = session_endpoint
        self.auth_config = auth_config or AuthConfig()
        self.spec_url = spec_url
        self.schema = InferredSchema.from_dict(inferred_schema)
        self.timeout = timeout
        self.client = httpx.AsyncClient(timeout=timeout)

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
        elif self.auth_config.type == "custom":
            header_name = self.auth_config.header_name or "Authorization"
            prefix = self.auth_config.header_prefix or ""
            return {header_name: f"{prefix}{self.auth_config.credentials}"}
        else:
            return {}

    def _build_payload(self, request: ConversationRequest) -> dict[str, Any]:
        """Build request payload based on API style."""
        payload: dict[str, Any] = {}
        latest_message = request.messages[-1].content

        if self.schema:
            if self.schema.api_style == API_STYLE_CHAT_STATELESS:
                messages = [{"role": m.role, "content": m.content} for m in request.messages]
                payload[self.schema.message_field] = messages

            elif self.schema.api_style == API_STYLE_SINGLE_MESSAGE:
                payload[self.schema.message_field] = latest_message

            elif self.schema.api_style == API_STYLE_LANGSERVE:
                input_key = self.schema.message_field if self.schema.message_field != "input" else "message"
                payload["input"] = {input_key: latest_message}
                if request.session_id:
                    payload["config"] = {"configurable": {"session_id": request.session_id}}

            elif self.schema.api_style == API_STYLE_FLOWISE:
                payload["question"] = latest_message
                if len(request.messages) > 1:
                    history = []
                    for i in range(0, len(request.messages) - 1, 2):
                        user_msg = request.messages[i].content if i < len(request.messages) else ""
                        assistant_msg = request.messages[i + 1].content if i + 1 < len(request.messages) else ""
                        if user_msg or assistant_msg:
                            history.append({"user": user_msg, "assistant": assistant_msg})
                    payload["history"] = history
                else:
                    payload["history"] = []
                if request.session_id:
                    payload["sessionId"] = request.session_id

            elif self.schema.api_style == API_STYLE_DIFY:
                if self.schema.message_field == "inputs":
                    payload["inputs"] = {"query": latest_message}
                    payload["response_mode"] = "blocking"
                else:
                    payload["query"] = latest_message
                payload["user"] = "eval-user"
                if request.session_id:
                    payload["conversation_id"] = request.session_id

            elif self.schema.api_style == API_STYLE_HAYSTACK:
                payload["query"] = latest_message
                payload["params"] = {}

            elif self.schema.api_style == API_STYLE_BOTPRESS:
                payload["type"] = "text"
                payload["payload"] = {"text": latest_message}
                if request.session_id:
                    payload["conversationId"] = request.session_id
                payload["userId"] = "eval-user"

            else:
                payload[self.schema.message_field] = latest_message

            if request.session_id and self.schema.session_id_field:
                payload[self.schema.session_id_field] = request.session_id

            if self.schema.additional_fields:
                payload.update(self.schema.additional_fields)

        else:
            messages = [{"role": m.role, "content": m.content} for m in request.messages]
            payload = {"messages": messages}
            if request.session_id:
                payload["session_id"] = request.session_id

        return payload

    def _extract_content(self, response_data: dict[str, Any]) -> str:
        """Extract assistant message content from response."""
        if self.schema and self.schema.response_content_field:
            value = self._extract_by_path(response_data, self.schema.response_content_field)
            if value is not None:
                return str(value)

        # OpenAI format
        if "choices" in response_data:
            choices = response_data["choices"]
            if choices and len(choices) > 0:
                choice = choices[0]
                if "message" in choice:
                    return choice["message"].get("content", "")
                elif "text" in choice:
                    return choice["text"]

        # Simple formats
        for field in ["response", "message", "content", "text", "answer"]:
            if field in response_data:
                val = response_data[field]
                if isinstance(val, dict):
                    return val.get("content", str(val))
                return str(val)

        return str(response_data)

    def _extract_by_path(self, data: dict[str, Any], path: str) -> Any:
        """Extract value from nested dict using dot notation and array access."""
        import re

        current = data
        parts = re.split(r'\.(?![^\[]*\])', path)

        for part in parts:
            if current is None:
                return None

            array_match = re.match(r'(\w+)\[(\d+)\]', part)
            if array_match:
                key, index = array_match.groups()
                if isinstance(current, dict) and key in current:
                    arr = current[key]
                    if isinstance(arr, list) and len(arr) > int(index):
                        current = arr[int(index)]
                    else:
                        return None
                else:
                    return None
            else:
                if isinstance(current, dict) and part in current:
                    current = current[part]
                else:
                    return None

        return current

    async def create_session(self) -> str:
        """Create a new conversation session."""
        session_method = "none"
        if self.schema:
            session_method = self.schema.session_create_method or "none"

        if session_method == "explicit" or self.session_endpoint:
            if self.session_endpoint:
                try:
                    response = await self.client.post(
                        f"{self.base_url}{self.session_endpoint}",
                        headers=self._auth_headers(),
                    )
                    response.raise_for_status()
                    data = response.json()
                    session_id = self._extract_session_id_from_response(data)
                    if session_id:
                        return session_id
                    return data.get("session_id", data.get("id", str(uuid4())))
                except Exception as e:
                    logger.warning("Failed to create session via endpoint", error=str(e))

        if session_method == "auto":
            return f"pending_{uuid4()}"

        return str(uuid4())

    def _extract_session_id_from_response(self, response_data: dict[str, Any]) -> str | None:
        """Extract session ID from response using inferred schema."""
        if self.schema and self.schema.session_id_in_response:
            value = self._extract_by_path(response_data, self.schema.session_id_in_response)
            if value is not None:
                return str(value)

        for field in ["session_id", "sessionId", "id", "conversation_id", "conversationId", "thread_id"]:
            if field in response_data:
                return str(response_data[field])

        return None

    async def send_message(self, request: ConversationRequest) -> ConversationResponse:
        """Send a message and get response."""
        start_time = time.monotonic()

        payload = self._build_payload(request)
        url = f"{self.base_url}{self.conversation_endpoint}"
        headers = self._build_request_headers(request.session_id)

        logger.debug(
            "Sending message",
            url=url,
            session_id=request.session_id,
            payload_keys=list(payload.keys()),
            has_schema=self.schema is not None,
            api_style=self.schema.api_style if self.schema else None,
        )

        response = await self.client.post(
            url,
            json=payload,
            headers=headers,
            timeout=request.timeout_seconds,
        )

        latency_ms = int((time.monotonic() - start_time) * 1000)

        if response.status_code != 200:
            logger.warning(
                "HTTP error from agent",
                status_code=response.status_code,
                url=url,
            )
            raise HTTPAgentError(
                status_code=response.status_code,
                message=f"Agent returned HTTP {response.status_code}: {response.reason_phrase}",
                url=url,
            )

        response_data = response.json()
        content = self._extract_content(response_data)
        session_id = self._resolve_session_id(request.session_id, response_data, response.headers)

        return ConversationResponse(
            session_id=session_id,
            content=content,
            latency_ms=latency_ms,
            raw_response=response_data,
        )

    def _build_request_headers(self, session_id: str | None) -> dict[str, str]:
        """Build request headers including auth and optional session ID."""
        headers = self._auth_headers()

        if session_id and self.schema and self.schema.session_id_in_header:
            if not session_id.startswith("pending_"):
                headers[self.schema.session_id_in_header] = session_id

        return headers

    def _resolve_session_id(
        self,
        request_session_id: str | None,
        response_data: dict[str, Any],
        response_headers: httpx.Headers,
    ) -> str:
        """Resolve final session ID from request, response body, or headers."""
        if self.schema and self.schema.session_id_in_header:
            header_session = response_headers.get(self.schema.session_id_in_header)
            if header_session:
                return header_session

        extracted_session = self._extract_session_id_from_response(response_data)
        if extracted_session:
            return extracted_session

        if request_session_id and request_session_id.startswith("pending_"):
            logger.warning("Auto session mode but no session ID in response, generating UUID")
            return str(uuid4())

        return request_session_id or str(uuid4())

    async def send_message_streaming(
        self, request: ConversationRequest
    ) -> AsyncIterator[str]:
        """Send a message and stream response tokens."""
        payload = self._build_payload(request)
        payload["stream"] = True
        url = f"{self.base_url}{self.conversation_endpoint}"

        try:
            async with self.client.stream(
                "POST",
                url,
                json=payload,
                headers=self._auth_headers(),
                timeout=request.timeout_seconds,
            ) as response:
                response.raise_for_status()

                async for line in response.aiter_lines():
                    if line.startswith("data: "):
                        data = line[6:]
                        if data == "[DONE]":
                            break
                        try:
                            import json
                            chunk = json.loads(data)
                            if "choices" in chunk:
                                delta = chunk["choices"][0].get("delta", {})
                                content = delta.get("content", "")
                                if content:
                                    yield content
                            elif "content" in chunk:
                                yield chunk["content"]
                            elif "text" in chunk:
                                yield chunk["text"]
                        except Exception:
                            continue

        except httpx.HTTPStatusError:
            response = await self.send_message(request)
            yield response.content

    async def close_session(self, session_id: str) -> None:
        """Close a conversation session."""
        if self.session_endpoint:
            try:
                await self.client.delete(
                    f"{self.base_url}{self.session_endpoint}/{session_id}",
                    headers=self._auth_headers(),
                )
            except Exception as e:
                logger.debug("Failed to close session", session_id=session_id, error=str(e))

    async def health_check(self) -> bool:
        """Check if the agent is reachable."""
        try:
            health_url = f"{self.base_url}/health"
            response = await self.client.get(health_url, timeout=10.0)
            return response.status_code < 500
        except Exception:
            try:
                response = await self.client.get(self.base_url, timeout=10.0)
                return response.status_code < 500
            except Exception:
                return False
