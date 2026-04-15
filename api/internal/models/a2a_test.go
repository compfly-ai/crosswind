package models

import (
	"encoding/json"
	"testing"
)

// TestA2AAgentCard_UnmarshalSpecCompliant verifies we parse agent cards
// whose `security` and `securitySchemes` follow the A2A v0.3 spec shape:
//   - securitySchemes: map keyed by scheme name
//   - security: array of requirement sets (each a map of scheme name -> scopes)
//
// Regression: these fields were previously modeled with inverted shapes, which
// caused retrieve/parse to fail for any spec-compliant agent card.
func TestA2AAgentCard_UnmarshalSpecCompliant(t *testing.T) {
	// Representative payload emitted by a2a-sdk v0.3.x agents (e.g. supply-chain,
	// incident-response). Trimmed to the fields under test.
	raw := []byte(`{
		"name": "Supply Chain & Procurement Agent",
		"protocolVersion": "0.3.0",
		"security": [
			{"apiKey": []}
		],
		"securitySchemes": {
			"apiKey": {
				"type": "apiKey",
				"in": "header",
				"name": "X-API-Key",
				"description": "Shared-secret API key for client authentication."
			}
		}
	}`)

	var card A2AAgentCard
	if err := json.Unmarshal(raw, &card); err != nil {
		t.Fatalf("unmarshal spec-compliant agent card: %v", err)
	}

	if card.ProtocolVersion != "0.3.0" {
		t.Errorf("protocolVersion = %q, want %q", card.ProtocolVersion, "0.3.0")
	}

	if len(card.Security) != 1 {
		t.Fatalf("security: got %d requirement sets, want 1", len(card.Security))
	}
	scopes, ok := card.Security[0]["apiKey"]
	if !ok {
		t.Errorf("security[0] missing 'apiKey' key; got %v", card.Security[0])
	}
	if len(scopes) != 0 {
		t.Errorf("security[0][apiKey] = %v, want empty slice", scopes)
	}

	scheme, ok := card.SecuritySchemes["apiKey"]
	if !ok {
		t.Fatalf("securitySchemes missing 'apiKey' key; got %v", card.SecuritySchemes)
	}
	if scheme.Type != "apiKey" || scheme.In != "header" || scheme.Name != "X-API-Key" {
		t.Errorf("apiKey scheme mismatch: %+v", scheme)
	}
}

// TestA2AAgentCard_MultipleSchemes covers the multi-scheme case where a card
// declares more than one scheme and picks one via the `security` requirements.
func TestA2AAgentCard_MultipleSchemes(t *testing.T) {
	raw := []byte(`{
		"name": "Multi-Auth Agent",
		"protocolVersion": "0.3.0",
		"security": [
			{"bearerAuth": []},
			{"apiKey": []}
		],
		"securitySchemes": {
			"bearerAuth": {"type": "http", "scheme": "bearer"},
			"apiKey":     {"type": "apiKey", "in": "header", "name": "X-API-Key"}
		}
	}`)

	var card A2AAgentCard
	if err := json.Unmarshal(raw, &card); err != nil {
		t.Fatalf("unmarshal multi-scheme card: %v", err)
	}

	if got := len(card.Security); got != 2 {
		t.Errorf("len(Security) = %d, want 2", got)
	}
	if got := len(card.SecuritySchemes); got != 2 {
		t.Errorf("len(SecuritySchemes) = %d, want 2", got)
	}
	if _, ok := card.SecuritySchemes["bearerAuth"]; !ok {
		t.Errorf("missing bearerAuth scheme")
	}
}

// TestA2AAgentCard_OmitEmpty verifies the optional-empty round-trip doesn't
// emit null/empty fields that would confuse downstream consumers.
func TestA2AAgentCard_OmitEmpty(t *testing.T) {
	card := A2AAgentCard{
		Name:            "Minimal",
		ProtocolVersion: "0.3.0",
	}
	out, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Security fields are marked omitempty; they should not appear when unset.
	for _, field := range []string{`"security":`, `"securitySchemes":`} {
		if containsSubstring(string(out), field) {
			t.Errorf("marshaled output unexpectedly contains %s: %s", field, out)
		}
	}
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
