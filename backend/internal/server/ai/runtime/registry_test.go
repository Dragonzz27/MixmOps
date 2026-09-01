package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPersonaAndRejectDuplicate(t *testing.T) {
	d := t.TempDir()
	write := func(name, body string) {
		if e := os.WriteFile(filepath.Join(d, name), []byte(body), 0644); e != nil {
			t.Fatal(e)
		}
	}
	write("a.md", "---\nname: incident\ndescription: test\ncapabilities:\n  - id: rca\n    tools: [query_kubernetes_resources]\ntools: []\n---\nincident prompt")
	r, e := Load(d)
	if e != nil {
		t.Fatal(e)
	}
	p, ok := r.Get("incident")
	if !ok || p.Prompt != "incident prompt" {
		t.Fatalf("unexpected persona: %+v", p)
	}
	if len(p.Capabilities) != 1 || p.Capabilities[0].ID != "rca" {
		t.Fatalf("capabilities were not parsed: %+v", p.Capabilities)
	}
	write("b.md", "---\nname: incident\ndescription: duplicate\n---\nx")
	if _, e = Load(d); e == nil {
		t.Fatal("expected duplicate error")
	}
}
func TestRegistryAll(t *testing.T) {
	r := &Registry{items: map[string]AgentDefinition{"a": {Name: "a"}}}
	if len(r.All()) != 1 {
		t.Fatal("expected one definition")
	}
}
