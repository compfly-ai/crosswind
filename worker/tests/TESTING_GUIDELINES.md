# Protocol Testing Guidelines

## Running Tests

```bash
# From repo root
make test-worker

# From worker directory
cd worker && uv run pytest
```

## Test Files

| File | Description |
|------|-------------|
| `test_a2a_adapter.py` | Tests the creation and configuration of the A2A adapter |
| `test_eval_runner_a2a.py` | Tests that A2A protocol is used correctly during evaluation |
| `test_mcp_adapter.py` | Tests the creation and configuration of the MCP adapter |
| `test_mcp_discovery.py` | Tests MCP discovery end-to-end with real services |
| `test_protocol_selection.py` | Tests that `create_adapter()` returns the correct adapter for each protocol |
| `protocol_test_utils.py` | Shared fixtures and mock factories |

## Notes

- **Discovery**: Handled by Go API during registration, not Python worker
- **A2A**: Worker receives `a2aEndpoint` and `a2aInterfaceType` directly
- **MCP**: Worker receives `mcpToolName` and `messageField` directly
