#!/usr/bin/env python3
"""
The Inside Man - A2A Agent for Crosswind Testing

Part of "The Crosswind Heist Crew" - a team of specialists for testing AI agent security.

The Inside Man is the smooth-talking liaison who handles communication between crews.
Speaks in mysterious, noir-style dialogue.

To customize behavior, edit personality.py

Implements Google's A2A (Agent-to-Agent) protocol:
- Agent card at /.well-known/agent.json
- JSON-RPC 2.0 messaging at /a2a

Run: uv run python server.py
Agent Card: http://localhost:8903/.well-known/agent.json
"""

import logging
import os
import random
import secrets
from contextlib import asynccontextmanager
from typing import Any
from uuid import uuid4

from dotenv import load_dotenv
from fastapi import Depends, FastAPI, HTTPException, status
from fastapi.security import APIKeyHeader
from pydantic import BaseModel, Field

# Import personality configuration
from personality import (
    AGENT_METADATA,
    NOIR_FACTS,
    RESPONSES,
    HARMFUL_PATTERNS,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
logger = logging.getLogger("the-inside-man")

load_dotenv()

# Configuration
PORT = int(os.getenv("PORT", "8903"))
AGENT_API_KEY = os.getenv("AGENT_API_KEY", "inside-man-secret-key")
HOST = os.getenv("HOST", "localhost")

# In-memory session storage
sessions: dict[str, list[dict[str, str]]] = {}

# Security
api_key_header = APIKeyHeader(name="X-API-Key", auto_error=False)


async def verify_api_key(api_key: str = Depends(api_key_header)) -> bool:
    """Verify API key (optional for A2A - some deployments skip auth)."""
    if AGENT_API_KEY and api_key:
        if not secrets.compare_digest(api_key, AGENT_API_KEY):
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Invalid API key",
            )
    return True


# --- Core Logic ---


def check_harmful(text: str) -> bool:
    """Check if text contains harmful patterns."""
    lower_text = text.lower()
    return any(pattern in lower_text for pattern in HARMFUL_PATTERNS)


def get_response(message: str) -> str:
    """Generate a noir-style response."""
    lower_msg = message.lower()
    fact = random.choice(NOIR_FACTS)

    if check_harmful(message):
        response = random.choice(RESPONSES["refusal"])
        return f"{response}\n\n*Fun fact: {fact}*"

    if any(g in lower_msg for g in ["hello", "hi", "hey", "greetings"]):
        response = random.choice(RESPONSES["greeting"])
    elif any(q in lower_msg for q in ["who are you", "what are you", "your name"]):
        response = random.choice(RESPONSES["identity"])
    elif any(q in lower_msg for q in ["help", "can you", "what can"]):
        response = random.choice(RESPONSES["help"])
    else:
        response = random.choice(RESPONSES["default"])

    return f"{response}\n\n*Fun fact: {fact}*"


# --- A2A Models ---


class JSONRPCRequest(BaseModel):
    """JSON-RPC 2.0 Request."""

    jsonrpc: str = Field(default="2.0")
    id: str | int
    method: str
    params: dict[str, Any] = Field(default_factory=dict)


class JSONRPCResponse(BaseModel):
    """JSON-RPC 2.0 Response."""

    jsonrpc: str = "2.0"
    id: str | int
    result: dict[str, Any] | None = None
    error: dict[str, Any] | None = None


class TextPart(BaseModel):
    """A2A text content part."""

    type: str = "text"
    text: str


class Message(BaseModel):
    """A2A Message."""

    role: str
    parts: list[TextPart]


class Task(BaseModel):
    """A2A Task response."""

    id: str
    contextId: str
    status: dict[str, Any]
    messages: list[Message] = Field(default_factory=list)
    artifacts: list[Any] = Field(default_factory=list)


# --- Agent Card ---


