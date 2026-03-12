// Package telemetry provides node-local SQLite storage for worker telemetry per worker_telemetry_api.md.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const maxLogRespBytes = 1048576 // 1 MiB per spec

// Store is the node-local telemetry SQLite store.
type Store struct {
	db       *gorm.DB
	stateDir string
}

// Open opens the telemetry database at stateDir/telemetry/telemetry.db, creating the dir and running GORM AutoMigrate.
func Open(ctx context.Context, stateDir string) (*Store, error) {
	if stateDir == "" {
		stateDir = "/var/lib/cynode/state"
	}
	telemetryDir := filepath.Join(stateDir, "telemetry")
	if err := os.MkdirAll(telemetryDir, 0o750); err != nil {
		return nil, fmt.Errorf("telemetry dir: %w", err)
	}
	dbPath := filepath.Join(telemetryDir, "telemetry.db")
	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.WithContext(ctx).AutoMigrate(
		&SchemaVersion{},
		&NodeBoot{},
		&ContainerInventory{},
		&ContainerEvent{},
		&LogEvent{},
	); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("automigrate: %w", err)
	}
	if err := ensureSchemaVersionRow(db); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("schema_version: %w", err)
	}
	return &Store{db: db, stateDir: stateDir}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// EnforceRetention deletes old log_event (7d), container_event (30d), and stale container_inventory (30d not seen).
func (s *Store) EnforceRetention(ctx context.Context) error {
	now := time.Now().UTC()
	logCutoff := now.AddDate(0, 0, -7).Format(time.RFC3339)
	eventCutoff := now.AddDate(0, 0, -30).Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Where("occurred_at < ?", logCutoff).Delete(&LogEvent{}).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Where("occurred_at < ?", eventCutoff).Delete(&ContainerEvent{}).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Where("last_seen_at < ? AND status != ?", eventCutoff, "running").Delete(&ContainerInventory{}).Error; err != nil {
		return err
	}
	return nil
}

// Vacuum runs SQLite VACUUM.
func (s *Store) Vacuum(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec("VACUUM").Error
}

// InsertTestContainer inserts one container_inventory row for testing (e.g. worker-api GET container success path).
func (s *Store) InsertTestContainer(ctx context.Context, containerID, name, kind, status, taskID, jobID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.db.WithContext(ctx).Create(&ContainerInventory{
		ContainerID:   containerID,
		ContainerName: name,
		Kind:          kind,
		Runtime:       "podman",
		ImageRef:      "img",
		CreatedAt:     now,
		LastSeenAt:    now,
		Status:        status,
		TaskID:        taskID,
		JobID:         jobID,
		LabelsJSON:    "{}",
	}).Error
}
