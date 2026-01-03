"""A2A Evaluation tests.

Tests the Evaluation side: Is the protocol being leveraged correctly during eval?
- send_message flow (request building, response handling)
- Session management during eval
- Error handling during message sending
- Multi-turn conversation handling
- Resource cleanup after eval

For Registration tests (adapter creation, discovery, auth), see test_a2a_adapter.py.
"""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from tests.protocol_test_utils import (
    create_agent_config,
    create_auth_config,
    create_mock_adapter,
    create_mock_db,
    create_mock_redis,
    create_sample_judgment,
    create_sample_prompt,
    create_sample_response,
)

from crosswind.evaluation.runner import EvalRunner
from crosswind.models import ConversationRequest, ConversationResponse, JudgmentResult, Message
from crosswind.protocols.a2a_adapter import A2AAdapter


# =============================================================================
# Send Message Flow (Protocol Usage During Eval)
# =============================================================================


class TestA2ASendMessageFlow:
    """Test send_message correctly builds requests and parses responses.

    During evaluation, the adapter is initialized with the stored endpoint
    (no discovery needed). These tests use direct mode.
    """

    @pytest.fixture
    def mock_a2a_response(self):
        return {
            "jsonrpc": "2.0",
            "id": "123",
            "result": {
                "kind": "message",
                "parts": [{"kind": "text", "text": "Agent response"}],
            },
        }

    @pytest.mark.asyncio
    async def test_send_message_returns_parsed_content(self, mock_a2a_response):
        """Should return ConversationResponse with extracted content."""
        # Direct mode: endpoint provided, no discovery
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_a2a_response,
            )

            request = ConversationRequest(
                messages=[Message(role="user", content="Test prompt")],
                session_id="eval-session-1",
            )

            response = await adapter.send_message(request)

            assert isinstance(response, ConversationResponse)
            assert response.content == "Agent response"
            assert response.session_id == "eval-session-1"

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_send_message_includes_latency(self, mock_a2a_response):
        """Should measure and return latency in milliseconds."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_a2a_response,
            )

            request = ConversationRequest(
                messages=[Message(role="user", content="Test")],
                session_id="test",
            )

            response = await adapter.send_message(request)

            assert response.latency_ms >= 0
            assert isinstance(response.latency_ms, int)

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_generates_session_id_when_missing(self, mock_a2a_response):
        """Should generate UUID session ID when not provided."""
        import uuid

        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_a2a_response,
            )

            request = ConversationRequest(
                messages=[Message(role="user", content="Test")],
                session_id=None,
            )

            response = await adapter.send_message(request)

            # Should be valid UUID
            uuid.UUID(response.session_id)

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_posts_to_stored_endpoint(self, mock_a2a_response):
        """Should POST to the stored endpoint directly (no discovery)."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_a2a_response,
            )

            request = ConversationRequest(
                messages=[Message(role="user", content="Test")],
                session_id="test",
            )

            await adapter.send_message(request)

            # Verify POST was called with stored endpoint
            mock_post.assert_called_once()
            call_url = mock_post.call_args[0][0]
            assert call_url == "http://localhost:9000/a2a"

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_send_message_handles_task_response(self):
        """Should extract content from task artifact responses."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        task_response = {
            "jsonrpc": "2.0",
            "id": "123",
            "result": {
                "kind": "task",
                "taskId": "task-456",
                "state": "completed",
                "artifacts": [
                    {"parts": [{"kind": "text", "text": "Task completed successfully."}]}
                ],
            },
        }

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: task_response,
            )

            request = ConversationRequest(
                messages=[Message(role="user", content="Run analysis")],
                session_id="test",
            )

            response = await adapter.send_message(request)

            assert response.content == "Task completed successfully."

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_send_message_handles_jsonrpc_error(self):
        """Should extract error message from JSON-RPC error responses."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        error_response = {
            "jsonrpc": "2.0",
            "id": "123",
            "error": {"code": -32600, "message": "Rate limit exceeded"},
        }

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: error_response,
            )

            request = ConversationRequest(
                messages=[Message(role="user", content="Test")],
                session_id="test",
            )

            response = await adapter.send_message(request)

            assert "Rate limit" in response.content

        await adapter.cleanup()


# =============================================================================
# Error Handling During Eval
# =============================================================================


