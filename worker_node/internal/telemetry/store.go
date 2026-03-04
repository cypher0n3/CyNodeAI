// Package telemetry provides node-local SQLite storage for worker telemetry per worker_telemetry_api.md.
package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	schemaVersion   = 1
	maxLogRespBytes = 1048576 // 1 MiB per spec
)

// Store is the node-local telemetry SQLite store.
type Store struct {
	db       *sql.DB
	stateDir string
}

// Open opens the telemetry database at stateDir/telemetry/telemetry.db, creating the dir and running migrations.
func Open(ctx context.Context, stateDir string) (*Store, error) {
	if stateDir == "" {
		stateDir = "/var/lib/cynode/state"
	}
	telemetryDir := filepath.Join(stateDir, "telemetry")
	if err := os.MkdirAll(telemetryDir, 0o750); err != nil {
		return nil, fmt.Errorf("telemetry dir: %w", err)
	}
	dbPath := filepath.Join(telemetryDir, "telemetry.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, stateDir: stateDir}
	if err := s.runMigrations(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) runMigrations(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
		  id INTEGER PRIMARY KEY CHECK (id = 1),
		  version INTEGER NOT NULL,
		  applied_at TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("schema_version: %w", err)
	}
	var current int
	err = s.db.QueryRowContext(ctx, "SELECT version FROM schema_version WHERE id = 1").Scan(&current)
	if err == sql.ErrNoRows {
		_, err = s.db.ExecContext(ctx, "INSERT INTO schema_version (id, version, applied_at) VALUES (1, 0, ?)", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		current = 0
	} else if err != nil {
		return err
	}
	if current < 1 {
		if err := s.applyV1(ctx); err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx, "UPDATE schema_version SET version = 1, applied_at = ? WHERE id = 1", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyV1(ctx context.Context) error {
	v1 := `
		CREATE TABLE IF NOT EXISTS node_boot (
		  boot_id TEXT PRIMARY KEY,
		  booted_at TEXT NOT NULL,
		  node_slug TEXT NOT NULL,
		  build_version TEXT NOT NULL,
		  git_sha TEXT NOT NULL,
		  platform_os TEXT NOT NULL,
		  platform_arch TEXT NOT NULL,
		  kernel_version TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS container_inventory (
		  container_id TEXT PRIMARY KEY,
		  container_name TEXT NOT NULL,
		  kind TEXT NOT NULL CHECK (kind IN ('managed', 'sandbox')),
		  runtime TEXT NOT NULL,
		  image_ref TEXT NOT NULL,
		  created_at TEXT NOT NULL,
		  last_seen_at TEXT NOT NULL,
		  status TEXT NOT NULL,
		  exit_code INTEGER,
		  task_id TEXT,
		  job_id TEXT,
		  labels_json TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_container_inventory_kind_status ON container_inventory(kind, status);
		CREATE INDEX IF NOT EXISTS idx_container_inventory_task_job ON container_inventory(task_id, job_id);
		CREATE TABLE IF NOT EXISTS container_event (
		  event_id TEXT PRIMARY KEY,
		  occurred_at TEXT NOT NULL,
		  container_id TEXT NOT NULL,
		  action TEXT NOT NULL,
		  status TEXT NOT NULL,
		  exit_code INTEGER,
		  task_id TEXT,
		  job_id TEXT,
		  details_json TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_container_event_container_time ON container_event(container_id, occurred_at);
		CREATE INDEX IF NOT EXISTS idx_container_event_task_job ON container_event(task_id, job_id);
		CREATE TABLE IF NOT EXISTS log_event (
		  log_id TEXT PRIMARY KEY,
		  occurred_at TEXT NOT NULL,
		  source_kind TEXT NOT NULL CHECK (source_kind IN ('service', 'container')),
		  source_name TEXT NOT NULL,
		  container_id TEXT,
		  stream TEXT CHECK (stream IN ('stdout', 'stderr')),
		  level TEXT,
		  message TEXT NOT NULL,
		  fields_json TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_log_event_source_time ON log_event(source_kind, source_name, occurred_at);
		CREATE INDEX IF NOT EXISTS idx_log_event_container_time ON log_event(container_id, occurred_at);
	`
	_, err := s.db.ExecContext(ctx, v1)
	return err
}

// EnforceRetention deletes old log_event (7d), container_event (30d), and stale container_inventory (30d not seen).
func (s *Store) EnforceRetention(ctx context.Context) error {
	now := time.Now().UTC()
	logCutoff := now.AddDate(0, 0, -7).Format(time.RFC3339)
	eventCutoff := now.AddDate(0, 0, -30).Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, "DELETE FROM log_event WHERE occurred_at < ?", logCutoff); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM container_event WHERE occurred_at < ?", eventCutoff); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM container_inventory WHERE last_seen_at < ? AND status != 'running'", eventCutoff); err != nil {
		return err
	}
	return nil
}

// Vacuum runs SQLite VACUUM.
func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

// InsertTestContainer inserts one container_inventory row for testing (e.g. worker-api GET container success path).
func (s *Store) InsertTestContainer(ctx context.Context, containerID, name, kind, status, taskID, jobID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO container_inventory (
		container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, task_id, job_id, labels_json
	) VALUES (?, ?, ?, 'podman', 'img', ?, ?, ?, ?, ?, '{}')`,
		containerID, name, kind, now, now, status, taskID, jobID)
	return err
}
