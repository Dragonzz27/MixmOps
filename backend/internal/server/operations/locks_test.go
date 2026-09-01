package operations

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestLocksAcquireAndRelease(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE operation_locks (lock_key TEXT PRIMARY KEY, owner_type TEXT, owner_id TEXT, expires_at TEXT, created_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	locks := NewLocks(db)
	if ok, err := locks.Acquire(context.Background(), "pod:ns/a", "background_task", "task-a", time.Minute); err != nil || !ok {
		t.Fatalf("first acquire ok=%v err=%v", ok, err)
	}
	if ok, err := locks.Acquire(context.Background(), "pod:ns/a", "background_task", "task-b", time.Minute); err != nil || ok {
		t.Fatalf("second owner should be blocked, ok=%v err=%v", ok, err)
	}
	if err := locks.Release(context.Background(), "pod:ns/a", "task-a"); err != nil {
		t.Fatal(err)
	}
	if ok, err := locks.Acquire(context.Background(), "pod:ns/a", "background_task", "task-b", time.Minute); err != nil || !ok {
		t.Fatalf("acquire after release ok=%v err=%v", ok, err)
	}
}
