package scenario

import "fmt"

// Capabilities describe the actual adapter, not the connection purpose or language.
// False means unsupported; no feature is inferred from reverse WS availability.
type Capabilities struct {
	AdaptiveTurns     bool   `json:"adaptiveTurns"`
	StateRestore      bool   `json:"stateRestore"`
	HistoryRestore    bool   `json:"historyRestore"`
	CheckpointRestore bool   `json:"checkpointRestore"`
	EventStimulus     bool   `json:"eventStimulus"`
	Hooks             []Hook `json:"hooks"`
}
type Hook struct {
	Component string `json:"component"`
	Phase     string `json:"phase"`
	Action    string `json:"action"`
}

// Preflight reports unsupported features before any session preparation or dispatch.
// It does not perform authorization, calibration or registry resolution; callers
// must complete those checks independently before execution.
func Preflight(d map[string]any, c Capabilities) []Issue {
	issues := []Issue{}
	add := func(path, feature string) {
		issues = append(issues, Issue{"unsupported_capability", path, "adapter does not support " + feature})
	}
	if setup, ok := d["setup"].(map[string]any); ok {
		for _, f := range []struct {
			k  string
			ok bool
		}{{"state", c.StateRestore}, {"history", c.HistoryRestore}, {"checkpoint", c.CheckpointRestore}} {
			if _, present := setup[f.k]; present && !f.ok {
				add("/setup/"+f.k, f.k+" restoration")
			}
		}
	}
	for i, x := range d["steps"].([]any) {
		s := x.(map[string]any)
		if s["adaptive"] != nil && !c.AdaptiveTurns {
			add(fmt.Sprintf("/steps/%d/adaptive", i), "platform-generated adaptive turns")
		}
		if s["type"] == "event" && !c.EventStimulus {
			add(fmt.Sprintf("/steps/%d", i), "external events")
		}
		if s["type"] == "set_state" && !c.StateRestore {
			add(fmt.Sprintf("/steps/%d", i), "state restoration")
		}
	}
	rules, _ := d["overrides"].([]any)
	for i, x := range rules {
		o := x.(map[string]any)
		if o["enabled"] == false {
			continue
		}
		m := o["match"].(map[string]any)
		a := o["action"].(map[string]any)
		supported := false
		for _, h := range c.Hooks {
			if h.Component == m["component"] && h.Phase == m["phase"] && h.Action == a["type"] {
				supported = true
				break
			}
		}
		if !supported {
			add(fmt.Sprintf("/overrides/%d", i), "requested interception hook")
		}
	}
	return issues
}
