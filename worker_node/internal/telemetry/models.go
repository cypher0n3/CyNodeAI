// Package telemetry: GORM models for worker_telemetry_api.md schema.

package telemetry

import (
	"time"

	"gorm.io/gorm"
)

// SchemaVersion tracks applied schema version per spec (used after AutoMigrate).
type SchemaVersion struct {
	ID        int    `gorm:"column:id;primaryKey"`
	Version   int    `gorm:"column:version;not null"`
	AppliedAt string `gorm:"column:applied_at;not null"`
}

func (SchemaVersion) TableName() string { return "schema_version" }

// NodeBoot is the node_boot table.
type NodeBoot struct {
	BootID        string `gorm:"column:boot_id;primaryKey"`
	BootedAt      string `gorm:"column:booted_at;not null"`
	NodeSlug      string `gorm:"column:node_slug;not null"`
	BuildVersion  string `gorm:"column:build_version;not null"`
	PlatformOS    string `gorm:"column:platform_os;not null"`
	PlatformArch  string `gorm:"column:platform_arch;not null"`
	KernelVersion string `gorm:"column:kernel_version;not null"`
}

func (NodeBoot) TableName() string { return "node_boot" }

// ContainerInventory is the container_inventory table.
type ContainerInventory struct {
	ContainerID   string `gorm:"column:container_id;primaryKey"`
	ContainerName string `gorm:"column:container_name;not null"`
	Kind          string `gorm:"column:kind;not null"`
	Runtime       string `gorm:"column:runtime;not null"`
	ImageRef      string `gorm:"column:image_ref;not null"`
	CreatedAt     string `gorm:"column:created_at;not null"`
	LastSeenAt    string `gorm:"column:last_seen_at;not null;index"` // hot: retention / stale queries
	Status        string `gorm:"column:status;not null;index"`       // hot: filter by running/exited
	ExitCode      *int   `gorm:"column:exit_code"`
	TaskID        string `gorm:"column:task_id"`
	JobID         string `gorm:"column:job_id"`
	LabelsJSON    string `gorm:"column:labels_json;not null"`
}

func (ContainerInventory) TableName() string { return "container_inventory" }

// ContainerEvent is the container_event table.
type ContainerEvent struct {
	EventID     string `gorm:"column:event_id;primaryKey"`
	OccurredAt  string `gorm:"column:occurred_at;not null"`
	ContainerID string `gorm:"column:container_id;not null"`
	Action      string `gorm:"column:action;not null"`
	Status      string `gorm:"column:status;not null"`
	ExitCode    *int   `gorm:"column:exit_code"`
	TaskID      string `gorm:"column:task_id"`
	JobID       string `gorm:"column:job_id"`
	DetailsJSON string `gorm:"column:details_json;not null"`
}

func (ContainerEvent) TableName() string { return "container_event" }

// LogEvent is the log_event table.
type LogEvent struct {
	LogID       string `gorm:"column:log_id;primaryKey"`
	OccurredAt  string `gorm:"column:occurred_at;not null;index"` // hot: time-ordered log queries
	SourceKind  string `gorm:"column:source_kind;not null"`
	SourceName  string `gorm:"column:source_name;not null"`
	ContainerID string `gorm:"column:container_id;index"` // hot: filter by container
	Stream      string `gorm:"column:stream"`
	Level       string `gorm:"column:level"`
	Message     string `gorm:"column:message;not null"`
	FieldsJSON  string `gorm:"column:fields_json;not null"`
}

func (LogEvent) TableName() string { return "log_event" }

// ensureSchemaVersionRow creates schema_version row id=1 if not present (spec compliance).
func ensureSchemaVersionRow(db *gorm.DB) error {
	var count int64
	if err := db.Model(&SchemaVersion{}).Where("id = ?", 1).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return db.Create(&SchemaVersion{
			ID: 1, Version: 1,
			AppliedAt: time.Now().UTC().Format(time.RFC3339),
		}).Error
	}
	return nil
}
