// Package database provides task artifact storage per docs/tech_specs/postgres_schema.md (Task Artifacts) and docs/tech_specs/mcp_tools/artifact_tools.md (artifact.get).
// REQ-ORCHES-0127: task create persists attachment references; ListArtifactPathsByTaskID supports API/CLI retrieval.
package database

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// GetArtifactByTaskIDAndPath returns the task artifact for the given task and path, or ErrNotFound.
func (db *DB) GetArtifactByTaskIDAndPath(ctx context.Context, taskID uuid.UUID, path string) (*models.TaskArtifact, error) {
	var record TaskArtifactRecord
	err := db.db.WithContext(ctx).Where("task_id = ? AND path = ?", taskID, path).First(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get artifact")
	}
	return record.ToTaskArtifact(), nil
}

// CreateTaskArtifact creates a task artifact row (path reference). storageRef may be empty for create-task attachment refs.
func (db *DB) CreateTaskArtifact(ctx context.Context, taskID uuid.UUID, path, storageRef string, sizeBytes *int64) (*models.TaskArtifact, error) {
	now := time.Now().UTC()
	record := &TaskArtifactRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		TaskArtifactBase: models.TaskArtifactBase{
			TaskID:     taskID,
			Path:       path,
			StorageRef: storageRef,
			SizeBytes:  sizeBytes,
		},
	}
	if err := db.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, wrapErr(err, "create task artifact")
	}
	return record.ToTaskArtifact(), nil
}

// ListArtifactPathsByTaskID returns artifact paths for the task (for API/CLI attachment linkage).
func (db *DB) ListArtifactPathsByTaskID(ctx context.Context, taskID uuid.UUID) ([]string, error) {
	var paths []string
	err := db.db.WithContext(ctx).Model(&TaskArtifactRecord{}).Where("task_id = ?", taskID).Pluck("path", &paths).Error
	if err != nil {
		return nil, wrapErr(err, "list artifact paths")
	}
	return paths, nil
}
