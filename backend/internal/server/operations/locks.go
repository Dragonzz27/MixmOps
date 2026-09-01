package operations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Locks struct{ db *sql.DB }

func NewLocks(db *sql.DB) *Locks { return &Locks{db: db} }
func (l *Locks) Acquire(ctx context.Context, key, ownerType, ownerID string, ttl time.Duration) (bool, error) {
	if l == nil || l.db == nil || key == "" || ownerID == "" {
		return false, fmt.Errorf("invalid operation lock")
	}
	now := time.Now().UTC()
	_, _ = l.db.ExecContext(ctx, `DELETE FROM operation_locks WHERE expires_at<?`, now.Format(time.RFC3339Nano))
	_, err := l.db.ExecContext(ctx, `INSERT INTO operation_locks(lock_key,owner_type,owner_id,expires_at,created_at) VALUES(?,?,?,?,?)`, key, ownerType, ownerID, now.Add(ttl).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		var current string
		if scanErr := l.db.QueryRowContext(ctx, `SELECT owner_id FROM operation_locks WHERE lock_key=?`, key).Scan(&current); scanErr == nil && current == ownerID {
			return true, nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "constraint") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func (l *Locks) Release(ctx context.Context, key, ownerID string) error {
	_, e := l.db.ExecContext(ctx, `DELETE FROM operation_locks WHERE lock_key=? AND owner_id=?`, key, ownerID)
	return e
}
