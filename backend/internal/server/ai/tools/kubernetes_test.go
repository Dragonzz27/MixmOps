package tools

import "testing"

func TestResolveNamespace(t *testing.T) {
	if got, err := resolveNamespace("", "autoops-test"); err != nil || got != "autoops-test" {
		t.Fatalf("default namespace: %q, %v", got, err)
	}
	if _, err := resolveNamespace("kube-system", "autoops-test"); err == nil {
		t.Fatal("expected namespace escape to be rejected")
	}
}
