// Package database: GORM records for task artifacts (postgres_schema / artifact tools) and PMA session bindings (REQ-ORCHES-0188).
package database

import (
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func gormUUIDTimestamps(m *gormmodel.GormModelUUID) (id uuid.UUID, createdAt, updatedAt time.Time) {
	return m.ID, m.CreatedAt, m.UpdatedAt
}

// TaskArtifactRecord is the GORM record struct for the task_artifacts table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain TaskArtifactBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type TaskArtifactRecord struct {
	gormmodel.GormModelUUID
	models.TaskArtifactBase
}

// TableName implements the GORM TableName interface.
func (TaskArtifactRecord) TableName() string {
	return "task_artifacts"
}

// ToTaskArtifact converts a TaskArtifactRecord to a domain TaskArtifact with all fields populated.
func (r *TaskArtifactRecord) ToTaskArtifact() *models.TaskArtifact {
	id, ca, ua := gormUUIDTimestamps(&r.GormModelUUID)
	return &models.TaskArtifact{
		TaskArtifactBase: r.TaskArtifactBase,
		ID:               id,
		CreatedAt:        ca,
		UpdatedAt:        ua,
	}
}

// SessionBindingRecord is the GORM record for the session_bindings table.
type SessionBindingRecord struct {
	gormmodel.GormModelUUID
	models.SessionBindingBase
}

// TableName implements gorm.Tabler.
func (SessionBindingRecord) TableName() string {
	return "session_bindings"
}

// ToSessionBinding maps the record to the domain type.
func (r *SessionBindingRecord) ToSessionBinding() *models.SessionBinding {
	id, ca, ua := gormUUIDTimestamps(&r.GormModelUUID)
	return &models.SessionBinding{
		SessionBindingBase: r.SessionBindingBase,
		ID:                 id,
		CreatedAt:          ca,
		UpdatedAt:          ua,
	}
}
