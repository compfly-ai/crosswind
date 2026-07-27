#!/usr/bin/env python3
"""Dependency-free semantic validation for the v1 contract fixtures."""

from __future__ import annotations

import json
import re
from copy import deepcopy
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parent
FIXTURES = ROOT / "fixtures"
RUNTIME_FIXTURES = (
    "custom-ws-json.json",
    "socketio-json.json",
    "custom-ws-protobuf.json",
    "socketio-protobuf.json",
)
DELIVERIES = {"initial", "every_turn", "carry_forward"}
FIELD_TYPES = {"string", "number", "boolean", "json"}
FIELD_NAME = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def load_json(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as handle:
        value = json.load(handle)
    assert isinstance(value, dict), f"{path.name}: root must be an object"
    return value


def assert_keys(value: dict[str, Any], allowed: set[str], where: str) -> None:
    unexpected = set(value) - allowed
    assert not unexpected, f"{where}: unexpected keys {sorted(unexpected)}"


def validate_default(field: dict[str, Any], where: str) -> None:
    if "default" not in field:
        return
    value = field["default"]
    expected = field["type"]
    valid = {
        "string": isinstance(value, str),
        "number": isinstance(value, (int, float)) and not isinstance(value, bool),
        "boolean": isinstance(value, bool),
        "json": True,
    }[expected]
    assert valid, f"{where}: default does not match type {expected}"


def validate_state(state: dict[str, Any] | None, protobuf: dict[str, Any] | None, where: str) -> None:
    if state is None:
        if protobuf:
            assert not protobuf.get("stateMappings"), f"{where}: mappings require declared state"
        return

    assert_keys(state, {"defaultDelivery", "fields"}, f"{where}.state")
    default_delivery = state.get("defaultDelivery")
    assert default_delivery in DELIVERIES, f"{where}: invalid default delivery"
    fields = state.get("fields")
    assert isinstance(fields, list), f"{where}: state.fields must be an array"

    names: list[str] = []
    carry_names: set[str] = set()
    for index, field in enumerate(fields):
        field_where = f"{where}.state.fields[{index}]"
        assert isinstance(field, dict), f"{field_where}: must be an object"
        assert_keys(
            field,
            {"name", "label", "type", "required", "description", "default", "sensitive", "delivery"},
            field_where,
        )
        name = field.get("name")
        assert isinstance(name, str) and FIELD_NAME.fullmatch(name), f"{field_where}: invalid name"
        assert field.get("type") in FIELD_TYPES, f"{field_where}: invalid type"
        if "delivery" in field:
            assert field["delivery"] in DELIVERIES, f"{field_where}: invalid delivery"
        if field.get("delivery", default_delivery) == "carry_forward":
            carry_names.add(name)
        validate_default(field, field_where)
        names.append(name)
    assert len(names) == len(set(names)), f"{where}: state field names must be unique"

    if protobuf is None:
        return
    mappings = protobuf.get("stateMappings") or {}
    request = mappings.get("request") or {}
    response = mappings.get("response") or {}
    declared = set(names)
    assert set(request) == declared, f"{where}: protobuf request mappings must match declared state fields"
    assert set(response) <= declared, f"{where}: protobuf response mappings contain undeclared fields"
    missing_carry = carry_names - set(response)
    assert not missing_carry, f"{where}: carry-forward fields lack response mappings {sorted(missing_carry)}"
    for direction, values in (("request", request), ("response", response)):
        assert all(isinstance(path, str) and path for path in values.values()), (
            f"{where}: {direction} state mapping paths must be non-empty strings"
        )


def validate_conversation(contract: dict[str, Any], where: str, *, manifest: bool = False) -> None:
    assert_keys(contract, {"contractVersion", "message", "state"}, where)
    assert contract.get("contractVersion") == "1", f"{where}: contractVersion must be 1"
    message = contract.get("message")
    assert isinstance(message, dict), f"{where}: message is required"
    assert_keys(message, {"encoding", "json", "protobuf"}, f"{where}.message")
    encoding = message.get("encoding")
    assert encoding in {"json", "protobuf"}, f"{where}: invalid encoding"

    protobuf: dict[str, Any] | None = None
    if encoding == "json":
        assert "protobuf" not in message, f"{where}: JSON cannot include protobuf configuration"
        assert message.get("json") == {"preset": "compfly_conversation_v1"}, (
            f"{where}: JSON must use compfly_conversation_v1"
        )
    else:
        assert "json" not in message, f"{where}: protobuf cannot include JSON configuration"
        protobuf = message.get("protobuf")
        assert isinstance(protobuf, dict), f"{where}: protobuf configuration is required"
        allowed = {"request", "response", "stateMappings"}
        if not manifest:
            allowed.add("schemaArtifactId")
            assert isinstance(protobuf.get("schemaArtifactId"), str) and protobuf["schemaArtifactId"], (
                f"{where}: normalized protobuf contract requires schemaArtifactId"
            )
        else:
            assert "schemaArtifactId" not in protobuf, f"{where}: upload manifest cannot set artifact IDs"
        assert_keys(protobuf, allowed, f"{where}.message.protobuf")
        request = protobuf.get("request")
        response = protobuf.get("response")
        assert isinstance(request, dict), f"{where}: protobuf request mapping is required"
        assert isinstance(response, dict), f"{where}: protobuf response mapping is required"
        assert set(request) == {"messageType", "promptField"}, f"{where}: invalid request mapping"
        assert {"messageType", "contentField"} <= set(response), f"{where}: invalid response mapping"
        assert set(response) <= {
            "messageType", "contentField", "sessionIdField", "errorCodeField", "errorMessageField"
        }, f"{where}: unexpected response mapping"
        assert all(isinstance(value, str) and value for value in request.values()), (
            f"{where}: request mappings must be non-empty strings"
        )
        assert all(isinstance(value, str) and value for value in response.values()), (
            f"{where}: response mappings must be non-empty strings"
        )

    validate_state(contract.get("state"), protobuf, where)


def validate_runtime(runtime: dict[str, Any], where: str) -> tuple[str, str]:
    assert_keys(runtime, {"endpointConfig", "conversationContract"}, where)
    endpoint = runtime.get("endpointConfig")
    assert isinstance(endpoint, dict), f"{where}: endpointConfig is required"
    assert_keys(endpoint, {"protocol", "endpoint", "socketio"}, f"{where}.endpointConfig")
    protocol = endpoint.get("protocol")
    assert protocol in {"custom_ws", "socketio"}, f"{where}: invalid protocol"
    assert isinstance(endpoint.get("endpoint"), str) and endpoint["endpoint"], f"{where}: endpoint is required"

    if protocol == "custom_ws":
        assert "socketio" not in endpoint, f"{where}: raw WebSocket cannot contain Socket.IO settings"
    else:
        socketio = endpoint.get("socketio")
        assert isinstance(socketio, dict), f"{where}: Socket.IO settings are required"
        assert_keys(socketio, {"namespace", "requestEvent", "responseMode", "responseEvent"}, f"{where}.socketio")
        assert all(isinstance(socketio.get(key), str) and socketio[key] for key in ("namespace", "requestEvent", "responseMode")), (
            f"{where}: normalized Socket.IO settings must be non-empty"
        )
        assert socketio["responseMode"] in {"ack", "event"}, f"{where}: invalid response mode"
        if socketio["responseMode"] == "event":
            assert isinstance(socketio.get("responseEvent"), str) and socketio["responseEvent"], (
                f"{where}: event response mode requires responseEvent"
            )
        else:
            assert "responseEvent" not in socketio, f"{where}: ack mode forbids responseEvent"

    contract = runtime.get("conversationContract")
    assert isinstance(contract, dict), f"{where}: conversationContract is required"
    forbidden = {"protocol", "endpoint", "socketio", "namespace", "requestEvent", "responseMode", "responseEvent", "credentials"}
    assert not (forbidden & set(contract)), f"{where}: conversation contract contains transport settings"
    validate_conversation(contract, f"{where}.conversationContract")
    return protocol, contract["message"]["encoding"]


def assert_rejected(name: str, callback) -> None:
    try:
        callback()
    except AssertionError:
        return
    raise AssertionError(f"negative case was accepted: {name}")


def resolve_pointer(document: Any, pointer: str, where: str) -> None:
    current = document
    if not pointer:
        return
    assert pointer.startswith("/"), f"{where}: unsupported JSON pointer {pointer!r}"
    for raw_segment in pointer[1:].split("/"):
        segment = raw_segment.replace("~1", "/").replace("~0", "~")
        assert isinstance(current, dict) and segment in current, f"{where}: unresolved JSON pointer {pointer!r}"
        current = current[segment]


def validate_schema_refs(schema: dict[str, Any], schema_path: Path) -> None:
    def walk(value: Any, where: str) -> None:
        if isinstance(value, dict):
            ref = value.get("$ref")
            if isinstance(ref, str):
                target_name, separator, fragment = ref.partition("#")
                target_path = schema_path if not target_name else schema_path.parent / target_name
                assert target_path.is_file(), f"{where}: missing schema reference {target_name!r}"
                target = schema if target_path == schema_path else load_json(target_path)
                if separator:
                    resolve_pointer(target, fragment, where)
            for key, child in value.items():
                walk(child, f"{where}.{key}")
        elif isinstance(value, list):
            for index, child in enumerate(value):
                walk(child, f"{where}[{index}]")

    walk(schema, schema_path.name)


def main() -> None:
    runtime_schema = load_json(ROOT / "conversation-runtime-v1.schema.json")
    manifest_schema = load_json(ROOT / "conversation-manifest-v1.schema.json")
    assert runtime_schema.get("$id") == "https://compfly.ai/contracts/conversation-runtime-v1.schema.json"
    assert manifest_schema.get("$id") == "https://compfly.ai/contracts/conversation-manifest-v1.schema.json"
    validate_schema_refs(runtime_schema, ROOT / "conversation-runtime-v1.schema.json")
    validate_schema_refs(manifest_schema, ROOT / "conversation-manifest-v1.schema.json")

    matrix: set[tuple[str, str]] = set()
    runtimes: dict[str, dict[str, Any]] = {}
    for name in RUNTIME_FIXTURES:
        runtime = load_json(FIXTURES / name)
        matrix.add(validate_runtime(runtime, name))
        runtimes[name] = runtime
    expected = {
        ("custom_ws", "json"),
        ("custom_ws", "protobuf"),
        ("socketio", "json"),
        ("socketio", "protobuf"),
    }
    assert matrix == expected, f"runtime fixture matrix incomplete: {sorted(expected - matrix)}"

    assert runtimes["custom-ws-json.json"]["conversationContract"] == (
        runtimes["socketio-json.json"]["conversationContract"]
    ), "JSON conversation contract must be transport-independent"

    manifest = load_json(FIXTURES / "flyedge-operator.contract.json")
    validate_conversation(manifest, "flyedge-operator.contract.json", manifest=True)
    normalized = json.loads(json.dumps(manifest))
    normalized["message"]["protobuf"]["schemaArtifactId"] = "schema_flyedge_operator_v1"
    assert normalized == runtimes["custom-ws-protobuf.json"]["conversationContract"], (
        "manifest plus inspected artifact must normalize to the runtime contract fixture"
    )

    coupled_transport = deepcopy(runtimes["custom-ws-json.json"])
    coupled_transport["conversationContract"]["protocol"] = "custom_ws"
    assert_rejected("protocol inside conversationContract", lambda: validate_runtime(coupled_transport, "negative"))

    websocket_with_socketio = deepcopy(runtimes["custom-ws-json.json"])
    websocket_with_socketio["endpointConfig"]["socketio"] = {
        "namespace": "/", "requestEvent": "message", "responseMode": "ack"
    }
    assert_rejected("Socket.IO settings on raw WebSocket", lambda: validate_runtime(websocket_with_socketio, "negative"))

    duplicate_state = deepcopy(runtimes["socketio-json.json"])
    duplicate_state["conversationContract"]["state"]["fields"].append(
        deepcopy(duplicate_state["conversationContract"]["state"]["fields"][0])
    )
    assert_rejected("duplicate state fields", lambda: validate_runtime(duplicate_state, "negative"))

    missing_request_mapping = deepcopy(runtimes["custom-ws-protobuf.json"])
    del missing_request_mapping["conversationContract"]["message"]["protobuf"]["stateMappings"]["request"]["location"]
    assert_rejected("missing protobuf state request mapping", lambda: validate_runtime(missing_request_mapping, "negative"))

    missing_carry_mapping = deepcopy(runtimes["custom-ws-protobuf.json"])
    del missing_carry_mapping["conversationContract"]["message"]["protobuf"]["stateMappings"]["response"]["turn_number"]
    assert_rejected("missing carry-forward response mapping", lambda: validate_runtime(missing_carry_mapping, "negative"))

    manifest_with_artifact = deepcopy(manifest)
    manifest_with_artifact["message"]["protobuf"]["schemaArtifactId"] = "caller-controlled"
    assert_rejected(
        "manifest sets schema artifact ID",
        lambda: validate_conversation(manifest_with_artifact, "negative", manifest=True),
    )

    inspection = load_json(FIXTURES / "contract-inspection-response.json")
    assert set(inspection) == {"schemaArtifact", "messages"}
    artifact = inspection["schemaArtifact"]
    assert artifact["kind"] == "protobuf_descriptor_set"
    assert isinstance(artifact["sizeBytes"], int) and artifact["sizeBytes"] > 0
    assert isinstance(inspection["messages"], list) and inspection["messages"]

    profile = load_json(FIXTURES / "state-profile-snapshot.json")
    assert set(profile) == {"profileId", "name", "values"}
    assert all(profile[key] for key in ("profileId", "name")) and isinstance(profile["values"], dict)

    verify = load_json(FIXTURES / "verify-request.json")
    assert set(verify) <= {"environment", "prompt", "stateValues"}
    assert verify["environment"] in {"dev", "staging", "prod"} and verify["prompt"].strip()
    assert isinstance(verify.get("stateValues"), dict)

    print("validated 2 schemas, 1 manifest, 4 runtime fixtures, and 3 API fixtures")
    print("matrix: custom_ws/socketio x json/protobuf")
    print("rejected 6 coupled or invalid contract cases")


if __name__ == "__main__":
    main()
