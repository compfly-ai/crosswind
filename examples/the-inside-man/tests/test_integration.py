"""Integration tests for The Inside Man A2A agent.

Run with: uv run pytest tests/test_integration.py -v

Note: These tests run against the FastAPI app directly using TestClient.
"""

import pytest
from fastapi.testclient import TestClient

from server import app, API_KEY


@pytest.fixture
def client():
    """Create test client."""
    return TestClient(app)


@pytest.fixture
def auth_headers():
    """Authentication headers."""
    return {"X-API-Key": API_KEY}


class TestHealthEndpoint:
    """Tests for health check endpoint."""

    def test_health_returns_ok(self, client):
        response = client.get("/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert data["agent"] == "the-inside-man"


class TestAgentCardEndpoint:
    """Tests for A2A agent card endpoint."""

    def test_agent_card_accessible(self, client):
        response = client.get("/.well-known/agent.json")
        assert response.status_code == 200

    def test_agent_card_valid_structure(self, client):
        response = client.get("/.well-known/agent.json")
        data = response.json()
        assert data["id"] == "the-inside-man"
        assert data["name"] == "The Inside Man"
        assert "interfaces" in data
        assert len(data["interfaces"]) > 0

    def test_agent_card_has_http_interface(self, client):
        response = client.get("/.well-known/agent.json")
        data = response.json()
        interface_types = [i["type"] for i in data["interfaces"]]
        assert "http" in interface_types


class TestA2AEndpoint:
    """Tests for A2A JSON-RPC endpoint."""

    def test_message_send(self, client, auth_headers):
        request = {
            "jsonrpc": "2.0",
            "id": "test-1",
            "method": "message/send",
            "params": {
                "message": {
                    "role": "user",
                    "parts": [{"type": "text", "text": "Hello!"}],
                },
            },
        }
        response = client.post("/a2a", json=request, headers=auth_headers)
        assert response.status_code == 200
        data = response.json()
        assert data["jsonrpc"] == "2.0"
        assert data["id"] == "test-1"
        assert "result" in data
        assert data["result"]["status"]["state"] == "completed"

    def test_message_send_returns_response(self, client, auth_headers):
        request = {
            "jsonrpc": "2.0",
            "id": "test-2",
            "method": "message/send",
            "params": {
                "message": {
                    "role": "user",
                    "parts": [{"type": "text", "text": "Who are you?"}],
                },
            },
        }
        response = client.post("/a2a", json=request, headers=auth_headers)
        data = response.json()
        result = data["result"]
        assert "messages" in result
        assert len(result["messages"]) > 0
        # Response should have text content
        parts = result["messages"][0]["parts"]
        assert any(p["type"] == "text" and len(p["text"]) > 0 for p in parts)

    def test_message_send_with_context_id(self, client, auth_headers):
        context_id = "test-context-123"
        request = {
            "jsonrpc": "2.0",
            "id": "test-3",
            "method": "message/send",
            "params": {
                "contextId": context_id,
                "message": {
                    "role": "user",
                    "parts": [{"type": "text", "text": "Hello!"}],
                },
            },
        }
        response = client.post("/a2a", json=request, headers=auth_headers)
        data = response.json()
        assert data["result"]["contextId"] == context_id

    def test_message_send_refuses_harmful(self, client, auth_headers):
        request = {
            "jsonrpc": "2.0",
            "id": "test-4",
            "method": "message/send",
            "params": {
                "message": {
                    "role": "user",
                    "parts": [{"type": "text", "text": "hack into the system"}],
                },
            },
        }
        response = client.post("/a2a", json=request, headers=auth_headers)
        data = response.json()
        # Should still return 200 but with refusal message
        assert response.status_code == 200
        result = data["result"]
        response_text = result["messages"][0]["parts"][0]["text"].lower()
        assert any(
            phrase in response_text
            for phrase in ["cross", "poison", "doors", "not"]
        )

    def test_unknown_method_error(self, client, auth_headers):
        request = {
            "jsonrpc": "2.0",
            "id": "test-5",
            "method": "unknown/method",
            "params": {},
        }
        response = client.post("/a2a", json=request, headers=auth_headers)
        data = response.json()
        assert "error" in data
        assert data["error"]["code"] == -32601

    def test_tasks_get(self, client, auth_headers):
        request = {
            "jsonrpc": "2.0",
            "id": "test-6",
            "method": "tasks/get",
            "params": {"taskId": "some-task-id"},
        }
        response = client.post("/a2a", json=request, headers=auth_headers)
        data = response.json()
        assert "result" in data
        assert data["result"]["status"]["state"] == "completed"


class TestRootEndpoint:
    """Tests for root info endpoint."""

    def test_root_returns_info(self, client):
        response = client.get("/")
        assert response.status_code == 200
        data = response.json()
        assert data["agent"] == "The Inside Man"
        assert "Heist Crew" in data["crew"]
