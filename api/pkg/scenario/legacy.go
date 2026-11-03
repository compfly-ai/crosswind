package scenario

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Migration is a preview, never a write. Source and evaluator snapshots remain
// outside the definition and are needed for reviewed publication and lineage.
type Migration struct {
	Status             string         `json:"status"`
	SourceRevision     string         `json:"sourceRevision"`
	Definition         map[string]any `json:"definition,omitempty"`
	EvaluatorSnapshots map[string]any `json:"evaluatorSnapshots"`
	Issues             []Issue        `json:"issues"`
	Notices            []string       `json:"notices"`
}
type MigrationOptions struct {
	ToolCallEvent string `json:"toolCallEvent"`
}

func list(v any) []any            { a, _ := v.([]any); return a }
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func str(v any) string            { s, _ := v.(string); return s }

// ConvertLibrary converts the supported legacy subset, returning review_required
// for ambiguity instead of dropping behavior. The caller chooses the evidence
// meaning of legacy tool_called; there is no silent requested/executed default.
func ConvertLibrary(source map[string]any, opts MigrationOptions) Migration {
	out := Migration{Status: "review_required", SourceRevision: str(source["revision"]), Issues: []Issue{}, Notices: []string{}, EvaluatorSnapshots: map[string]any{}}
	issue := func(path, msg string) { out.Issues = append(out.Issues, Issue{"migration_review_required", path, msg}) }
	if source["promptRedacted"] == true || source["split"] == "holdout" {
		issue("", "protected scenarios require authorized server-side conversion")
		return out
	}
	if len(list(source["excludedJudgeIds"])) > 0 {
		issue("/excludedJudgeIds", "resolve excluded judges before conversion")
	}
	content := object(source["content"])
	id := str(source["id"])
	name := str(source["title"])
	if name == "" {
		name = id
	}
	d := map[string]any{"schemaVersion": 1, "id": id, "name": name}
	attrs := map[string]any{}
	if tags := list(source["tags"]); len(tags) > 0 {
		attrs["tags"] = tags
	}
	if len(attrs) > 0 {
		d["attributes"] = attrs
	}
	steps := []any{}
	turns := list(content["turns"])
	if len(turns) == 0 {
		if prompt := str(content["prompt"]); prompt != "" {
			turns = []any{map[string]any{"role": "user", "content": prompt}}
		}
	}
	for i, t := range turns {
		turn := object(t)
		if turn["role"] != "user" {
			issue(fmt.Sprintf("/content/turns/%d", i), "recorded agent replies require explicit history conversion")
			continue
		}
		steps = append(steps, map[string]any{"id": fmt.Sprintf("turn-%d", i+1), "message": turn})
	}
	d["steps"] = steps
	if _, ok := content["state"]; ok {
		issue("/content/state", "declare restoration adapter; legacy prompt context cannot silently become restored state")
	}
	if len(list(content["judges"])) > 0 {
		issue("/content/judges", "resolve exact registry judge versions")
	}
	if len(list(content["turnChecks"])) > 0 {
		issue("/content/turnChecks", "turn-scoped conversion is not supported by this preview")
	}
	if str(content["expectedResponse"]) != "" {
		issue("/content/expectedResponse", "preserve response matching semantics explicitly")
	}
	checks := []map[string]any{}
	byID := map[string]map[string]any{}
	addExpected := func(expected map[string]any, path string) {
		for key, v := range expected {
			if key != "checks" && key != "checkPolicy" && v != nil {
				issue(path+"/"+key, "legacy expectation needs explicit conversion")
			}
		}
		if policy := object(expected["checkPolicy"]); len(policy) > 0 {
			if policy["minimumScore"] != float64(1) || policy["comparison"] != "gte" || policy["gating"] != true {
				issue(path+"/checkPolicy", "weighted threshold cannot be flattened automatically")
			}
		}
		for i, x := range list(expected["checks"]) {
			c := object(x)
			cid := str(c["id"])
			if cid == "" {
				issue(fmt.Sprintf("%s/checks/%d/id", path, i), "missing stable check identity")
				continue
			}
			if old, ok := byID[cid]; ok {
				if !reflect.DeepEqual(old, c) {
					issue(path+"/checks/"+cid, "duplicate check definitions disagree")
				}
				continue
			}
			byID[cid] = c
			checks = append(checks, c)
		}
	}
	addExpected(object(content["expected"]), "/content/expected")
	for i, x := range list(source["bindings"]) {
		b := object(x)
		path := fmt.Sprintf("/bindings/%d", i)
		if b["state"] != "ready" {
			issue(path+"/state", "binding is not ready")
		}
		if len(list(b["judges"])) > 0 || len(list(b["registryJudges"])) > 0 {
			issue(path, "registry bindings need pinned conversion")
		}
		addExpected(object(b["expected"]), path+"/expected")
	}
	judges := []any{}
	for _, c := range checks {
		cid := str(c["id"])
		impact := "required"
		if c["outcomeImpact"] == "informational" || c["required"] == false {
			impact = "advisory"
		}
		if c["outcomeImpact"] == "informational" && c["required"] == true {
			out.Notices = append(out.Notices, cid+": informational takes precedence over legacy required flag")
		}
		target := map[string]any{"event": "agent.response"}
		assert := map[string]any{}
		tool := str(c["tool"])
		if c["kind"] == "semantic" {
			assert["type"] = "semantic"
			assert["rubric"] = c["rubric"]
			if c["evidenceSource"] == "tool_result" {
				target["event"] = "component.executed"
				target["component"] = "tool"
				target["name"] = tool
			}
			if c["evidenceSource"] == "trace" {
				issue("/judges/"+cid, "whole-trace semantic context needs explicit mapping")
			}
			out.EvaluatorSnapshots[cid] = c
			if impact == "required" {
				issue("/judges/"+cid, "required semantic evaluator needs calibration rebinding to canonical fingerprint")
			}
		} else {
			switch c["assert"] {
			case "tool_called", "tool_not_called":
				if opts.ToolCallEvent != "component.requested" && opts.ToolCallEvent != "component.executed" {
					issue("/judges/"+cid, "choose requested or real-executed meaning for legacy tool calls")
				}
				target = map[string]any{"event": opts.ToolCallEvent, "component": "tool", "name": tool}
				assert["type"] = "exists"
				if c["assert"] == "tool_not_called" {
					assert = map[string]any{"type": "count", "value": 0}
				}
			case "tool_response":
				target = map[string]any{"event": "component.executed", "component": "tool", "name": tool}
				path := str(c["path"])
				field := strings.TrimPrefix(path, "$.")
				if !strings.HasPrefix(path, "$.") || field == "" || strings.ContainsAny(field, ".[]*") {
					issue("/judges/"+cid, "complex result path needs explicit JSON Pointer conversion")
				}
				if c["op"] != "eq" {
					issue("/judges/"+cid, "only literal equality result conversion is supported")
				}
				assert = map[string]any{"type": "equals", "path": "/result/" + pointer(field), "value": c["value"]}
			default:
				issue("/judges/"+cid, "unsupported legacy assertion")
			}
		}
		judges = append(judges, map[string]any{"id": cid, "impact": impact, "target": target, "assert": assert})
	}
	d["judges"] = judges
	out.Definition = d
	raw, _ := json.Marshal(d)
	validated := Validate(raw)
	out.Issues = append(out.Issues, validated.Issues...)
	if len(out.Issues) == 0 {
		out.Status = "convertible"
		out.Definition = validated.Definition
	}
	return out
}
