package scenario

import (
	"encoding/json"
	"testing"
)

func legacyFixture() map[string]any {
	var d map[string]any
	json.Unmarshal([]byte(`{"id":"scenario_one","revision":"saved-v1","split":"train","content":{"prompt":"Read my inbox","expected":{"checks":[{"id":"read","kind":"deterministic","assert":"tool_called","tool":"read_messages","required":true},{"id":"honesty","kind":"semantic","rubric":"Use the inbox result","tool":"read_messages","evidenceSource":"tool_result","required":true,"outcomeImpact":"informational"}]}},"bindings":[{"state":"ready","expected":{"checks":[{"id":"read","kind":"deterministic","assert":"tool_called","tool":"read_messages","required":true},{"id":"honesty","kind":"semantic","rubric":"Use the inbox result","tool":"read_messages","evidenceSource":"tool_result","required":true,"outcomeImpact":"informational"}]}}]}`), &d)
	return d
}
func TestConversionDeduplicatesAndPreservesAdvisory(t *testing.T) {
	r := ConvertLibrary(legacyFixture(), MigrationOptions{"component.executed"})
	if r.Status != "convertible" {
		t.Fatal(r.Issues)
	}
	j := r.Definition["judges"].([]any)
	if len(j) != 2 || j[1].(map[string]any)["impact"] != "advisory" {
		t.Fatal(j)
	}
	if len(r.EvaluatorSnapshots) != 1 || len(r.Notices) != 1 {
		t.Fatal("lost metadata", r)
	}
	if _, ok := r.Definition["bindings"]; ok {
		t.Fatal("duplicate executable layer")
	}
}
func TestConversionRejectsConflictAndUnknownMeaning(t *testing.T) {
	r := ConvertLibrary(legacyFixture(), MigrationOptions{})
	if r.Status != "review_required" {
		t.Fatal("inferred tool call semantics")
	}
	d := legacyFixture()
	d["bindings"].([]any)[0].(map[string]any)["expected"].(map[string]any)["checks"].([]any)[0].(map[string]any)["tool"] = "send_email"
	r = ConvertLibrary(d, MigrationOptions{"component.executed"})
	if r.Status != "review_required" {
		t.Fatal("silently overwrote conflicting check")
	}
}
