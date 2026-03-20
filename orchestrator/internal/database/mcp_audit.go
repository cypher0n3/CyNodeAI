// Package database provides PostgreSQL database operations for the orchestrator.
package database

import (
	"context"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// CreateMcpToolCallAuditLog inserts one MCP tool call audit record. Caller may leave ID and CreatedAt zero; they are set here.
func (db *DB) CreateMcpToolCallAuditLog(ctx context.Context, rec *models.McpToolCallAuditLog) error {
	record := &McpToolCallAuditLogRecord{
		McpToolCallAuditLogBase: models.McpToolCallAuditLogBase{
			TaskID:      rec.TaskID,
			ProjectID:   rec.ProjectID,
			RunID:       rec.RunID,
			JobID:       rec.JobID,
			SubjectType: rec.SubjectType,
			SubjectID:   rec.SubjectID,
			UserID:      rec.UserID,
			GroupIDs:    rec.GroupIDs,
			RoleNames:   rec.RoleNames,
			ToolName:    rec.ToolName,
			Decision:    rec.Decision,
			Status:      rec.Status,
			DurationMs:  rec.DurationMs,
			ErrorType:   rec.ErrorType,
		},
	}
	ensureAuditIDAndTime(&record.ID, &record.CreatedAt)
	if err := db.createRecord(ctx, record, "create mcp tool call audit log"); err != nil {
		return err
	}
	// Populate the input struct with the generated ID and CreatedAt
	rec.ID = record.ID
	rec.CreatedAt = record.CreatedAt
	return nil
}
