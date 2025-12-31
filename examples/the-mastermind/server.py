#!/usr/bin/env python3
"""
The Mastermind - HTTP Agent for Crosswind Testing

Part of "The Crosswind Heist Crew" - a team of specialists for testing AI agent security.

The Mastermind is the cool, collected planner who always has an answer.
Every response includes a heist-related fun fact or piece of trivia.

To customize behavior, edit personality.py

Run: uv run python server.py
Test: curl http://localhost:8901/health
"""

import logging
import os
import random
import secrets
import uuid
from contextlib import asynccontextmanager

from dotenv import load_dotenv
from fastapi import Depends, FastAPI, HTTPException, status
from fastapi.security import APIKeyHeader
from pydantic import BaseModel, Field

# Import personality configuration
from personality import (
    SYSTEM_PROMPT,
    HEIST_FACTS,
    RESPONSES,
    HARMFUL_PATTERNS,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
logger = logging.getLogger("the-mastermind")

load_dotenv()

# Configuration
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "mock")
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")
GROQ_API_KEY = os.getenv("GROQ_API_KEY")
MODEL = os.getenv("MODEL", "gpt-4o-mini")
PORT = int(os.getenv("PORT", "8901"))
API_KEY = os.getenv("API_KEY", "mastermind-secret-key")

# In-memory session storage
sessions: dict[str, list[dict[str, str]]] = {}

# LLM client
llm_client = None

# Security
api_key_header = APIKeyHeader(name="X-API-Key", auto_error=False)


async def verify_api_key(api_key: str = Depends(api_key_header)) -> bool:
    """Verify API key authentication."""
    if not api_key or not secrets.compare_digest(api_key, API_KEY):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or missing API key",
        )
    return True


class Message(BaseModel):
    """A single message in a conversation."""

    role: str = Field(..., description="Role: 'user' or 'assistant'")
    content: str = Field(..., description="Message content")


class ChatRequest(BaseModel):
    """Request to chat with The Mastermind."""

    messages: list[Message] = Field(..., description="Conversation messages")
    session_id: str | None = Field(None, description="Optional session ID")


class ChatResponse(BaseModel):
    """Response from The Mastermind."""

    response: str = Field(..., description="The Mastermind's response")
    session_id: str = Field(..., description="Session ID for continuing conversation")


class HealthResponse(BaseModel):
    """Health check response."""

    status: str
    agent: str
    role: str
    provider: str


# --- Core Logic ---


def check_harmful_content(message: str) -> bool:
    """Check if message contains harmful requests."""
    lower_msg = message.lower()
    return any(pattern in lower_msg for pattern in HARMFUL_PATTERNS)


def get_refusal_response() -> str:
    """Return a character-appropriate refusal."""
    response = random.choice(RESPONSES["refusal"])
    fact = random.choice(HEIST_FACTS)
    return f"{response}\n\n*Fun fact: {fact}*"


def generate_mock_response(user_message: str) -> str:
    """Generate a themed response without calling an LLM."""
    lower_msg = user_message.lower()
    fact = random.choice(HEIST_FACTS)

    if any(g in lower_msg for g in ["hello", "hi", "hey", "greetings"]):
        response = random.choice(RESPONSES["greeting"])
    elif any(q in lower_msg for q in ["who are you", "what are you", "your name"]):
        response = random.choice(RESPONSES["identity"])
    elif any(q in lower_msg for q in ["help", "can you", "what can"]):
        response = random.choice(RESPONSES["help"])
    else:
        response = random.choice(RESPONSES["default"])

    return f"{response}\n\n*Fun fact: {fact}*"


async def call_llm(messages: list[dict]) -> str:
    """Call the configured LLM provider."""
    global llm_client

    if LLM_PROVIDER == "mock" or llm_client is None:
        user_msg = messages[-1]["content"] if messages else ""
        return generate_mock_response(user_msg)

    try:
        response = await llm_client.chat.completions.create(
            model=MODEL,
            messages=messages,
            max_tokens=500,
            temperature=0.7,
        )
        return response.choices[0].message.content
    except Exception as e:
        logger.error(f"LLM call failed: {e}")
        user_msg = messages[-1]["content"] if messages else ""
        return generate_mock_response(user_msg)


# --- FastAPI Application ---


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    global llm_client

    print("=" * 60)
    print("THE MASTERMIND - Crosswind Heist Crew")
    print("=" * 60)
    print(f"Port: {PORT}")
    print(f"Provider: {LLM_PROVIDER}")
    print(f"API Key: {API_KEY[:10]}...")
    print("=" * 60)

    if LLM_PROVIDER == "openai" and OPENAI_API_KEY:
        from openai import AsyncOpenAI

        llm_client = AsyncOpenAI(api_key=OPENAI_API_KEY)
        print("OpenAI client initialized")
    elif LLM_PROVIDER == "groq" and GROQ_API_KEY:
        from openai import AsyncOpenAI

        llm_client = AsyncOpenAI(
            api_key=GROQ_API_KEY, base_url="https://api.groq.com/openai/v1"
        )
        print("Groq client initialized")
    else:
        print("Running in mock mode (no LLM API calls)")

    yield
    print("The Mastermind is leaving the building...")


app = FastAPI(
    title="The Mastermind",
    description="HTTP Agent - Part of the Crosswind Heist Crew",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint (no auth required)."""
    return HealthResponse(
        status="healthy",
        agent="the-mastermind",
        role="HTTP Agent",
        provider=LLM_PROVIDER,
    )


@app.post("/chat", response_model=ChatResponse)
async def chat(request: ChatRequest, authenticated: bool = Depends(verify_api_key)):
    """Chat with The Mastermind."""
    session_id = request.session_id or str(uuid.uuid4())
    if session_id not in sessions:
        sessions[session_id] = []

    user_message = request.messages[-1].content if request.messages else ""

    if check_harmful_content(user_message):
        response_text = get_refusal_response()
    else:
        messages = [{"role": "system", "content": SYSTEM_PROMPT}]
        messages.extend(sessions[session_id])
        for msg in request.messages:
            messages.append({"role": msg.role, "content": msg.content})
        response_text = await call_llm(messages)

    # Update session
    for msg in request.messages:
        sessions[session_id].append({"role": msg.role, "content": msg.content})
    sessions[session_id].append({"role": "assistant", "content": response_text})

    if len(sessions[session_id]) > 20:
        sessions[session_id] = sessions[session_id][-20:]

    return ChatResponse(response=response_text, session_id=session_id)


@app.get("/")
async def root():
    """Root endpoint with agent info."""
    return {
        "agent": "The Mastermind",
        "crew": "Crosswind Heist Crew",
        "role": "HTTP Agent - The Planner",
        "description": "Cool, collected, always has a plan. And a fun fact.",
        "endpoints": {
            "/health": "Health check (no auth)",
            "/chat": "POST - Chat (requires X-API-Key header)",
        },
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT)
