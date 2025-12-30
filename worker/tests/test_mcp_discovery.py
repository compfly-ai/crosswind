"""End-to-end test for MCP protocol discovery and registration.

This test requires:
1. MongoDB running on localhost:27017
2. Redis running on localhost:6379 (or 6380)
3. The API server to be started separately

Usage:
    # Start the test MCP server
    uv run python tests/test_mcp_discovery.py --server

    # In another terminal, run the test
    uv run python tests/test_mcp_discovery.py --test
"""

import argparse
import json
import os
import subprocess
import sys
import time
from pathlib import Path

import httpx
from dotenv import load_dotenv

# Load environment variables from deploy/.env
_deploy_env = Path(__file__).parent.parent.parent / "deploy" / ".env"
load_dotenv(_deploy_env)


def start_mcp_server():
    """Start a test MCP server using FastMCP."""
    import uvicorn
    from mcp.server.fastmcp import FastMCP

    mcp = FastMCP("Test Support Agent")

    @mcp.tool()
    def chat(message: str) -> str:
        """Send a message to the customer support agent for help with orders, returns, and general inquiries."""
        return f"I'd be happy to help you with: {message}"

    @mcp.tool()
    def search_orders(query: str, limit: int = 10) -> str:
        """Search for customer orders by order ID, email, or product name."""
        return f"Found {limit} orders matching: {query}"

    print("Starting MCP test server on http://localhost:9000/mcp")
    app = mcp.streamable_http_app()
    uvicorn.run(app, host="0.0.0.0", port=9000)


def run_test(api_url: str = "http://localhost:8081", api_key: str | None = None):
    """Run the MCP registration test."""
    if api_key is None:
        api_key = os.environ.get("API_KEY")
        if not api_key:
            print("ERROR: API_KEY not found. Set it in deploy/.env or pass --api-key")
            sys.exit(1)

    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
    }

    agent_id = "test-mcp-agent"

    # 1. Delete any existing test agent
    print(f"Deleting existing agent '{agent_id}'...")
    with httpx.Client() as client:
        client.delete(f"{api_url}/v1/agents/{agent_id}", headers=headers)

    # 2. Register MCP agent
    print("Registering MCP agent...")
    payload = {
        "agentId": agent_id,
        "industry": "customer-support",
        "endpointConfig": {
            "protocol": "mcp",
            "endpoint": "http://localhost:9000/mcp",
            "mcpTransport": "streamable_http",
            "mcpToolName": "chat",
        },
        "authConfig": {"type": "none"},
    }

    with httpx.Client() as client:
        response = client.post(f"{api_url}/v1/agents", headers=headers, json=payload)

    if response.status_code != 201:
        print(f"FAILED: Registration returned {response.status_code}")
        print(response.text)
        sys.exit(1)

    agent = response.json()
    print(f"Agent created: {json.dumps(agent, indent=2)}")

    # 3. Verify auto-populated fields
    errors = []

    if not agent.get("name"):
        errors.append("name not auto-populated")
    elif agent["name"] != "chat":
        errors.append(f"name should be 'chat', got '{agent['name']}'")

    if not agent.get("description"):
        errors.append("description not auto-populated")
    elif "customer support" not in agent["description"].lower():
        errors.append("description doesn't contain expected content")

    if not agent.get("goal"):
        errors.append("goal not auto-populated")

    mcp_schema = agent.get("mcpToolSchema")
    if not mcp_schema:
        errors.append("mcpToolSchema not populated")
    else:
        if mcp_schema.get("messageField") != "message":
            errors.append(f"messageField should be 'message', got '{mcp_schema.get('messageField')}'")
        if mcp_schema.get("serverName") != "Test Support Agent":
            errors.append(f"serverName should be 'Test Support Agent', got '{mcp_schema.get('serverName')}'")
        if not mcp_schema.get("inputSchema"):
            errors.append("inputSchema not populated")

    capabilities = agent.get("declaredCapabilities")
    if not capabilities:
        errors.append("declaredCapabilities not populated")
    elif "chat" not in capabilities.get("tools", []):
        errors.append("chat tool not in declaredCapabilities.tools")

    # 4. Cleanup
    print(f"Cleaning up agent '{agent_id}'...")
    with httpx.Client() as client:
        client.delete(f"{api_url}/v1/agents/{agent_id}", headers=headers)

    # 5. Report results
    if errors:
        print("\n❌ TEST FAILED:")
        for error in errors:
            print(f"  - {error}")
        sys.exit(1)
    else:
        print("\n✅ TEST PASSED: MCP discovery auto-populated all fields correctly")


def main():
    parser = argparse.ArgumentParser(description="MCP Discovery E2E Test")
    parser.add_argument("--server", action="store_true", help="Start the test MCP server")
    parser.add_argument("--test", action="store_true", help="Run the registration test")
    parser.add_argument("--api-url", default="http://localhost:8081", help="API server URL")
    parser.add_argument("--api-key", default=None, help="API key (defaults to API_KEY from deploy/.env)")

    args = parser.parse_args()

    if args.server:
        start_mcp_server()
    elif args.test:
        run_test(args.api_url, args.api_key)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
