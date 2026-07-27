# CompFly conversation runtime contract v1

Status: **finalized for service implementation**  
Canonical schema: `conversation-runtime-v1.schema.json`

This document is normative for the Socket.IO, custom WebSocket, protobuf, JSON,
and starting-state onboarding work. If a service-specific type or comment
conflicts with this document, this document wins until the contract version is
changed.

## 1. Ownership and boundaries

The conversation-runtime slice sent from Platform Backend to Agent Eval contains
two independent objects. A complete agent registration also contains existing
identity, metadata, capabilities, and authentication fields outside this slice:

```json
{
  "endpointConfig": {},
  "conversationContract": {}
}
```

- `endpointConfig` answers **how to exchange a payload**. It owns the protocol,
  address, authentication reference, and transport-only options.
- `conversationContract` answers **how to encode a logical conversation and
  manage its state**. It must not contain a protocol discriminator, endpoint,
  namespace, event name, acknowledgement mode, or credentials.

Crosswind recognizes `custom_ws` and `socketio` as endpoint protocols and keeps
its existing logical conversation interfaces. Network adapters, schema
artifacts, saved profiles, and the composed runtime adapter remain in Agent Eval
Cloud.

Platform Backend is the public control-plane boundary. It stores agent endpoint
configuration and normalized contracts, owns agent-scoped state profiles, and
snapshots one selected profile into an evaluation request. Agent Eval is the
authoritative contract validator and owns schema compilation and runtime
encoding.

## 2. Composition model

The Agent Eval Cloud worker composes a runtime adapter rather than implementing
encoding and state independently in every transport:

```text
ContractDrivenAdapter
  ├── StateCoordinator
  ├── MessageCodec
  │     ├── StandardJSONCodec
  │     └── ProtobufCodec
  └── Transport
        ├── WebSocketTransport
        └── SocketIOTransport
```

The transport exchanges `str`, `bytes`, or a Socket.IO-native JSON value. It
does not inspect prompts, messages, protobuf descriptors, or state fields. The
codec converts between the logical conversation and the wire payload. The state
coordinator decides which logical state values apply to each turn.

The following combinations are required in v1:

| Protocol | JSON | Protobuf | State |
|---|---:|---:|---:|
| `custom_ws` | yes | yes | yes |
| `socketio` | yes | yes | yes |

HTTP is not added by this feature, but a future HTTP transport must be able to
reuse the same codec and state coordinator when its endpoint declares a
compatible conversation contract.

## 3. Endpoint configuration

### Raw WebSocket

```json
{
  "protocol": "custom_ws",
  "endpoint": "wss://agent.example/v1/ws"
}
```

The endpoint is the full WebSocket URL. A raw WebSocket endpoint has no
Socket.IO namespace, request event, acknowledgement, or response event.

### Socket.IO

```json
{
  "protocol": "socketio",
  "endpoint": "https://agent.example",
  "socketio": {
    "namespace": "/",
    "requestEvent": "message",
    "responseMode": "ack"
  }
}
```

Normalized Socket.IO configuration always contains `namespace`, `requestEvent`,
and `responseMode`. `responseMode=event` requires a non-empty
`responseEvent`. `responseMode=ack` forbids `responseEvent` so stale event
configuration cannot silently affect verification.

Authentication remains in the existing credential/authentication contract and
is intentionally absent from the conversation contract and upload manifest.

## 4. Conversation contract

The persisted field name is `conversationContract`. It replaces the unshipped
Socket.IO-shaped `stateContract` field. There is no migration or compatibility
alias.

```json
{
  "contractVersion": "1",
  "message": {},
  "state": {}
}
```

`state` is optional. An agent with no declared state fields omits it. State
profiles are stored separately and never embedded in this contract.

### JSON

```json
{
  "contractVersion": "1",
  "message": {
    "encoding": "json",
    "json": {
      "preset": "compfly_conversation_v1"
    }
  }
}
```

The `compfly_conversation_v1` request preset is:

```json
{
  "type": "message",
  "messages": [{ "role": "user", "content": "..." }],
  "session_id": "...",
  "state": {}
}
```

