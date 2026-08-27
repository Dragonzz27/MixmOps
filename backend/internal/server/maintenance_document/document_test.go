package maintenancedocument

import "testing"

func TestParseFrontMatter(t *testing.T) {
	meta, body, err := Parse("---\ntitle: 集群架构\ntype: cluster-architecture\ntags:\n  - minikube\n---\n# 集群\n正文")
	if err != nil || meta.Title != "集群架构" || meta.Type != TypeClusterArchitecture || len(meta.Tags) != 1 || body != "# 集群\n正文" {
		t.Fatalf("unexpected parse: %#v %q %v", meta, body, err)
	}
}

func TestParseRejectsInvalidMetadata(t *testing.T) {
	for _, content := range []string{"# no front matter", "---\ntitle: x\ntype: bad\n---\n# body", "---\ntype: operations-manual\n---\n# body"} {
		if _, _, err := Parse(content); err == nil {
			t.Fatalf("expected parse error for %q", content)
		}
	}
}
