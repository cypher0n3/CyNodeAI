// Package database provides PostgreSQL database operations via GORM.
// This file defines GORM record structs for task and job-related tables.
// See docs/tech_specs/go_sql_database_standards.md (CYNAI.STANDS.GormModelStructure).
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// TaskRecord is the GORM record struct for the tasks table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain TaskBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type TaskRecord struct {
	gormmodel.GormModelUUID
	models.TaskBase
}

// TableName implements the GORM TableName interface.
func (TaskRecord) TableName() string {
	return "tasks"
}

// ToTask converts a TaskRecord to a domain Task with all fields populated.
func (r *TaskRecord) ToTask() *models.Task {
	return &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy:          r.TaskBase.CreatedBy,
			ProjectID:          r.TaskBase.ProjectID,
			PlanID:             r.TaskBase.PlanID,
			Status:             r.TaskBase.Status,
			Closed:             r.TaskBase.Closed,
			Prompt:             r.TaskBase.Prompt,
			AcceptanceCriteria: r.TaskBase.AcceptanceCriteria,
			Summary:            r.TaskBase.Summary,
			Metadata:           r.TaskBase.Metadata,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// JobRecord is the GORM record struct for the jobs table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain JobBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type JobRecord struct {
	gormmodel.GormModelUUID
	models.JobBase
}

// TableName implements the GORM TableName interface.
func (JobRecord) TableName() string {
	return "jobs"
}

// ToJob converts a JobRecord to a domain Job with all fields populated.
func (r *JobRecord) ToJob() *models.Job {
	return &models.Job{
		JobBase: models.JobBase{
			TaskID:         r.JobBase.TaskID,
			NodeID:          r.JobBase.NodeID,
			Status:          r.JobBase.Status,
			Payload:         r.JobBase.Payload,
			Result:          r.JobBase.Result,
			LeaseID:         r.JobBase.LeaseID,
			LeaseExpiresAt:  r.JobBase.LeaseExpiresAt,
			StartedAt:       r.JobBase.StartedAt,
			EndedAt:         r.JobBase.EndedAt,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// TaskDependencyRecord is the GORM record struct for the task_dependencies table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain TaskDependencyBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type TaskDependencyRecord struct {
	gormmodel.GormModelUUID
	models.TaskDependencyBase
}

// TableName implements the GORM TableName interface.
func (TaskDependencyRecord) TableName() string {
	return "task_dependencies"
}

// ToTaskDependency converts a TaskDependencyRecord to a domain TaskDependency with all fields populated.
func (r *TaskDependencyRecord) ToTaskDependency() *models.TaskDependency {
	return &models.TaskDependency{
		TaskDependencyBase: models.TaskDependencyBase{
			TaskID:          r.TaskDependencyBase.TaskID,
			DependsOnTaskID: r.TaskDependencyBase.DependsOnTaskID,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}