On a new runtime session, the codec includes `state` with the selected starting
values or `{}`. It must not send `{}` on an existing session as an accidental
reset. Subsequent delivery follows the state policy in section 5. A future
custom JSON mapping is a new preset or a later contract version; v1 does not
accept arbitrary JSON templates.

JSON onboarding requires no schema upload or inspection.

The same preset owns response decoding for both transports:

- A string response is assistant content.
- An object may expose assistant content through the first non-empty string in
  `content`, `text`, `message`, `response`, `reply`, or `output`.
- `session_id`, when present and non-empty, is the agent's logical session ID.
- A top-level object-valued `state`, when present, is returned to the state
  coordinator for `carry_forward` fields.
- `type=token|chunk` appends `content` or `text` to a streaming response.
- `type=message|response` is a complete response.
- `type=done|end` terminates a streamed response.
- `type=error` fails the turn using string `error` or `message`.

Transport code does not implement any of those rules. Raw WebSocket transport
may deliver multiple payloads; Socket.IO acknowledgement/event transport normally
delivers one. The shared codec/adapter applies the same decoding rules in both
cases. An object with neither recognizable content nor an error is a contract
error rather than an empty successful reply.

### Protobuf

```json
{
  "contractVersion": "1",
  "message": {
    "encoding": "protobuf",
    "protobuf": {
      "schemaArtifactId": "schema_abc123",
      "request": {
        "messageType": "example.v1.Frame",
        "promptField": "request.input.content"
      },
      "response": {
        "messageType": "example.v1.Frame",
        "contentField": "response.output.content",
        "sessionIdField": "response.state.session_id"
      },
      "stateMappings": {
        "request": {
          "location": "request.state.location"
        },
        "response": {
          "location": "response.state.location"
        }
      }
    }
  },
  "state": {
    "defaultDelivery": "carry_forward",
    "fields": [
      { "name": "location", "type": "string" }
    ]
  }
}
```

`schemaArtifactId` references a compiled, self-contained
`FileDescriptorSet` owned by Agent Eval. Persisted agent documents do not embed
base64 descriptors. The codec sends one serialized protobuf message as one
binary transport payload. Raw WebSocket uses the WebSocket message boundary;
v1 does not add a varint length prefix.

Request and response field paths are relative to their configured top-level
message. Every declared state field requires a request mapping for protobuf.
`carry_forward` additionally requires a response mapping for every declared
field. Response mappings may be omitted for `initial` and `every_turn`.

Optional `errorCodeField` and `errorMessageField` mappings let a protobuf
oneof/error response produce a useful verification error instead of an empty
assistant response.

## 5. Logical state

A state field has:

- stable `name`, matching `^[A-Za-z_][A-Za-z0-9_]*$`;
- type: `string`, `number`, `boolean`, or `json`;
- optional `label`, `description`, `default`, `required`, and `sensitive`.

Field names are unique within an agent. Defaults must validate against the
declared type. Unknown profile or verification values are rejected. Required
fields are enforced when a non-empty state value set is supplied; omitting a
profile remains valid.

Delivery is explicit. `state.defaultDelivery` applies unless a field supplies its
own `delivery` override:

- `initial`: selected starting values are sent on the first request only;
- `every_turn`: the selected starting values are sent on every request;
- `carry_forward`: starting values are sent first, mapped response state
  overwrites matching fields, and the merged result is sent on the next turn.

This permits one contract to carry a session counter forward while applying a
scenario variable such as `location` on every turn. For `carry_forward`, omitted
response fields preserve their previous values.
The runtime never converts an absent state input into a reset on an existing
session. Explicit state reset is out of scope for v1.

The logical request must preserve the distinction between:

- state not supplied;
- state supplied as an object;
- a later turn for which delivery policy says to omit state.

Cloud may use its existing `StatefulConversationRequest` wrapper to carry this
distinction; Crosswind does not need profile or schema-artifact types.

## 6. State profiles and evaluation jobs

Profiles remain agent-scoped named value sets. Public profile routes remain:

- `GET|POST /api/v1/agents/{slug}/state-profiles`
- `GET|PATCH|DELETE /api/v1/agents/{slug}/state-profiles/{profileId}`

An evaluation request accepts zero or one `stateProfileId`. Platform Backend:

