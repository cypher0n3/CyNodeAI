// Package database provides PostgreSQL database operations for the orchestrator.
package database

import (
	"context"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// CreateMcpToolCallAuditLog inserts one MCP tool call audit record. Caller may leave ID and CreatedAt zero; they are set here.
func (db *DB) CreateMcpToolCallAuditLog(ctx context.Context, rec *models.McpToolCallAuditLog) error {
	ensureAuditIDAndTime(&rec.ID, &rec.CreatedAt)
	return db.createRecord(ctx, rec, "create mcp tool call audit log")
}
