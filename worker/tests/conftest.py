"""Pytest configuration and fixtures."""

import os

# Set required environment variables BEFORE importing crosswind modules
# This must happen before any crosswind imports to avoid validation errors
os.environ.setdefault(
    "ENCRYPTION_KEY",
    "0000000000000000000000000000000000000000000000000000000000000000"
)

import asyncio
import json
import subprocess
import sys
import time
from pathlib import Path

import pytest


@pytest.fixture(scope="session")
def a2a_test_server():
    """Start A2A test server for integration tests.

    Starts a minimal WebSocket-enabled A2A server on ports 8905 (HTTP) and 8906 (WS).
    The server runs for the entire test session.
    """
    # Create a minimal test server script
    server_code = '''
import asyncio
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
import threading
import websockets

AGENT_CARD = {
    "id": "test-agent",
    "name": "Test Agent",
    "description": "Test agent for integration tests",
    "version": "1.0.0",
    "protocolVersion": "0.2.0",
    "provider": {"name": "Test"},
    "capabilities": {"streaming": True},
    "skills": [],
    "interfaces": [
        {"type": "websocket", "url": "ws://localhost:8906/"},
        {"type": "http", "url": "http://localhost:8905/"},
    ],
}

def build_response(request_id, text):
    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "result": {
            "kind": "message",
            "role": "agent",
            "parts": [{"kind": "text", "text": f"Echo: {text}"}],
        }
    }

async def ws_handler(websocket):
    async for message in websocket:
        req = json.loads(message)
        parts = req.get("params", {}).get("message", {}).get("parts", [])
        text = parts[0].get("text", "") if parts else ""
        response = build_response(req.get("id"), text)
        await websocket.send(json.dumps(response))

class HTTPHandler(BaseHTTPRequestHandler):
    def log_message(self, *args): pass

    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(AGENT_CARD).encode())

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length))
        parts = body.get("params", {}).get("message", {}).get("parts", [])
        text = parts[0].get("text", "") if parts else ""
        response = build_response(body.get("id"), text)
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(response).encode())

def run_http():
    HTTPServer(("0.0.0.0", 8905), HTTPHandler).serve_forever()

async def main():
    threading.Thread(target=run_http, daemon=True).start()
    async with websockets.serve(ws_handler, "0.0.0.0", 8906):
        await asyncio.Future()

asyncio.run(main())
'''

    # Write server script to temp file
    server_file = Path(__file__).parent / "_test_server.py"
    server_file.write_text(server_code)

    # Start server process
    proc = subprocess.Popen(
        [sys.executable, str(server_file)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )

    # Wait for server to be ready
    time.sleep(1)

    # Check if server started successfully
    if proc.poll() is not None:
        stdout, stderr = proc.communicate()
        raise RuntimeError(f"Server failed to start: {stderr.decode()}")

    yield proc

    # Cleanup
    proc.terminate()
    proc.wait(timeout=5)
    server_file.unlink(missing_ok=True)


@pytest.fixture(scope="session")
def a2a_websocket_server():
    """Start A2A test server with WebSocket-only interface.

    Single smoke test fixture to verify real WebSocket communication works.
    """
    server_code = '''
import asyncio
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
import threading
import websockets

AGENT_CARD = {
    "id": "ws-agent",
    "name": "WebSocket Agent",
    "description": "Agent with WebSocket interface",
    "version": "1.0.0",
    "protocolVersion": "0.2.0",
    "provider": {"name": "Test"},
    "capabilities": {"streaming": True},
    "skills": [],
    "interfaces": [{"type": "websocket", "url": "ws://localhost:8908/"}],
}

async def ws_handler(websocket):
    async for message in websocket:
        req = json.loads(message)
        parts = req.get("params", {}).get("message", {}).get("parts", [])
        text = parts[0].get("text", "") if parts else ""
        response = {
            "jsonrpc": "2.0",
            "id": req.get("id"),
            "result": {
                "kind": "message",
                "parts": [{"kind": "text", "text": f"Echo: {text}"}],
            }
        }
        await websocket.send(json.dumps(response))

class HTTPHandler(BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(AGENT_CARD).encode())

def run_http():
    HTTPServer(("0.0.0.0", 8907), HTTPHandler).serve_forever()

async def main():
    threading.Thread(target=run_http, daemon=True).start()
    async with websockets.serve(ws_handler, "0.0.0.0", 8908):
        await asyncio.Future()

asyncio.run(main())
'''

    server_file = Path(__file__).parent / "_ws_server.py"
    server_file.write_text(server_code)

    proc = subprocess.Popen(
        [sys.executable, str(server_file)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )

    time.sleep(1)

    if proc.poll() is not None:
        stdout, stderr = proc.communicate()
        raise RuntimeError(f"WebSocket server failed to start: {stderr.decode()}")

    yield proc

    proc.terminate()
    proc.wait(timeout=5)
    server_file.unlink(missing_ok=True)

