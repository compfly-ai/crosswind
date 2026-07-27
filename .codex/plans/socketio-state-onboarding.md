# Transport-neutral conversation and state onboarding plan

## Goal

Support raw WebSocket and Socket.IO agents with either JSON or protobuf message
encoding, plus optional user-declared starting state. Transport, encoding, and
state lifecycle are independent. JSON needs no upload. A run selects zero or one
saved agent-scoped state profile.

## Canonical contracts

The finalized v1 contract artifacts are:

- `../contracts/conversation-runtime-v1.schema.json`
- `../contracts/conversation-manifest-v1.schema.json`
- `../contracts/conversation-runtime-v1.md`
- `../contracts/fixtures/`
- `../contracts/validate_contracts.py`

Those artifacts are normative for downstream service implementation. The
persisted/shared field is `conversationContract`; it replaces the unshipped
Socket.IO-shaped `stateContract`. There is no compatibility alias or migration.

The normalized Agent Eval registration boundary is:

```json
{
  "endpointConfig": {
    "protocol": "custom_ws | socketio",
    "endpoint": "...",
    "socketio": {}
  },
  "conversationContract": {
    "contractVersion": "1",
    "message": {
      "encoding": "json | protobuf"
    },
    "state": {
      "defaultDelivery": "initial | every_turn | carry_forward",
      "fields": []
    }
  }
}
```

`endpointConfig` owns transport/addressing. `conversationContract` owns the
message codec, schema/mappings, and logical state lifecycle, and contains no
protocol or Socket.IO settings.

## Finalized architecture

### Crosswind

- Keep the completed `socketio` protocol constant and ordinary endpoint
  validation changes.
- Do not add or move network adapters, descriptors, profiles, or contract
  inspection into Crosswind.
- Use the Agent Eval Cloud `StatefulConversationRequest` wrapper to carry
  optional logical starting values without expanding Crosswind's control-plane
  responsibilities.

### Agent Eval Cloud worker

- Replace encoding/state logic embedded in `SocketIOAdapter` with one composed
  `ContractDrivenAdapter`.
- Extract transport-only `WebSocketTransport` and `SocketIOTransport` behavior.
- Add shared `StandardJSONCodec`, `ProtobufCodec`, and `StateCoordinator`.
- Use the existing protocol factory to compose transport + codec + state.
- Support the four required combinations:
  `custom_ws|socketio` x `json|protobuf`.
- Preserve one WebSocket message as one protobuf payload; do not add an
  unrequested length prefix.

### Agent Eval API

- Replace Socket.IO-specific state/inspection types and services with the
  finalized `conversationContract` types.
- Make `/analyze` statically validate either protocol against the same contract.
- Make contract inspection compile protobuf input and optional manifests without
  contacting the target or an LLM.
- Store compiled descriptor artifacts by ID; runtime contracts reference
  `schemaArtifactId` rather than inline base64.
- Use the same adapter factory for live verification and evaluations.

### Platform Backend

- Store and forward `conversationContract` verbatim; Agent Eval owns deep
  validation.
- Preserve normal native create/PATCH, synchronization, automatic `/analyze`,
  and `/verify` lifecycle for both protocols.
- Expose `POST /api/v1/agents/{id}/contract/inspect`; JSON does not call it.
- Keep agent-scoped profile routes and singular `stateProfileId` selection.
- Validate and snapshot the chosen profile using
  `conversationContract.state.fields`; never send a profile ID that belongs to
  another agent or organization.
- Do not log or persist raw state values in evaluation results.

### Frontend

- Replace `SocketIOContractForm` with one protocol-neutral endpoint contract
  editor shown for both `custom_ws` and `socketio`.
- Render transport selection/addressing separately from encoding.
- Default to the versioned JSON preset and require no upload.
- Show shared protobuf source/folder/descriptor/manifest inspection and mapping
  controls only for protobuf.
- Show Socket.IO namespace/event/response controls only for Socket.IO.
- Keep state in a separate optional UI section, with a default delivery policy
  and optional per-field overrides.
