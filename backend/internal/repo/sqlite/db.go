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
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS background_tasks (id TEXT PRIMARY KEY, alert_fingerprint TEXT NOT NULL, alert_name TEXT, namespace TEXT, alert_severity TEXT, alert_labels TEXT, alert_annotations TEXT, status TEXT NOT NULL, target_pod TEXT, analysis TEXT, proposed_action TEXT, severity TEXT DEFAULT 'unknown', severity_reason TEXT, repair_status TEXT, repair_result TEXT, evidence_snapshot TEXT, incident_id TEXT, current_round INTEGER DEFAULT 0, max_rounds INTEGER DEFAULT 3, decision_snapshot TEXT, plan_snapshot TEXT, observation_snapshot TEXT, last_action_at TEXT, lock_key TEXT, retry_count INTEGER DEFAULT 0, cooldown_until TEXT, last_verified_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT);
CREATE UNIQUE INDEX IF NOT EXISTS idx_background_fingerprint_active ON background_tasks(alert_fingerprint) WHERE status NOT IN ('resolved','failed','cancelled');
CREATE TABLE IF NOT EXISTS incidents (id TEXT PRIMARY KEY, alert_fingerprint TEXT, alert_name TEXT, status TEXT NOT NULL, context_snapshot TEXT, source TEXT DEFAULT 'manual', background_task_id TEXT, severity TEXT, last_context_refresh_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, resolved_at TEXT);
CREATE TABLE IF NOT EXISTS agent_messages (id TEXT PRIMARY KEY, owner_type TEXT NOT NULL, owner_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT NOT NULL, metadata TEXT, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS agent_timeline (id TEXT PRIMARY KEY, owner_type TEXT NOT NULL, owner_id TEXT NOT NULL, event_type TEXT NOT NULL, event_name TEXT, status TEXT NOT NULL, input TEXT, output TEXT, error TEXT, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS incident_actions (id TEXT PRIMARY KEY, incident_id TEXT NOT NULL, action_type TEXT NOT NULL, namespace TEXT, target TEXT, resource_version TEXT, payload TEXT, reason TEXT, impact TEXT, risk TEXT, rollback TEXT, verification TEXT, status TEXT NOT NULL, proposed_by TEXT, confirmed_by TEXT, created_at TEXT NOT NULL, confirmed_at TEXT, executed_at TEXT, result TEXT, error TEXT);
CREATE TABLE IF NOT EXISTS agent_runs (id TEXT PRIMARY KEY, owner_type TEXT, owner_id TEXT, agent_name TEXT NOT NULL, status TEXT NOT NULL, input_snapshot TEXT, output_snapshot TEXT, started_at TEXT NOT NULL, ended_at TEXT, error TEXT);
CREATE TABLE IF NOT EXISTS agent_workers (id TEXT PRIMARY KEY, run_id TEXT NOT NULL, parent_worker_id TEXT, agent_name TEXT NOT NULL, prompt TEXT, status TEXT NOT NULL, result TEXT, error TEXT, started_at TEXT NOT NULL, ended_at TEXT);
CREATE TABLE IF NOT EXISTS agent_tool_calls (id TEXT PRIMARY KEY, run_id TEXT NOT NULL, worker_id TEXT, tool_name TEXT NOT NULL, tool_class TEXT, arguments TEXT, status TEXT NOT NULL, result TEXT, error TEXT, started_at TEXT NOT NULL, ended_at TEXT);
CREATE TABLE IF NOT EXISTS evidence_snapshots (id TEXT PRIMARY KEY, owner_type TEXT NOT NULL, owner_id TEXT NOT NULL, round INTEGER DEFAULT 0, source TEXT, payload TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS operation_locks (lock_key TEXT PRIMARY KEY, owner_type TEXT NOT NULL, owner_id TEXT NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL);

CREATE TABLE IF NOT EXISTS approvals (id TEXT PRIMARY KEY, task_id TEXT NOT NULL, action_type TEXT NOT NULL, action_payload TEXT NOT NULL, status TEXT NOT NULL, approved_by TEXT, reason TEXT, created_at TEXT NOT NULL, resolved_at TEXT);`)
	if err != nil {
		return err
	}
	// Older releases de-duplicated only by fingerprint.  Keep the migration
	// explicit so alerts with the same fingerprint in different namespaces or
	// rules do not incorrectly suppress one another.
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_background_fingerprint_active`)
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_background_alert_active ON background_tasks(alert_fingerprint,namespace,alert_name) WHERE status NOT IN ('resolved','failed','cancelled')`); err != nil {
		// A database created by a development build may already contain two
		// active rows for the same tuple. Do not make the whole server
		// unstartable: retain an indexed lookup and let CreateFromAlert's
		// idempotent read path handle the legacy duplicate.
		if _, fallbackErr := db.Exec(`CREATE INDEX IF NOT EXISTS idx_background_alert_lookup ON background_tasks(alert_fingerprint,namespace,alert_name)`); fallbackErr != nil {
			return fallbackErr
		}
	}
	// Add fields introduced by the background/incident workflow to databases
	// created by earlier AutoOps versions.
	for table, columns := range map[string][]string{
		"background_tasks": {"alert_severity TEXT", "alert_labels TEXT", "alert_annotations TEXT", "severity TEXT DEFAULT 'unknown'", "severity_reason TEXT", "repair_status TEXT", "repair_result TEXT", "evidence_snapshot TEXT", "incident_id TEXT", "current_round INTEGER DEFAULT 0", "max_rounds INTEGER DEFAULT 3", "decision_snapshot TEXT", "plan_snapshot TEXT", "observation_snapshot TEXT", "last_action_at TEXT", "lock_key TEXT", "retry_count INTEGER DEFAULT 0", "cooldown_until TEXT", "last_verified_at TEXT"},
		"incidents":        {"source TEXT DEFAULT 'manual'", "background_task_id TEXT", "severity TEXT", "last_context_refresh_at TEXT"},
		"incident_actions": {"impact TEXT", "resource_version TEXT"},
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