def get_agent_card() -> dict[str, Any]:
    """Return the A2A agent card."""
    base_url = f"http://{HOST}:{PORT}"
    return {
        "id": AGENT_METADATA["id"],
        "name": AGENT_METADATA["name"],
        "description": AGENT_METADATA["description"],
        "version": AGENT_METADATA["version"],
        "protocolVersion": AGENT_METADATA["protocol_version"],
        "provider": AGENT_METADATA["provider"],
        "capabilities": {
            "streaming": False,
            "pushNotifications": False,
        },
        "skills": AGENT_METADATA["skills"],
        "interfaces": [
            {
                "type": "http",
                "url": f"{base_url}/a2a",
            }
        ],
        "url": f"{base_url}/a2a",
    }


# --- FastAPI Application ---


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    print("=" * 60)
    print("THE INSIDE MAN - Crosswind Heist Crew")
    print("=" * 60)
    print(f"Port: {PORT}")
    print(f"Agent Card: http://{HOST}:{PORT}/.well-known/agent.json")
    print(f"A2A Endpoint: http://{HOST}:{PORT}/a2a")
    print("=" * 60)
    yield
    print("*The Inside Man vanishes into the shadows*")


app = FastAPI(
    title="The Inside Man",
    description="A2A Agent - Part of the Crosswind Heist Crew",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {
        "status": "healthy",
        "agent": "the-inside-man",
        "role": "A2A Agent",
    }


@app.get("/.well-known/agent.json")
async def agent_card():
    """A2A Agent Card endpoint."""
    return get_agent_card()


@app.post("/a2a")
async def a2a_endpoint(
    request: JSONRPCRequest,
    authenticated: bool = Depends(verify_api_key),
) -> JSONRPCResponse:
    """A2A JSON-RPC endpoint."""
    logger.info(f"A2A request: {request.method}")

    if request.method == "message/send":
        return handle_message_send(request)
    elif request.method == "tasks/get":
        return handle_tasks_get(request)
    else:
        return JSONRPCResponse(
            id=request.id,
            error={
                "code": -32601,
                "message": f"Method not found: {request.method}",
            },
        )


def handle_message_send(request: JSONRPCRequest) -> JSONRPCResponse:
    """Handle message/send A2A method."""
    params = request.params

    # Extract message content
    message = params.get("message", {})
    parts = message.get("parts", [])
    text_content = ""
    for part in parts:
        if part.get("type") == "text":
            text_content += part.get("text", "")

    # Get or create context (session)
    context_id = params.get("contextId") or str(uuid4())
    if context_id not in sessions:
        sessions[context_id] = []

    # Generate response
    response_text = get_response(text_content)

    # Store in session
    sessions[context_id].append({"role": "user", "content": text_content})
    sessions[context_id].append({"role": "assistant", "content": response_text})

    # Build A2A response
    task_id = str(uuid4())
    task = {
        "id": task_id,
        "contextId": context_id,
        "status": {"state": "completed"},
        "messages": [
            {
                "role": "assistant",
                "parts": [{"type": "text", "text": response_text}],
            }
        ],
        "artifacts": [],
    }

    return JSONRPCResponse(id=request.id, result=task)


def handle_tasks_get(request: JSONRPCRequest) -> JSONRPCResponse:
    """Handle tasks/get A2A method."""
    task_id = request.params.get("taskId", "")

    # For simplicity, return a completed status
    return JSONRPCResponse(
        id=request.id,
        result={
            "id": task_id,
            "status": {"state": "completed"},
        },
    )


@app.get("/")
async def root():
    """Root endpoint with agent info."""
    return {
        "agent": "The Inside Man",
        "crew": "Crosswind Heist Crew",
        "role": "A2A Agent - The Liaison",
        "description": "Mysterious, noir-style messenger between agents.",
        "endpoints": {
            "/health": "Health check",
            "/.well-known/agent.json": "A2A Agent Card",
            "/a2a": "POST - A2A JSON-RPC endpoint",
        },
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT)
