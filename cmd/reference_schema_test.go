package cmd

import (
	"encoding/json"
	"os/exec"
	"testing"
)

// TestReference_EveryLeafCommandHasRealSchemaAndExample guards against
// output_schema regressing to a stub: every leaf command must reference a
// schema label that exists in the top-level schemas map with a non-empty field
// list, and must carry at least one runnable example. This keeps `reference` a
// usable source of truth for agents. It black-boxes `reference --format json`
// (the same surface the FCC guard checks) so it needs no internal symbols.
func TestReference_EveryLeafCommandHasRealSchemaAndExample(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/wechat-mp-cli", "reference", "--format", "json")
	cmd.Dir = fccModuleRoot()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running reference: %v", err)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Commands []map[string]any          `json:"commands"`
			Schemas  map[string]map[string]any `json:"schemas"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("parsing reference envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatal("reference returned ok=false")
	}
	if len(envelope.Data.Commands) == 0 {
		t.Fatal("reference enumerated zero commands")
	}
	if len(envelope.Data.Schemas) == 0 {
		t.Fatal("reference exposed no schemas map")
	}

	leaves := 0
	var walk func(node map[string]any)
	walk = func(node map[string]any) {
		children, _ := node["commands"].([]any)
		if len(children) > 0 {
			for _, c := range children {
				if m, ok := c.(map[string]any); ok {
					walk(m)
				}
			}
			return
		}
		leaves++
		path, _ := node["path"].(string)
		label, _ := node["output_schema"].(string)
		if label == "" {
			t.Errorf("%s: empty output_schema", path)
			return
		}
		schema, ok := envelope.Data.Schemas[label]
		if !ok {
			t.Errorf("%s: output_schema %q not defined in schemas map", path, label)
			return
		}
		if fields, _ := schema["fields"].([]any); len(fields) == 0 {
			t.Errorf("%s: schema %q has no fields (stub)", path, label)
		}
		if examples, _ := node["examples"].([]any); len(examples) == 0 {
			t.Errorf("%s: no examples", path)
		}
	}
	for _, node := range envelope.Data.Commands {
		walk(node)
	}
	if leaves == 0 {
		t.Fatal("reference enumerated zero leaf commands")
	}
	t.Logf("schema guard: %d leaf commands verified", leaves)
}
