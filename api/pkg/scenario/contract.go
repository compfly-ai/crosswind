// Package scenario defines the common atomic scenario authoring boundary.
// Validation never installs mocks, restores state, grants permission or executes an agent.
package scenario

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schema.json
var Schema []byte

const SchemaURI = "compfly://scenarios/v1/schema"
const MaxDocumentBytes = 1 << 20

type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}
type Result struct {
	Valid      bool           `json:"valid"`
	Issues     []Issue        `json:"issues"`
	Digest     string         `json:"digest,omitempty"`
	Definition map[string]any `json:"definition,omitempty"`
}

var compiled *jsonschema.Schema
var compileErr error
var once sync.Once

func validator() (*jsonschema.Schema, error) {
	once.Do(func() {
		v, e := jsonschema.UnmarshalJSON(bytes.NewReader(Schema))
		if e != nil {
			compileErr = e
			return
		}
		c := jsonschema.NewCompiler()
		if e = c.AddResource(SchemaURI, v); e != nil {
			compileErr = e
			return
		}
		compiled, compileErr = c.Compile(SchemaURI)
	})
	return compiled, compileErr
}
func pointer(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }

// Decode accepts strict JSON-compatible YAML, including JSON. It rejects aliases,
// duplicate keys, multiple documents and non-JSON scalars before schema validation.
func Decode(raw []byte) (map[string]any, error) {
	if len(raw) > MaxDocumentBytes {
		return nil, fmt.Errorf("scenario exceeds %d bytes", MaxDocumentBytes)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var n yaml.Node
	if err := dec.Decode(&n); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("exactly one document is required")
	}
	var visit func(*yaml.Node) (any, error)
	visit = func(n *yaml.Node) (any, error) {
		switch n.Kind {
		case yaml.DocumentNode:
			if len(n.Content) != 1 {
				return nil, fmt.Errorf("one document required")
			}
			return visit(n.Content[0])
		case yaml.MappingNode:
			if n.Tag != "!!map" {
				return nil, fmt.Errorf("custom YAML tags are unsupported")
			}
			out := map[string]any{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
					return nil, fmt.Errorf("object keys must be strings")
				}
				if _, ok := out[k.Value]; ok {
					return nil, fmt.Errorf("duplicate key %q", k.Value)
				}
				v, e := visit(n.Content[i+1])
				if e != nil {
					return nil, e
				}
				out[k.Value] = v
			}
			return out, nil
		case yaml.SequenceNode:
			if n.Tag != "!!seq" {
				return nil, fmt.Errorf("custom YAML tags are unsupported")
			}
			out := []any{}
			for _, child := range n.Content {
				v, e := visit(child)
				if e != nil {
					return nil, e
				}
				out = append(out, v)
			}
			return out, nil
		case yaml.ScalarNode:
			if n.Tag != "!!str" && n.Tag != "!!bool" && n.Tag != "!!int" && n.Tag != "!!float" && n.Tag != "!!null" {
				return nil, fmt.Errorf("only JSON scalar types are supported")
			}
			var v any
			if e := n.Decode(&v); e != nil {
				return nil, e
			}
			return v, nil
		default:
			return nil, fmt.Errorf("aliases and custom YAML nodes are unsupported")
		}
	}
	v, err := visit(&n)
	if err != nil {
		return nil, err
	}
	// Normalize all callers through the same JSON representation and number type.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var d map[string]any
	err = json.Unmarshal(b, &d)
	if err != nil || d == nil {
		return nil, fmt.Errorf("scenario must be an object")
	}
	return d, nil
}
func Validate(raw []byte) Result {
	r := Result{Issues: []Issue{}}
	d, e := Decode(raw)
	if e != nil {
		r.Issues = append(r.Issues, Issue{"invalid_document", "", e.Error()})
		return r
	}
	v, e := validator()
	if e != nil {
		r.Issues = append(r.Issues, Issue{"contract_unavailable", "", e.Error()})
		return r
	}
	if e = v.Validate(d); e != nil {
		var flatten func(*jsonschema.ValidationError)
		flatten = func(e *jsonschema.ValidationError) {
			if len(e.Causes) > 0 {
				for _, c := range e.Causes {
					flatten(c)
				}
				return
			}
			p := ""
			for _, s := range e.InstanceLocation {
				p += "/" + pointer(s)
			}
			r.Issues = append(r.Issues, Issue{"invalid_shape", p, e.Error()})
		}
		if ve, ok := e.(*jsonschema.ValidationError); ok {
			flatten(ve)
		} else {
			r.Issues = append(r.Issues, Issue{"invalid_shape", "", e.Error()})
		}
		return r
	}
	add := func(code, path, message string) { r.Issues = append(r.Issues, Issue{code, path, message}) }
	steps := map[string]bool{}
	for i, x := range d["steps"].([]any) {
		s := x.(map[string]any)
		id := s["id"].(string)
		if steps[id] {
			add("duplicate_id", fmt.Sprintf("/steps/%d/id", i), "step IDs must be unique")
		}
		steps[id] = true
	}
	for _, field := range []string{"overrides", "judges"} {
		ids := map[string]bool{}
		items, _ := d[field].([]any)
		for i, x := range items {
			item := x.(map[string]any)
			base := "/" + field + "/" + strconv.Itoa(i)
			id := item["id"].(string)
			if ids[id] {
				add("duplicate_id", base+"/id", "IDs must be unique")
			}
			ids[id] = true
			selector := "target"
			if field == "overrides" {
				selector = "match"
			}
			sel := item[selector].(map[string]any)
			if step, ok := sel["step"].(string); ok && !steps[step] {
				add("unknown_step", base+"/"+selector+"/step", "step does not exist")
			}
			if field == "overrides" {
				action := item["action"].(map[string]any)["type"]
				phase := sel["phase"]
				if (action == "replace_response") != (phase == "after") {
					add("invalid_phase", base+"/match/phase", "only replace_response uses the after phase")
				}
			}
			if field == "judges" {
				a := item["assert"].(map[string]any)
				if a["type"] == "before" {
					other := a["other"].(map[string]any)
					if step, ok := other["step"].(string); ok && !steps[step] {
						add("unknown_step", base+"/assert/other/step", "step does not exist")
					}
				}
			}
		}
	}
	budget := 0
	for i, x := range d["steps"].([]any) {
		step := x.(map[string]any)
		if adaptive, ok := step["adaptive"].(map[string]any); ok {
			budget += int(adaptive["maxTurns"].(float64))
			if adaptive["stopOn"] == "required_judge_failure" {
				found := false
				for _, raw := range d["judges"].([]any) {
					j := raw.(map[string]any)
					if j["impact"] == "required" && j["target"].(map[string]any)["step"] == step["id"] {
						found = true
					}
				}
				if !found {
					add("missing_stop_judge", fmt.Sprintf("/steps/%d/adaptive/stopOn", i), "requires a required judge targeted at this adaptive step")
				}
			}
		} else {
			budget++
		}
	}
	if budget > 128 {
		add("turn_budget_exceeded", "/steps", "scenario supports at most 128 planned turns")
	}
	sort.SliceStable(r.Issues, func(i, j int) bool { return r.Issues[i].Path < r.Issues[j].Path })
	if len(r.Issues) > 0 {
		return r
	}
	b, _ := json.Marshal(d)
	hash := sha256.Sum256(b)
	r.Valid = true
	r.Definition = d
	r.Digest = "sha256:" + hex.EncodeToString(hash[:])
	return r
}
