// Package database provides GORM record structs for workflow-related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// WorkflowCheckpointRecord is the GORM record struct for the workflow_checkpoints table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain WorkflowCheckpointBase struct.
// Note: WorkflowCheckpoint domain type uses UpdatedAt from GormModelUUID.UpdatedAt.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type WorkflowCheckpointRecord struct {
	gormmodel.GormModelUUID
	models.WorkflowCheckpointBase
}

// TableName implements the GORM TableName interface.
func (WorkflowCheckpointRecord) TableName() string {
	return "workflow_checkpoints"
}

// ToWorkflowCheckpoint converts a WorkflowCheckpointRecord to a domain WorkflowCheckpoint with all fields populated.
func (r *WorkflowCheckpointRecord) ToWorkflowCheckpoint() *models.WorkflowCheckpoint {
	return &models.WorkflowCheckpoint{
		WorkflowCheckpointBase: models.WorkflowCheckpointBase{
			TaskID:     r.WorkflowCheckpointBase.TaskID,
			State:      r.WorkflowCheckpointBase.State,
			LastNodeID: r.WorkflowCheckpointBase.LastNodeID,
		},
		ID:        r.GormModelUUID.ID,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// TaskWorkflowLeaseRecord is the GORM record struct for the task_workflow_leases table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain TaskWorkflowLeaseBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type TaskWorkflowLeaseRecord struct {
	gormmodel.GormModelUUID
	models.TaskWorkflowLeaseBase
}

// TableName implements the GORM TableName interface.
func (TaskWorkflowLeaseRecord) TableName() string {
	return "task_workflow_leases"
}

// ToTaskWorkflowLease converts a TaskWorkflowLeaseRecord to a domain TaskWorkflowLease with all fields populated.
func (r *TaskWorkflowLeaseRecord) ToTaskWorkflowLease() *models.TaskWorkflowLease {
	return &models.TaskWorkflowLease{
		TaskWorkflowLeaseBase: models.TaskWorkflowLeaseBase{
			TaskID:    r.TaskWorkflowLeaseBase.TaskID,
			LeaseID:   r.TaskWorkflowLeaseBase.LeaseID,
			HolderID:  r.TaskWorkflowLeaseBase.HolderID,
			ExpiresAt: r.TaskWorkflowLeaseBase.ExpiresAt,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}
