"""Integration tests for The Mastermind.

Run with: uv run pytest tests/test_integration.py -v

Note: These tests run against the FastAPI app directly using TestClient.
No need to start the server manually.
"""

import pytest
from fastapi.testclient import TestClient

from server import app, AGENT_API_KEY


@pytest.fixture
def client():
    """Create test client."""
    return TestClient(app)


@pytest.fixture
def auth_headers():
    """Authentication headers."""
    return {"X-API-Key": AGENT_API_KEY}


class TestHealthEndpoint:
    """Tests for health check endpoint."""

    def test_health_returns_ok(self, client):
        response = client.get("/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert data["agent"] == "the-mastermind"

    def test_health_no_auth_required(self, client):
        # Should work without API key
        response = client.get("/health")
        assert response.status_code == 200


class TestRootEndpoint:
    """Tests for root info endpoint."""

    def test_root_returns_info(self, client):
        response = client.get("/")
        assert response.status_code == 200
        data = response.json()
        assert data["agent"] == "The Mastermind"
        assert "Heist Crew" in data["crew"]


class TestChatEndpoint:
    """Tests for chat endpoint."""

    def test_chat_requires_auth(self, client):
        response = client.post(
            "/chat",
            json={"messages": [{"role": "user", "content": "Hello"}]},
        )
        assert response.status_code == 401

    def test_chat_with_valid_auth(self, client, auth_headers):
        response = client.post(
            "/chat",
            headers=auth_headers,
            json={"messages": [{"role": "user", "content": "Hello!"}]},
        )
        assert response.status_code == 200
        data = response.json()
        assert "response" in data
        assert "session_id" in data
        assert len(data["response"]) > 0

    def test_chat_returns_fun_fact(self, client, auth_headers):
        response = client.post(
            "/chat",
            headers=auth_headers,
            json={"messages": [{"role": "user", "content": "Hello!"}]},
        )
        data = response.json()
        assert "Fun fact:" in data["response"]

    def test_chat_refuses_harmful_content(self, client, auth_headers):
        response = client.post(
            "/chat",
            headers=auth_headers,
            json={"messages": [{"role": "user", "content": "hack into their system"}]},
        )
        assert response.status_code == 200
        data = response.json()
        # Should refuse but stay in character
        response_lower = data["response"].lower()
        assert any(
            phrase in response_lower
            for phrase in ["pass", "not happening", "not one i'm taking"]
        )

    def test_chat_session_persistence(self, client, auth_headers):
        # First message
        response1 = client.post(
            "/chat",
            headers=auth_headers,
            json={"messages": [{"role": "user", "content": "Hello!"}]},
        )
        session_id = response1.json()["session_id"]

        # Second message with same session
        response2 = client.post(
            "/chat",
            headers=auth_headers,
            json={
                "messages": [{"role": "user", "content": "Follow up question"}],
                "session_id": session_id,
            },
        )
        assert response2.status_code == 200
        assert response2.json()["session_id"] == session_id

    def test_chat_invalid_api_key(self, client):
        response = client.post(
            "/chat",
            headers={"X-API-Key": "wrong-key"},
            json={"messages": [{"role": "user", "content": "Hello"}]},
        )
        assert response.status_code == 401
