// Package database provides task artifact storage per docs/tech_specs/postgres_schema.md (Task Artifacts) and mcp_tool_catalog.md (artifact.get).
package database

import (
	"context"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// GetArtifactByTaskIDAndPath returns the task artifact for the given task and path, or ErrNotFound.
func (db *DB) GetArtifactByTaskIDAndPath(ctx context.Context, taskID uuid.UUID, path string) (*models.TaskArtifact, error) {
	var ent models.TaskArtifact
	err := db.db.WithContext(ctx).Where("task_id = ? AND path = ?", taskID, path).First(&ent).Error
	if err != nil {
		return nil, wrapErr(err, "get artifact")
	}
	return &ent, nil
}
