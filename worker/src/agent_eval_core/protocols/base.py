"""Abstract base class for protocol adapters."""

from abc import ABC, abstractmethod
from collections.abc import AsyncIterator

from agent_eval_core.models import ConversationRequest, ConversationResponse


class ProtocolAdapter(ABC):
    """Abstract base class for agent protocol adapters.

    Each adapter implements communication with a specific type of agent
    (HTTP API, WebSocket, A2A, MCP, etc.).
    """

    @abstractmethod
    async def create_session(self) -> str:
        """Create a new conversation session.

        Returns:
            Session ID
        """
        pass

    @abstractmethod
    async def send_message(self, request: ConversationRequest) -> ConversationResponse:
        """Send a message and get a complete response.

        For streaming APIs, this collects the full response before returning.

        Args:
            request: The conversation request

        Returns:
            The complete response
        """
        pass

    @abstractmethod
    async def send_message_streaming(
        self, request: ConversationRequest
    ) -> AsyncIterator[str]:
        """Send a message and stream response tokens.

        Args:
            request: The conversation request

        Yields:
            Response tokens as they arrive
        """
        pass

    @abstractmethod
    async def close_session(self, session_id: str) -> None:
        """Close a conversation session.

        Args:
            session_id: The session to close
        """
        pass

    @abstractmethod
    async def health_check(self) -> bool:
        """Check if the agent endpoint is reachable.

        Returns:
            True if healthy, False otherwise
        """
        pass

    async def __aenter__(self) -> "ProtocolAdapter":
        """Async context manager entry."""
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:  # type: ignore[no-untyped-def]
        """Async context manager exit."""
        await self.cleanup()

    async def cleanup(self) -> None:
        """Clean up resources. Override in subclasses if needed."""
        pass
