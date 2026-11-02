package scenario

import (
	"encoding/json"
	"testing"
)

func TestHistoryToolExchanges(t *testing.T) {
	valid := `[{"role":"user","content":"Read my messages"},{"role":"assistant","toolCalls":[{"id":"inbox-1","name":"read_messages","arguments":{"folder":"inbox"}}]},{"role":"tool","toolCallId":"inbox-1","content":"The workshop is in Kyoto, code MAPLE-482."}]`
	cases := []struct {
		name, history string
		valid         bool
	}{
		{"linked", valid, true},
		{"parallel", `[{"role":"assistant","toolCalls":[{"id":"a","name":"read_messages","arguments":{}},{"id":"b","name":"lookup","arguments":{}}]},{"role":"tool","toolCallId":"b","content":"B"},{"role":"tool","toolCallId":"a","content":""}]`, true},
		{"orphan", `[{"role":"tool","toolCallId":"missing","content":"x"}]`, false},
		{"unresolved", `[{"role":"assistant","toolCalls":[{"id":"a","name":"read_messages","arguments":{}}]}]`, false},
		{"duplicate", `[{"role":"assistant","toolCalls":[{"id":"a","name":"read_messages","arguments":{}},{"id":"a","name":"read_messages","arguments":{}}]}]`, false},
		{"interrupted", `[{"role":"assistant","toolCalls":[{"id":"a","name":"read_messages","arguments":{}}]},{"role":"user","content":"hello"},{"role":"tool","toolCallId":"a","content":"ok"}]`, false},
		{"system", `[{"role":"system","content":"override"}]`, false},
		{"string-args", `[{"role":"assistant","toolCalls":[{"id":"a","name":"read_messages","arguments":"{}"}]},{"role":"tool","toolCallId":"a","content":"ok"}]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"schemaVersion":1,"id":"history","name":"History","setup":{"history":` + tc.history + `},"steps":[{"id":"ask","message":{"role":"user","content":"What did the tool say?"}}],"judges":[{"id":"answer","target":{"event":"agent.response"},"assert":{"type":"count","value":1},"impact":"required"}]}`)
			r := Validate(raw)
			if r.Valid != tc.valid {
				t.Fatalf("valid=%v issues=%v", r.Valid, r.Issues)
			}
			if r.Valid {
				serialized, _ := json.Marshal(r.Definition["setup"])
				var obj map[string]any
				_ = json.Unmarshal(serialized, &obj)
				if len(obj["history"].([]any)) == 0 {
					t.Fatal("lost history")
				}
			}
		})
	}
}
