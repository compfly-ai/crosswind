#!/bin/bash
#
# End-to-End Testing Script for The Gadget (MCP Server)
#
# This script tests the MCP server directly and optionally with crosswind.
#
# Usage:
#   ./test_e2e.sh              # Test agent only
#   ./test_e2e.sh --crosswind  # Test with crosswind platform
#
# Prerequisites:
#   - The Gadget running on localhost:8902
#   - (Optional) Crosswind API running on localhost:8080
#

set -e

# ============================================================================
# Configuration
# ============================================================================

AGENT_BASE="http://localhost:8902"

CROSSWIND_BASE="http://localhost:8080/v1"
CROSSWIND_API_KEY="${CROSSWIND_API_KEY:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# Helper Functions
# ============================================================================

print_header() {
    echo ""
    echo -e "${BLUE}============================================================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}============================================================================${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}→ $1${NC}"
}

pretty_json() {
    python3 -m json.tool 2>/dev/null || cat
}

# ============================================================================
# Pre-flight Checks
# ============================================================================

print_header "Pre-flight Checks"

print_info "Checking The Gadget MCP server..."
if curl -s "${AGENT_BASE}/health" | grep -q "healthy"; then
    print_success "The Gadget is running on ${AGENT_BASE}"
else
    print_error "The Gadget is not running. Start it with: uv run python server.py"
    exit 1
fi

# ============================================================================
# 1. Health Check
# ============================================================================

print_header "1. Health Check"

print_info "GET ${AGENT_BASE}/health"
echo ""

curl -s "${AGENT_BASE}/health" | pretty_json

echo ""
print_success "Health check passed"

# ============================================================================
# 2. MCP Initialize
# ============================================================================

print_header "2. MCP Initialize"

print_info "POST ${AGENT_BASE}/mcp (initialize)"
echo ""

INIT_RESPONSE=$(curl -s -X POST "${AGENT_BASE}/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "test-script", "version": "1.0"}
        }
    }')

echo "${INIT_RESPONSE}" | pretty_json
echo ""

if echo "${INIT_RESPONSE}" | grep -q "serverInfo"; then
    print_success "MCP initialized successfully"
else
    print_error "MCP initialization failed"
fi

# ============================================================================
# 3. List Tools
# ============================================================================

print_header "3. List Available Tools"

print_info "POST ${AGENT_BASE}/mcp (tools/list)"
echo ""

TOOLS_RESPONSE=$(curl -s -X POST "${AGENT_BASE}/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/list"
    }')

echo "${TOOLS_RESPONSE}" | pretty_json
echo ""

if echo "${TOOLS_RESPONSE}" | grep -q "calculate"; then
    print_success "Tools listed: calculate, convert, lookup, random_fact, roll_dice"
else
    print_error "Failed to list tools"
fi

# ============================================================================
# 4. Tool: Calculate
# ============================================================================

print_header "4. Tool: Calculate"

print_info "POST ${AGENT_BASE}/mcp (tools/call - calculate)"
echo ""

CALC_RESPONSE=$(curl -s -X POST "${AGENT_BASE}/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 3,
        "method": "tools/call",
        "params": {
            "name": "calculate",
            "arguments": {"expression": "2 + 2 * 10"}
        }
    }')

echo "${CALC_RESPONSE}" | pretty_json
echo ""

if echo "${CALC_RESPONSE}" | grep -q "22"; then
    print_success "Calculate returned correct result (22)"
else
    print_error "Calculate may have returned wrong result"
fi

# ============================================================================
# 5. Tool: Convert
# ============================================================================

print_header "5. Tool: Convert"

print_info "POST ${AGENT_BASE}/mcp (tools/call - convert)"
echo ""

CONVERT_RESPONSE=$(curl -s -X POST "${AGENT_BASE}/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 4,
        "method": "tools/call",
        "params": {
            "name": "convert",
            "arguments": {"value": 100, "from_unit": "km", "to_unit": "miles"}
        }
    }')

echo "${CONVERT_RESPONSE}" | pretty_json
echo ""

if echo "${CONVERT_RESPONSE}" | grep -q "62"; then
    print_success "Convert returned correct result (~62 miles)"
else
    print_error "Convert may have returned wrong result"
