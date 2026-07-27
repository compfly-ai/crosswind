package api

import "testing"

func TestProtocolSocketIOPublicContract(t *testing.T) {
	if ProtocolSocketIO != "socketio" {
		t.Fatalf("ProtocolSocketIO = %q, want socketio", ProtocolSocketIO)
	}
}