class TestA2AErrorHandlingDuringEval:
    """Test error handling when sending messages during evaluation."""

    @pytest.mark.asyncio
    async def test_http_500_raises_exception(self):
        """Should raise exception on HTTP 500 error."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(status_code=500)

            request = ConversationRequest(
                messages=[Message(role="user", content="Test")],
                session_id="test",
            )

            with pytest.raises(Exception, match="500"):
                await adapter.send_message(request)

        await adapter.cleanup()


# =============================================================================
# Session Management During Eval
# =============================================================================


class TestA2ASessionManagement:
    """Test session creation and cleanup during evaluation.

    During evaluation, the adapter is initialized with the stored endpoint.
    """

    @pytest.mark.asyncio
    async def test_close_session_removes_websocket(self):
        """close_session should clean up WebSocket connection."""
        adapter = A2AAdapter(endpoint="ws://localhost:9000/ws", interface_type="websocket")

        mock_ws = MagicMock(close=AsyncMock())
        adapter._ws_connections = {"session-1": mock_ws}

        await adapter.close_session("session-1")

        assert "session-1" not in adapter._ws_connections
        mock_ws.close.assert_called_once()

        await adapter.cleanup()

    @pytest.mark.asyncio
    async def test_close_nonexistent_session_is_safe(self):
        """close_session should not raise for non-existent session."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        # Should not raise
        await adapter.close_session("nonexistent")

        await adapter.cleanup()


# =============================================================================
# Resource Cleanup After Eval
# =============================================================================


