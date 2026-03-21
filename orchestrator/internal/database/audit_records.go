// Package database provides GORM record structs for audit and log tables.
package database

import (
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// McpToolCallAuditLogRecord is the GORM record struct for the mcp_tool_call_audit_log table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain McpToolCallAuditLogBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type McpToolCallAuditLogRecord struct {
	gormmodel.GormModelUUID
	models.McpToolCallAuditLogBase
}

// TableName implements the GORM TableName interface.
func (McpToolCallAuditLogRecord) TableName() string {
	return "mcp_tool_call_audit_log"
}

// ToMcpToolCallAuditLog converts a McpToolCallAuditLogRecord to a domain McpToolCallAuditLog with all fields populated.
func (r *McpToolCallAuditLogRecord) ToMcpToolCallAuditLog() *models.McpToolCallAuditLog {
	return &models.McpToolCallAuditLog{
		McpToolCallAuditLogBase: r.McpToolCallAuditLogBase,
		ID:                      r.ID,
		CreatedAt:               r.CreatedAt,
	}
}

// PreferenceAuditLogRecord is the GORM record struct for the preference_audit_log table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain PreferenceAuditLogBase struct.
// Note: PreferenceAuditLogBase contains ChangedAt which is used instead of CreatedAt.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type PreferenceAuditLogRecord struct {
	gormmodel.GormModelUUID
	models.PreferenceAuditLogBase
}

// TableName implements the GORM TableName interface.
func (PreferenceAuditLogRecord) TableName() string {
	return "preference_audit_log"
}

// ToPreferenceAuditLog converts a PreferenceAuditLogRecord to a domain PreferenceAuditLog with all fields populated.
func (r *PreferenceAuditLogRecord) ToPreferenceAuditLog() *models.PreferenceAuditLog {
	return &models.PreferenceAuditLog{
		PreferenceAuditLogBase: r.PreferenceAuditLogBase,
		ID:                     r.ID,
	}
}

// ChatAuditLogRecord is the GORM record struct for the chat_audit_log table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain ChatAuditLogBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type ChatAuditLogRecord struct {
	gormmodel.GormModelUUID
	models.ChatAuditLogBase
}

// TableName implements the GORM TableName interface.
func (ChatAuditLogRecord) TableName() string {
	return "chat_audit_log"
}

// ToChatAuditLog converts a ChatAuditLogRecord to a domain ChatAuditLog with all fields populated.
func (r *ChatAuditLogRecord) ToChatAuditLog() *models.ChatAuditLog {
	return &models.ChatAuditLog{
		ChatAuditLogBase: r.ChatAuditLogBase,
		ID:               r.ID,
		CreatedAt:        r.CreatedAt,
	}
}

func (r *ChatAuditLogRecord) auditIDPtr() (*uuid.UUID, *time.Time) {
	return &r.ID, &r.CreatedAt
}

// AccessControlAuditLogRecord is the GORM record struct for the access_control_audit_log table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain AccessControlAuditLogBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type AccessControlAuditLogRecord struct {
	gormmodel.GormModelUUID
	models.AccessControlAuditLogBase
}

// TableName implements the GORM TableName interface.
func (AccessControlAuditLogRecord) TableName() string {
	return "access_control_audit_log"
}

// ToAccessControlAuditLog converts an AccessControlAuditLogRecord to a domain AccessControlAuditLog with all fields populated.
func (r *AccessControlAuditLogRecord) ToAccessControlAuditLog() *models.AccessControlAuditLog {
	return &models.AccessControlAuditLog{
		AccessControlAuditLogBase: r.AccessControlAuditLogBase,
		ID:                        r.ID,
		CreatedAt:                 r.CreatedAt,
	}
}

func (r *AccessControlAuditLogRecord) auditIDPtr() (*uuid.UUID, *time.Time) {
	return &r.ID, &r.CreatedAt
}