- Keep zero-or-one profile selection in evaluation UI.

## Work removed or replaced

- Socket.IO-only `stateContract` and hardcoded `transport: socketio` types.
- Protobuf encoding implemented inside `SocketIOAdapter` only.
- JSON payload construction duplicated across WebSocket and Socket.IO adapters.
- Inline base64 descriptor storage in agent contracts.
- Socket.IO-only protobuf inspection route/service names and protocol gates.
- Socket.IO-only verification dispatch.
- Cloud-only create bypass and separate onboarding lifecycle.
- Mandatory protobuf upload for JSON.
- Organization persona sets, permissions, automatic synthesis, and multiple
  profiles per run.

## Service checkpoints and sub-agent scopes

Only one service sub-agent runs at a time. Every sub-agent must preserve
unrelated user changes, modify only its assigned repository, run focused tests,
run that repository's complete relevant suite, run `git diff --check`, and report
files, tests, results, assumptions, risks, and unresolved issues. Work pauses for
review after each report.

1. **Agent Eval sub-agent**
   - Repository: `agent-eval` only.
   - Responsibility: implement the finalized contract, inspection/validation,
     composed Cloud runtime, factory selection, verification, and evaluation
     behavior.
   - Dependencies: canonical `.codex/contracts` artifacts; completed Crosswind
     protocol constants.
   - Required tests: Go contract/inspection/verification tests; Python codec,
     state coordinator, transport, factory, and four-combination tests; focused
     worker suite; relevant full Go/Python suites.
   - Pause for review.

2. **Platform Backend sub-agent**
   - Repository: `platform-backend` only.
   - Responsibility: implement storage/forwarding, generic inspection proxy,
     common analyze/verify lifecycle, and profile snapshot validation using the
     finalized contract.
   - Dependencies: approved Agent Eval checkpoint and its exact API behavior.
   - Required tests: model/payload, handler, lifecycle, tenant/profile,
     inspection error propagation, relevant full Go suite.
   - Pause for review.

3. **Frontend sub-agent**
   - Repository: `compfly-security-platform-socketio-state-onboarding` only.
   - Responsibility: implement the shared endpoint/encoding/state UX and update
     frontend API contracts to the approved Platform Backend behavior.
   - Dependencies: approved Agent Eval and Platform Backend checkpoints.
   - Required tests: contract helpers, component behavior, BFF proxy/error
     propagation, typecheck, lint for touched files, relevant Vitest suites.
   - Pause for review.

Crosswind receives no new feature implementation sub-agent because its approved
runtime scope is already complete. Its focused/full regression tests run during
integration and final testing.

## Integration sequence

After all service checkpoints are approved:

1. Rebuild/restart affected Terraform-local services in dependency order.
2. Verify Platform Backend -> Agent Eval API -> worker dependency health.
3. Test JSON Socket.IO onboarding with no upload, ack and response-event modes.
4. Test JSON raw WebSocket onboarding and regression behavior.
5. Test protobuf Socket.IO inspection, save, verification, and evaluation.
6. Test protobuf raw WebSocket against `examples/flyedge-operator`, including
   prompt mapping, decoded reply, error mapping, and mixed carry/every-turn state.
7. Test no profile and exactly one agent-scoped profile per run.
8. Confirm state values are absent from logs and persisted result projections.
9. Run complete relevant Crosswind, Agent Eval, Platform Backend, and frontend
   suites plus `git diff --check` in every repository.
10. Fix only failures directly caused by this feature and stop for final review.

## Definition of done

- Protocol, encoding, and state lifecycle are independent contract dimensions.
- No codec or state behavior is duplicated between WebSocket and Socket.IO.
- Both transports support JSON and protobuf through the same lifecycle.
- JSON works with no inspection or schema upload.
- Protobuf and optional manifests normalize to a saved schema artifact and one
  versioned conversation contract.
- State is optional, typed, agent-scoped, supports explicit delivery semantics,
  and uses at most one selected profile per run.
- Crosswind contains no network adapter or Cloud control-plane logic.
- Every service and final integration checkpoint is reviewed and approved.
