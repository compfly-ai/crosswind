package scenario

import (
	"bytes"
	"encoding/json"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func fixture(t *testing.T) map[string]any {
	t.Helper()
	b, e := os.ReadFile("testdata/mock-send-email.json")
	if e != nil {
		t.Fatal(e)
	}
	r := Validate(b)
	if !r.Valid {
		t.Fatal(r.Issues)
	}
	return r.Definition
}
func TestAuthoringRoundTrip(t *testing.T) {
	d := fixture(t)
	j, _ := json.Marshal(d)
	y, _ := yaml.Marshal(d)
	a, b := Validate(j), Validate(y)
	if !a.Valid || !b.Valid || a.Digest != b.Digest {
		t.Fatalf("round trip changed meaning: %+v %+v", a, b)
	}
	if bytes.Contains(y, []byte("bindings:")) {
		t.Fatal("legacy binding leaked")
	}
}
func TestStrictCodec(t *testing.T) {
	for _, raw := range []string{"id: a\nid: b", "x: &a [1]\ny: *a", "x: .nan", "true: value", "x: !secret value", "x: 1\n---\nx: 2"} {
		if _, e := Decode([]byte(raw)); e == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
func TestContractRejectsLossyInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"legacy", func(d map[string]any) { d["bindings"] = []any{} }},
		{"contradictory impact", func(d map[string]any) { d["judges"].([]any)[0].(map[string]any)["required"] = true }},
		{"unknown step", func(d map[string]any) {
			d["overrides"].([]any)[0].(map[string]any)["match"].(map[string]any)["step"] = "missing"
		}},
		{"wrong phase", func(d map[string]any) {
			d["overrides"].([]any)[0].(map[string]any)["match"].(map[string]any)["phase"] = "after"
		}},
		{"duplicate step", func(d map[string]any) { d["steps"] = append(d["steps"].([]any), d["steps"].([]any)[0]) }},
		{"null state", func(d map[string]any) { d["setup"] = map[string]any{"state": nil} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := fixture(t)
			tc.edit(d)
			b, _ := json.Marshal(d)
			if Validate(b).Valid {
				t.Fatal("accepted invalid definition")
			}
		})
	}
}
func TestEmptyStateIsNotAbsent(t *testing.T) {
	d := fixture(t)
	b, _ := json.Marshal(d)
	a := Validate(b)
	d["setup"] = map[string]any{"state": map[string]any{}}
	b, _ = json.Marshal(d)
	c := Validate(b)
	if !c.Valid || a.Digest == c.Digest {
		t.Fatal("empty state must remain explicit")
	}
}
func TestCapabilityPreflightNeverDowngradesStateToPrompt(t *testing.T) {
	d := fixture(t)
	c := Capabilities{Hooks: []Hook{{"tool", "before", "return"}}}
	if issues := Preflight(d, c); len(issues) != 0 {
		t.Fatal(issues)
	}
	d["setup"] = map[string]any{"state": map[string]any{}}
	if issues := Preflight(d, c); len(issues) != 1 || issues[0].Path != "/setup/state" {
		t.Fatal(issues)
	}
	delete(d, "setup")
	d["overrides"].([]any)[0].(map[string]any)["enabled"] = false
	if issues := Preflight(d, Capabilities{}); len(issues) != 0 {
		t.Fatal(issues)
	}
}

func TestPolicyDecisionSelector(t *testing.T) {
	d := fixture(t)
	j := d["judges"].([]any)[0].(map[string]any)
	j["target"] = map[string]any{"event": "policy.decision", "component": "tool", "name": "read_messages", "stage": "tool_call_response"}
	j["assert"] = map[string]any{"type": "equals", "path": "/outcome", "value": map[string]any{"action": "deny", "reason": "inbox_injection"}}
	raw, _ := json.Marshal(d)
	if v := Validate(raw); !v.Valid {
		t.Fatal(v.Issues)
	}
	j["target"].(map[string]any)["stage"] = "made_up_stage"
	raw, _ = json.Marshal(d)
	if v := Validate(raw); v.Valid {
		t.Fatal("accepted unknown policy stage")
	}
}
