# Protocol Testing Guidelines

This document defines the testing standards for protocol adapters in crosswind.

## Two Sides of Testing

### Registration Side
**Question: Can we create and configure this protocol adapter?**

| Test Category | What to Test |
|---------------|--------------|
| Config Validation | Required fields present, invalid values rejected |
| Adapter Creation | `create_adapter()` returns correct type |
| Discovery | A2A: AgentCard fetch, MCP: tool schema discovery |
| Auth Setup | Auth headers built correctly |
| Endpoint Parsing | URL parsing, base URL extraction |

### Evaluation Side
**Question: Is the protocol being leveraged correctly during eval?**

| Test Category | What to Test |
|---------------|--------------|
| Message Sending | `send_message()` called with correct request format |
| Response Parsing | Response extracted from protocol-specific format |
| Session Management | Sessions created, reused, closed correctly |
| Error Handling | Timeouts, connection errors, malformed responses |
| Multi-turn | History included in follow-up requests |

**Note:** Evaluation tests verify protocol usage, NOT judgment accuracy. Judgment is tested separately.

---

## Test Structure

```
tests/
├── TESTING_GUIDELINES.md
├── protocol_test_utils.py
│
├── # Registration tests
├── test_a2a_adapter.py           # A2A adapter + registration
├── test_protocol_selection.py    # create_adapter() routing
│
├── # Evaluation tests
├── test_eval_runner_a2a.py       # A2A protocol usage in eval
└── test_eval_runner_http.py      # HTTP protocol usage in eval
```

---

## Required Tests Per Protocol

### Registration Tests

```python
class TestMyProtocolRegistration:
    # Config validation
    def test_valid_config_creates_adapter(self): ...
    def test_missing_required_field_raises(self): ...

    # Auth
    def test_bearer_auth_headers(self): ...
    def test_api_key_auth_headers(self): ...

    # Discovery (if applicable)
    def test_discovery_success(self): ...
    def test_discovery_failure_handled(self): ...
```

### Evaluation Tests

```python
class TestMyProtocolEvaluation:
    # Protocol usage
    def test_send_message_called(self): ...
    def test_request_format_correct(self): ...
    def test_response_parsed_correctly(self): ...

    # Session management
    def test_session_created(self): ...
    def test_session_reused(self): ...

    # Error handling
    def test_connection_error_handled(self): ...
    def test_timeout_handled(self): ...
```

---

## Shared Utilities

Use `protocol_test_utils.py`:

```python
from tests.protocol_test_utils import (
    # Data factories
    create_sample_prompt,
    create_sample_request,
    create_sample_response,
    create_agent_config,
    create_auth_config,

    # Mock fixtures
    create_mock_db,
    create_mock_redis,
    create_mock_adapter,
)
```

---

## Current Coverage

| Protocol | Registration | Evaluation |
|----------|-------------|------------|
| A2A | ✅ 46 tests | ✅ 6 tests |
| HTTP/Custom | ✅ 5 tests | ✅ 6 tests |
| MCP | ❌ Pending | ❌ Pending |

---

## Adding a New Protocol

1. **Registration tests** (`test_{protocol}_adapter.py`)
   - Config validation
   - Auth header building
   - Discovery (if applicable)

2. **Selection tests** (add to `test_protocol_selection.py`)
   - `create_adapter()` returns correct type

3. **Evaluation tests** (`test_eval_runner_{protocol}.py`)
   - Protocol used correctly during eval
   - Request/response format
   - Error handling

4. **Update utilities** (if needed)
   - Add protocol to `create_agent_config()`

---

## Running Tests

```bash
# All protocol tests
uv run pytest tests/test_*adapter*.py tests/test_protocol_selection.py tests/test_eval_runner_*.py -v

# Registration only
uv run pytest tests/test_*adapter*.py tests/test_protocol_selection.py -v

# Evaluation only
uv run pytest tests/test_eval_runner_*.py -v

# Specific protocol
uv run pytest tests/test_a2a_adapter.py tests/test_eval_runner_a2a.py -v
```