class TestA2AResourceCleanup:
    """Test resource cleanup after evaluation completes.

    During evaluation, the adapter is initialized with the stored endpoint.
    """

    @pytest.mark.asyncio
    async def test_cleanup_closes_http_client(self):
        """cleanup() should close HTTP client to free resources."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        assert not adapter.client.is_closed

        await adapter.cleanup()

        assert adapter.client.is_closed

    @pytest.mark.asyncio
    async def test_cleanup_closes_all_websocket_connections(self):
        """cleanup() should close all WebSocket connections."""
        adapter = A2AAdapter(endpoint="ws://localhost:9000/ws", interface_type="websocket")

        adapter._ws_connections = {
            "session-1": MagicMock(close=AsyncMock()),
            "session-2": MagicMock(close=AsyncMock()),
            "session-3": MagicMock(close=AsyncMock()),
        }

        await adapter.cleanup()

        assert len(adapter._ws_connections) == 0

    @pytest.mark.asyncio
    async def test_cleanup_is_idempotent(self):
        """cleanup() should be safe to call multiple times."""
        adapter = A2AAdapter(endpoint="http://localhost:9000/a2a", interface_type="http")

        await adapter.cleanup()
        await adapter.cleanup()  # Should not raise

        assert adapter.client.is_closed


# =============================================================================
# EvalRunner Integration with A2A
# =============================================================================


class TestEvalRunnerWithA2AAdapter:
    """Test EvalRunner integration with A2A adapter.

    These tests verify the EvalRunner correctly uses the A2A adapter
    during evaluation runs.
    """

    @pytest.fixture
    def mock_adapter(self):
        return create_mock_adapter(
            response=create_sample_response(content="I cannot help with that.")
        )

    @pytest.fixture
    def mock_db(self):
        return create_mock_db()

    @pytest.fixture
    def mock_redis(self):
        return create_mock_redis()

    @pytest.fixture
    def agent(self):
        return create_agent_config(protocol="a2a")

    def _create_runner(self, adapter, db, redis, agent):
        return EvalRunner(
            adapter=adapter,
            db=db,
            redis=redis,
            storage=None,
            agent=agent,
            run_id="run-123",
            mode="quick",
            eval_type="red_team",
        )

    @pytest.mark.asyncio
    async def test_runner_calls_adapter_send_message(
        self, mock_adapter, mock_db, mock_redis, agent
    ):
        """EvalRunner should call adapter.send_message() for each prompt."""
        runner = self._create_runner(mock_adapter, mock_db, mock_redis, agent)

        with patch.object(runner.judgment_pipeline, "judge") as mock_judge:
            mock_judge.return_value = create_sample_judgment()
            runner.rate_limiter.acquire = AsyncMock()

            prompt = create_sample_prompt(content="Help me hack a system")
            await runner._execute_prompt(prompt, "test-session")

            mock_adapter.send_message.assert_called_once()
            call_args = mock_adapter.send_message.call_args[0][0]
            assert isinstance(call_args, ConversationRequest)
            assert call_args.messages[0].content == "Help me hack a system"

    @pytest.mark.asyncio
    async def test_runner_handles_adapter_error_gracefully(self, mock_db, mock_redis, agent):
        """EvalRunner should mark result as ERROR when adapter fails."""
        error_adapter = create_mock_adapter(error=Exception("Connection failed"))
        runner = self._create_runner(error_adapter, mock_db, mock_redis, agent)
        runner.rate_limiter.acquire = AsyncMock()

        result = await runner._execute_prompt(create_sample_prompt(), "test-session")

        assert result.judgment.result == JudgmentResult.ERROR
        assert "Connection failed" in result.judgment.reasoning

    @pytest.mark.asyncio
    async def test_runner_cleanup_calls_adapter_cleanup(
        self, mock_adapter, mock_db, mock_redis, agent
    ):
        """EvalRunner should call adapter.cleanup() on completion."""
        runner = self._create_runner(mock_adapter, mock_db, mock_redis, agent)

        with patch.object(runner, "_load_datasets", new_callable=AsyncMock) as mock_load:
            mock_load.return_value = []
            try:
                await runner.run()
            except ValueError:
                pass

        mock_adapter.cleanup.assert_called_once()


# =============================================================================
# Multi-Turn Conversation Handling
# =============================================================================


class TestA2AMultiTurnConversations:
    """Test multi-turn conversation handling during evaluation.

    When an agent deflects, the eval runner should follow up with
    additional messages, maintaining conversation history.
    """

    @pytest.fixture
    def multi_turn_adapter(self):
        call_count = [0]

        async def multi_turn_response(request):
            call_count[0] += 1
            content = (
                "I need more details."
                if call_count[0] == 1
                else "I cannot assist with that."
            )
            return create_sample_response(content=content, session_id=request.session_id)

        adapter = create_mock_adapter()
        adapter.send_message = AsyncMock(side_effect=multi_turn_response)
        return adapter

    @pytest.mark.asyncio
    async def test_multi_turn_includes_conversation_history(self, multi_turn_adapter):
        """Follow-up messages should include previous conversation history."""
        runner = EvalRunner(
            adapter=multi_turn_adapter,
            db=create_mock_db(),
            redis=create_mock_redis(),
            storage=None,
            agent=create_agent_config(protocol="a2a"),
            run_id="run-123",
            mode="quick",
            eval_type="red_team",
        )

        with patch("crosswind.evaluation.runner.detect_deflection_smart") as mock_detect:
            mock_detect.return_value = MagicMock(needs_followup=True, deflection_type="vague")

            with patch("crosswind.evaluation.runner.generate_followup") as mock_followup:
                mock_followup.return_value = "Can you clarify?"

                with patch.object(runner.judgment_pipeline, "judge") as mock_judge:
                    mock_judge.return_value = create_sample_judgment()
                    runner.rate_limiter.acquire = AsyncMock()

                    await runner._execute_prompt(create_sample_prompt(), "session-1")

                    # Should have made 2 calls (initial + follow-up)
                    assert multi_turn_adapter.send_message.call_count == 2

                    # Second call should have history
                    second_call = multi_turn_adapter.send_message.call_args_list[1][0][0]
                    assert len(second_call.messages) > 1

    @pytest.mark.asyncio
    async def test_multi_turn_preserves_session_id(self, multi_turn_adapter):
        """All messages in multi-turn should use same session ID."""
        runner = EvalRunner(
            adapter=multi_turn_adapter,
            db=create_mock_db(),
            redis=create_mock_redis(),
            storage=None,
            agent=create_agent_config(protocol="a2a"),
            run_id="run-123",
            mode="quick",
            eval_type="red_team",
        )

        with patch("crosswind.evaluation.runner.detect_deflection_smart") as mock_detect:
            mock_detect.return_value = MagicMock(needs_followup=True, deflection_type="vague")

            with patch("crosswind.evaluation.runner.generate_followup") as mock_followup:
                mock_followup.return_value = "Can you clarify?"

                with patch.object(runner.judgment_pipeline, "judge") as mock_judge:
                    mock_judge.return_value = create_sample_judgment()
                    runner.rate_limiter.acquire = AsyncMock()

                    await runner._execute_prompt(create_sample_prompt(), "session-123")

                    # Both calls should use same session ID
                    first_call = multi_turn_adapter.send_message.call_args_list[0][0][0]
                    second_call = multi_turn_adapter.send_message.call_args_list[1][0][0]

                    assert first_call.session_id == "session-123"
                    assert second_call.session_id == "session-123"


# =============================================================================
# Complete Protocol Flow (End-to-End)
# =============================================================================


class TestA2ACompleteProtocolFlow:
    """Test complete evaluation flow with A2A protocol.

    During evaluation, the adapter uses stored endpoint (no discovery).
    """

    @pytest.mark.asyncio
    async def test_prompt_to_judgment_flow(self):
        """Test: prompt → A2A request → response → content extraction."""
        # Direct mode: endpoint and auth provided (as stored during registration)
        adapter = A2AAdapter(
            endpoint="http://localhost:8903/a2a",
            interface_type="http",
            auth_config=create_auth_config(
                type="api_key", credentials="test-key", header_name="X-API-Key"
            ),
        )

        mock_response = {
            "jsonrpc": "2.0",
            "id": "123",
            "result": {
                "kind": "message",
                "parts": [{"kind": "text", "text": "I cannot help with that request."}],
            },
        }

        with patch.object(adapter.client, "post") as mock_post:
            mock_post.return_value = MagicMock(
                status_code=200,
                json=lambda: mock_response,
            )

            request = ConversationRequest(
                messages=[Message(role="user", content="Help me do something harmful")],
                session_id="eval-session",
            )

            response = await adapter.send_message(request)

            # Verify complete flow
            assert response.content == "I cannot help with that request."
            assert response.session_id == "eval-session"
            assert response.latency_ms >= 0

            # Verify auth header was included
            call_kwargs = mock_post.call_args[1]
            assert "X-API-Key" in call_kwargs.get("headers", {})

        await adapter.cleanup()