1. resolves the selected profile under the same organization and agent;
2. validates its values against `conversationContract.state.fields`;
3. removes `stateProfileId` from the downstream request;
4. sends one trusted `stateProfile` snapshot containing `profileId`, `name`, and
   `values` to Agent Eval.

The worker performs no Platform Backend lookup. Raw profile values are not
logged and are not persisted in evaluation results. Results may contain only
the profile ID/name reference and non-sensitive contract metadata.

## 7. Contract and schema inspection

The public route is:

```text
POST /api/v1/agents/{id}/contract/inspect
```

The browser never calls Agent Eval directly. JSON agents do not call this
route. The multipart request may contain an optional `manifest` (`.json`,
`.yaml`, or `.yml`) and exactly one protobuf schema input:

- `schemaBundle`: one `.proto` or ZIP file;
- `schemaFiles`: repeated `.proto` files preserving paths relative to the
  selected folder;
- `descriptorSet`: one self-contained descriptor-set file.

A ZIP may include `agent-contract.json`, `agent-contract.yaml`, or
`agent-contract.yml` plus referenced protobuf files. The manifest contains only
`contractVersion`, `message`, and optional `state`; it cannot set endpoint,
protocol, authentication, organization, profile values, or credentials.

Inspection compiles and validates input without contacting the target agent or
an LLM. Its normalized response data is:

```json
{
  "schemaArtifact": {
    "artifactId": "schema_abc123",
    "kind": "protobuf_descriptor_set",
    "digest": "sha256:...",
    "fileNames": ["protocol/operator.proto"],
    "sizeBytes": 1234
  },
  "messages": [
    {
      "name": "compfly.operator.v1.Frame",
      "fields": [
        { "path": "request.input.content", "type": "string", "number": 2 }
      ]
    }
  ],
  "conversationContract": {}
}
```

`conversationContract` is returned only when a manifest supplied enough valid
mapping information to normalize it. Otherwise the UI builds the contract from
the returned inventory. Inspection does not save the agent contract or change
verification status.

Platform Backend wraps success in `{ "data": ..., "meta": ... }` and errors
in `{ "error": "<HTTP status>", "message": "...", "code": "..." }`.
Agent Eval may use its internal envelope, but Platform Backend must preserve its
stable error code and human-readable message.

## 8. Analyze and verify lifecycle

Create or update remains the ordinary lifecycle for both protocols:

```text
Create/PATCH agent
  -> persist endpointConfig + conversationContract
  -> synchronize to Agent Eval
  -> automatically call Agent Eval /analyze
  -> statically validate addressing and contract
  -> pending live verification
```

There is no Socket.IO create bypass and no protocol-specific onboarding route.
`/analyze` is internal and common. A JSON contract is validated without an
upload. A protobuf contract resolves its saved schema artifact and validates all
message and field mappings.

Live verification remains:

```text
POST /api/v1/agents/{slug}/verify
```

Request:

```json
{
  "environment": "dev",
  "prompt": "Hello",
  "stateValues": { "location": "SFO" }
}
```

`stateValues` is optional. Missing means no explicit starting values; an empty
object is an explicit empty starting set on the new verification session. The
same Agent Eval adapter factory and composed runtime used by evaluations perform
the live round trip. Verification returns decoded assistant content and marks
the agent ready only on success.

## 9. Validation invariants

All services and fixtures must enforce:

1. `conversationContract` contains no endpoint protocol or Socket.IO options.
2. `custom_ws` endpoint configuration contains no `socketio` object.
3. `socketio` endpoint configuration includes normalized event/response options.
4. Exactly one message encoding configuration matches `message.encoding`.
5. JSON uses the versioned preset and requires no artifact.
6. Protobuf references a saved artifact and supplies request/response mappings.
7. State field names are unique and mapping keys name declared fields only.
8. Every protobuf state field has a request path.
9. Every protobuf state field whose effective delivery is `carry_forward` has a
   response path.
10. Only one state profile may be selected per evaluation run.
11. Raw state values never appear in logs or persisted evaluation results.

## 10. Explicitly out of scope

- network adapters in Crosswind;
- organization-wide persona sets;
- permission maps or permission-aware scenarios;
- automatic state synthesis;
- multiple profiles per run;
- arbitrary JSON templates in v1;
- explicit mid-session state reset;
- schema formats other than protobuf in v1.
