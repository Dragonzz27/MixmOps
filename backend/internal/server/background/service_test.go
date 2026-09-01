package background

import (
	"context"
	"testing"

	sqliterepo "AutoOps/internal/repo/sqlite"
)

func TestTimelineListHandlesNullableError(t *testing.T) {
	db, err := sqliterepo.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewService(db.DB, nil)
	task, created, err := service.CreateFromAlert(context.Background(), Alert{
		Fingerprint: "fp-null-error",
		Name:        "AutoOpsTestAlert",
		Namespace:   "autoops-test",
		Labels:      map[string]string{"severity": "warning"},
	})
	if err != nil || !created {
		t.Fatalf("create task: created=%v err=%v", created, err)
	}
	if err := service.Timeline(context.Background(), task.ID, "evidence_collected", "success", "evidence"); err != nil {
		t.Fatal(err)
	}
	items, err := service.TimelineList(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("timeline with NULL error should be readable: %v", err)
	}
	if len(items) != 1 || items[0]["error"] != "" {
		t.Fatalf("unexpected timeline: %#v", items)
	}
}
