// Package database provides GORM record structs for artifact-related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

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
	return &models.TaskArtifact{
		TaskArtifactBase: r.TaskArtifactBase,
		ID:               r.ID,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}
