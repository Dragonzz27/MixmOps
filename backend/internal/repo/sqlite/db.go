package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

func Open(path string) (*DB, error) {
	if path == "" {
		path = "data/autoops.db"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, err
	}
	if err = migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS background_tasks (id TEXT PRIMARY KEY, alert_fingerprint TEXT NOT NULL, alert_name TEXT, namespace TEXT, status TEXT NOT NULL, target_pod TEXT, analysis TEXT, proposed_action TEXT, severity TEXT DEFAULT 'unknown', severity_reason TEXT, repair_status TEXT, repair_result TEXT, evidence_snapshot TEXT, incident_id TEXT, retry_count INTEGER DEFAULT 0, cooldown_until TEXT, last_verified_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT);
CREATE UNIQUE INDEX IF NOT EXISTS idx_background_fingerprint_active ON background_tasks(alert_fingerprint) WHERE status NOT IN ('resolved','failed','cancelled');
CREATE TABLE IF NOT EXISTS incidents (id TEXT PRIMARY KEY, alert_fingerprint TEXT, alert_name TEXT, status TEXT NOT NULL, context_snapshot TEXT, source TEXT DEFAULT 'manual', background_task_id TEXT, severity TEXT, last_context_refresh_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, resolved_at TEXT);
CREATE TABLE IF NOT EXISTS work_orders (id TEXT PRIMARY KEY, type TEXT NOT NULL, title TEXT NOT NULL, namespace TEXT, target TEXT, parameters TEXT, status TEXT NOT NULL, description TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT);
CREATE TABLE IF NOT EXISTS agent_messages (id TEXT PRIMARY KEY, owner_type TEXT NOT NULL, owner_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT NOT NULL, metadata TEXT, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS agent_timeline (id TEXT PRIMARY KEY, owner_type TEXT NOT NULL, owner_id TEXT NOT NULL, event_type TEXT NOT NULL, event_name TEXT, status TEXT NOT NULL, input TEXT, output TEXT, error TEXT, created_at TEXT NOT NULL);

CREATE TABLE IF NOT EXISTS approvals (id TEXT PRIMARY KEY, task_id TEXT NOT NULL, action_type TEXT NOT NULL, action_payload TEXT NOT NULL, status TEXT NOT NULL, approved_by TEXT, reason TEXT, created_at TEXT NOT NULL, resolved_at TEXT);`)
	if err != nil {
		return err
	}
	// Add fields introduced by the background/incident workflow to databases
	// created by earlier AutoOps versions.
	for table, columns := range map[string][]string{
		"background_tasks": {"severity TEXT DEFAULT 'unknown'", "severity_reason TEXT", "repair_status TEXT", "repair_result TEXT", "evidence_snapshot TEXT", "incident_id TEXT", "retry_count INTEGER DEFAULT 0", "cooldown_until TEXT", "last_verified_at TEXT"},
		"incidents":        {"source TEXT DEFAULT 'manual'", "background_task_id TEXT", "severity TEXT", "last_context_refresh_at TEXT"},
	} {
		for _, column := range columns {
			name := column[:strings.Index(column, " ")]
			var count int
			if e := db.QueryRow("SELECT count(*) FROM pragma_table_info(?) WHERE name=?", table, name).Scan(&count); e != nil {
				return e
			}
			if count == 0 {
				if _, e := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
