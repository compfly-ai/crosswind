"""Session management for agent interactions."""

from datetime import datetime

import structlog

from crosswind.models import Message, SessionState
from crosswind.protocols.base import ProtocolAdapter

logger = structlog.get_logger()


class SessionManager:
    """Manages agent sessions with error tracking and reset logic.

    Handles session lifecycle, tracks errors, and determines when to reset
    sessions due to repeated failures.
    """

    def __init__(
        self,
        adapter: ProtocolAdapter,
        max_consecutive_errors: int = 5,
        session_strategy: str = "agent_managed",
    ) -> None:
        """Initialize the session manager.

        Args:
            adapter: Protocol adapter for the agent
            max_consecutive_errors: Maximum consecutive errors before reset
            session_strategy: How to manage session state
        """
        self.adapter = adapter
        self.max_consecutive_errors = max_consecutive_errors
        self.session_strategy = session_strategy

        self.active_sessions: dict[str, SessionState] = {}
        self.conversation_histories: dict[str, list[Message]] = {}
        self._current_session_id: str | None = None

    async def get_or_create_session(self) -> str:
        """Get an existing session or create a new one.

        Returns:
            Session ID
        """
        if self._current_session_id and self._current_session_id in self.active_sessions:
            session = self.active_sessions[self._current_session_id]
            if session.consecutive_errors < self.max_consecutive_errors:
                return self._current_session_id

        return await self.create_new_session()

    async def create_new_session(self) -> str:
        """Create a new session.

        Returns:
            New session ID
        """
        session_id = await self.adapter.create_session()

        self.active_sessions[session_id] = SessionState(
            id=session_id,
            consecutive_errors=0,
            total_successes=0,
            total_errors=0,
            prompts_executed=0,
        )

        if self.session_strategy == "client_history":
            self.conversation_histories[session_id] = []

        self._current_session_id = session_id
        logger.info("Created new session", session_id=session_id)

        return session_id

    def get_conversation_history(self, session_id: str) -> list[Message]:
        """Get conversation history for a session.

        Args:
            session_id: Session ID

        Returns:
            List of messages in the conversation
        """
        return self.conversation_histories.get(session_id, [])

    def add_to_history(self, session_id: str, message: Message) -> None:
        """Add a message to the conversation history.

        Args:
            session_id: Session ID
            message: Message to add
        """
        if self.session_strategy == "client_history":
            if session_id not in self.conversation_histories:
                self.conversation_histories[session_id] = []
            self.conversation_histories[session_id].append(message)

    def record_success(self, session_id: str) -> None:
        """Record a successful interaction.

        Args:
            session_id: Session ID
        """
        if session_id in self.active_sessions:
            session = self.active_sessions[session_id]
            session.consecutive_errors = 0
            session.total_successes += 1
            session.prompts_executed += 1

    def record_error(self, session_id: str, error: str | None = None) -> None:
        """Record an error for a session.

        Args:
            session_id: Session ID
            error: Error message
        """
        if session_id in self.active_sessions:
            session = self.active_sessions[session_id]
            session.consecutive_errors += 1
            session.total_errors += 1
            session.last_error = error
            session.last_error_time = datetime.utcnow()

            logger.warning(
                "Session error recorded",
                session_id=session_id,
                consecutive_errors=session.consecutive_errors,
                error=error,
            )

    def should_reset_session(self, session_id: str) -> bool:
        """Check if a session should be reset due to errors.

        Args:
            session_id: Session ID

        Returns:
            True if session should be reset
        """
        if session_id not in self.active_sessions:
            return False

        session = self.active_sessions[session_id]
        return session.consecutive_errors >= self.max_consecutive_errors

    async def reset_session(self, session_id: str) -> str:
        """Reset a session by closing it and creating a new one.

        Args:
            session_id: Session ID to reset

        Returns:
            New session ID
        """
        logger.info("Resetting session", session_id=session_id)

        await self.close_session(session_id)
        return await self.create_new_session()

    async def close_session(self, session_id: str) -> None:
        """Close a session.

        Args:
            session_id: Session ID to close
        """
        if session_id in self.active_sessions:
            del self.active_sessions[session_id]

        if session_id in self.conversation_histories:
            del self.conversation_histories[session_id]

        try:
            await self.adapter.close_session(session_id)
        except Exception as e:
            logger.debug("Error closing session", session_id=session_id, error=str(e))

        if self._current_session_id == session_id:
            self._current_session_id = None

    async def close_all_sessions(self) -> None:
        """Close all active sessions."""
        for session_id in list(self.active_sessions.keys()):
            await self.close_session(session_id)

    def get_session_stats(self, session_id: str) -> SessionState | None:
        """Get statistics for a session.

        Args:
            session_id: Session ID

        Returns:
            Session state or None if not found
        """
        return self.active_sessions.get(session_id)

    def get_total_stats(self) -> dict[str, int]:
        """Get aggregate statistics across all sessions.

        Returns:
            Dictionary of statistics
        """
        total_successes = 0
        total_errors = 0
        total_prompts = 0

        for session in self.active_sessions.values():
            total_successes += session.total_successes
            total_errors += session.total_errors
            total_prompts += session.prompts_executed

        return {
            "sessions_created": len(self.active_sessions),
            "total_successes": total_successes,
            "total_errors": total_errors,
            "total_prompts": total_prompts,
        }
