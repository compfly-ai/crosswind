# Socket.IO and reusable state onboarding plan

## Goal

Add Socket.IO as another selectable agent protocol while preserving the existing
`custom_ws` onboarding, synchronization, verification, and evaluation lifecycle.
Add optional, reusable starting state in a transport-independent form. JSON must
work without protobuf inspection; protobuf is an optional encoding of the same
logical request.

## Finalized architecture decisions

### Crosswind

- Add `socketio` only to the protocol constants and ordinary endpoint validation.
- Do not add or move WebSocket or Socket.IO network adapters into Crosswind.
- Treat `socketio` like `custom_ws`: it is a valid protocol and requires an endpoint.

### Agent Eval Cloud worker

- Keep `OpenAPIWebSocketAdapter` in its current Cloud-worker package.
- Keep `SocketIOAdapter` beside it.
- Select either through the existing protocol adapter factory.
- Run Socket.IO agents through the same create, synchronize, verify, and evaluate
  lifecycle as `custom_ws`.

### Default Socket.IO wire behavior

- Emit a configured request event with this JSON payload by default:

```json
{
  "type": "message",
  "messages": [{"role": "user", "content": "..."}],
  "session_id": "...",
  "state": {}
}
```

- Receive the response through either the Socket.IO acknowledgement or a
  configured response event.
- Apply a selected state profile only to the first scenario request. Omit the
  `state` field on later turns so `{}` cannot be interpreted as resetting the
  agent's session state.

### Optional state and protobuf

- Model state as transport-independent, typed user-defined fields.
- Save profiles under an agent, and allow exactly one saved profile to be selected
  for each evaluation run.
- Do not require a profile: omitted state is `{}`.
- Keep protobuf optional. JSON onboarding and verification do not inspect or upload
  a `.proto` file.
- When protobuf is selected, store an optional protobuf artifact/configuration and
  encode the same logical conversation request through the Socket.IO adapter.

## Contracts between systems

### Protocol configuration

The agent protocol remains the existing protocol discriminator. Socket.IO extends
the existing endpoint configuration with transport details rather than creating a
second onboarding API:

- `protocol`: `socketio`
- `endpoint`: required Socket.IO server URL
- `socketio.namespace`: optional; defaults to `/`
- `socketio.requestEvent`: required/configured event used to send prompts
- `socketio.responseMode`: `ack` or `event`
- `socketio.responseEvent`: required only for event mode
- `socketio.encoding`: `json` by default, optionally `protobuf`
- request/response field mappings where a non-default agent contract needs them
- optional protobuf artifact/message metadata only when encoding is `protobuf`

### State definition

Agent onboarding may declare a state schema. Each field has a stable name, label,
type, required flag, and optional description/default. The initial supported scalar
types are string, number, boolean, and JSON. Transport configuration may map the
logical state object to its encoded request location; for the default Socket.IO JSON
contract it is the top-level `state` field.

State schema definitions must not contain organization permission maps, automatic
field synthesis, scenario permissions, or generated sample/persona data.

### State profiles

Profiles are agent-scoped, named value sets validated against the agent's current
state schema:

- `GET /api/v1/agents/{slug}/state-profiles`
- `POST /api/v1/agents/{slug}/state-profiles`
- `GET /api/v1/agents/{slug}/state-profiles/{profileId}`
- `PATCH /api/v1/agents/{slug}/state-profiles/{profileId}`
- `DELETE /api/v1/agents/{slug}/state-profiles/{profileId}`

An evaluation request accepts one optional `stateProfileId` (singular). Platform
Backend validates that it belongs to the evaluated agent and snapshots the values
into the internal job. The worker must not need a second control-plane lookup, and
must not log raw state values. Persisted evaluation results expose only a
non-sensitive profile reference/name projection.

### Analyze and verify

- JSON analyze requests use the normal JSON endpoint and never require protobuf.
- Protobuf analyze requests use multipart upload only when encoding is protobuf.
- Verification uses the shared agent verification lifecycle and accepts
  `{environment, prompt, stateValues}`; state values are optional and default to
  `{}`.

## Work to remove from the current implementation

- The Cloud-only `createSocketIOAgent` bypass.
- Socket.IO-specific onboarding and verification lifecycle code.
- Mandatory protobuf upload or protobuf inspection for JSON agents.
- Organization-wide persona/state-profile sets.
- Permission maps and permission-aware scenarios.
- Automatic state synthesis.
- Multiple selected profiles per evaluation run.

Unrelated work bundled into the existing PRs must be preserved; removal is by
feature slice, not by wholesale PR revert.

## Implementation sequence and review gates

All repositories use branch `feature/socketio-state-onboarding`, created from the
latest `main`. Changes remain uncommitted unless the reviewer asks otherwise.

1. **Crosswind**
   - Files: `api/internal/models/agent.go`,
     `api/internal/services/agent_service.go`, and
     `api/internal/services/agent_validation_test.go`.
   - Add the protocol constant and normal endpoint validation/tests only.
   - Run focused service tests, the full Go test suite, and `git diff --check`.
   - Pause for review.

2. **Agent Eval**
   - Re-audit the latest `main` and Socket.IO PR changes before editing.
   - Keep both Cloud adapters together and register Socket.IO in the existing
     protocol factory.
   - Make the adapter implement the default JSON payload, configured request event,
     ack/event response modes, first-turn state, and optional protobuf encoding.
   - Delete the parallel Socket.IO lifecycle and route all operations through the
     existing `custom_ws` lifecycle services.
   - Add adapter/factory/lifecycle tests, then pause for review.

3. **Platform Backend**
   - Re-audit latest `main` and preserve unrelated changes from PR #196.
   - Remove `createSocketIOAgent` and use the normal agent create/sync/verify path.
   - Finalize protocol, state-schema, state-profile, analyze/verify, and singular
     evaluation-selection API contracts.
   - Remove organization persona APIs, permission behavior, synthesis, and
     multi-profile evaluation contracts.
   - Add model/service/handler tests, then pause for review.

4. **Frontend**
   - Re-audit latest `main` and preserve unrelated changes from PR #244.
   - Make Socket.IO an option in the existing agent onboarding flow.
   - Default to JSON; expose event/response settings and protobuf fields only when
     protobuf is selected.
   - Add optional state-field definition and agent-scoped profile management.
   - Allow zero or one profile per evaluation and remove persona/permission/
     synthesis/multi-profile UI.
   - Add component and client contract tests, then pause for review.

5. **Integration tests**
   - JSON Socket.IO onboarding without a `.proto` file.
   - Shared synchronization and verification lifecycle.
   - Ack and response-event modes.
   - No-state (`{}`) and one-profile first-turn state evaluation.
   - Optional protobuf Socket.IO path.
   - Regression coverage for `custom_ws` and other existing protocols.

## Definition of done

- Socket.IO is selected like `custom_ws`, not created through a special bypass.
- Crosswind contains no network adapter changes.
- JSON is the zero-configuration encoding and sends the documented payload.
- State is optional, common across onboarding transports, agent-scoped, and limited
  to one selected saved profile per run.
- Protobuf is optional and shares the same lifecycle.
- Removed persona/permission/synthesis/multi-profile behavior is absent from public
  and internal contracts.
- Each system passes its focused and full relevant tests at its review gate, and
  the final cross-system integration suite passes.