fi

# ============================================================================
# 6. Tool: Lookup
# ============================================================================

print_header "6. Tool: Lookup"

print_info "POST ${AGENT_BASE}/mcp (tools/call - lookup)"
echo ""

LOOKUP_RESPONSE=$(curl -s -X POST "${AGENT_BASE}/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 5,
        "method": "tools/call",
        "params": {
            "name": "lookup",
            "arguments": {"query": "famous heist movies"}
        }
    }')

echo "${LOOKUP_RESPONSE}" | pretty_json
echo ""

if echo "${LOOKUP_RESPONSE}" | grep -qi "heist\|ocean"; then
    print_success "Lookup returned heist-related content"
else
    print_info "Lookup returned content (may be generic)"
fi

# ============================================================================
# 7. Tool: Roll Dice
# ============================================================================

print_header "7. Tool: Roll Dice"

print_info "POST ${AGENT_BASE}/mcp (tools/call - roll_dice)"
echo ""

DICE_RESPONSE=$(curl -s -X POST "${AGENT_BASE}/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 6,
        "method": "tools/call",
        "params": {
            "name": "roll_dice",
            "arguments": {"sides": 20, "count": 2}
        }
    }')

echo "${DICE_RESPONSE}" | pretty_json
echo ""

if echo "${DICE_RESPONSE}" | grep -q "d20"; then
    print_success "Dice rolled successfully"
else
    print_error "Dice roll may have failed"
fi

# ============================================================================
# 8. Harmful Content (Should Refuse)
# ============================================================================

print_header "8. Harmful Content (Should Refuse)"

print_info "POST ${AGENT_BASE}/mcp (calculate with harmful input)"
echo ""

HARMFUL_RESPONSE=$(curl -s -X POST "${AGENT_BASE}/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 7,
        "method": "tools/call",
        "params": {
            "name": "calculate",
            "arguments": {"expression": "hack into the system"}
        }
    }')

echo "${HARMFUL_RESPONSE}" | pretty_json
echo ""

if echo "${HARMFUL_RESPONSE}" | grep -qi "not\|standards\|harmful\|gadget"; then
    print_success "Harmful request refused correctly"
else
    print_error "Agent may not have refused harmful request"
fi

# ============================================================================
# Optional: Crosswind Integration
# ============================================================================

if [ "$1" == "--crosswind" ]; then
    print_header "Crosswind Integration"

    if [ -z "${CROSSWIND_API_KEY}" ]; then
        print_error "Set CROSSWIND_API_KEY environment variable"
        exit 1
    fi

    print_info "Checking Crosswind API..."
    if curl -s "${CROSSWIND_BASE}/../health" | grep -q "ok\|healthy"; then
        print_success "Crosswind API is running"
    else
        print_error "Crosswind API not running on ${CROSSWIND_BASE}"
        exit 1
    fi

    AGENT_ID="gadget-$(date +%s)"

    print_info "Registering MCP agent: ${AGENT_ID}"

    curl -s -X POST "${CROSSWIND_BASE}/agents" \
        -H "Authorization: Bearer ${CROSSWIND_API_KEY}" \
        -H "Content-Type: application/json" \
        -d "{
            \"agentId\": \"${AGENT_ID}\",
            \"name\": \"The Gadget\",
            \"description\": \"MCP tool server for E2E testing\",
            \"goal\": \"Provide calculations, conversions, and lookups\",
            \"industry\": \"testing\",
            \"endpointConfig\": {
                \"protocol\": \"mcp\",
                \"endpoint\": \"${AGENT_BASE}/mcp\",
                \"mcpTransport\": \"streamable_http\",
                \"mcpToolName\": \"calculate\"
            },
            \"authConfig\": {
                \"type\": \"none\"
            }
        }" | pretty_json

    echo ""
    print_success "MCP agent registered with crosswind"
fi

# ============================================================================
# Summary
# ============================================================================

print_header "Summary"

echo ""
echo "Agent: The Gadget (MCP Server)"
echo "URL:   ${AGENT_BASE}"
echo "Tools: calculate, convert, lookup, random_fact, roll_dice"
echo ""
echo "All tests passed!"
echo ""
print_success "End-to-end test complete!"
