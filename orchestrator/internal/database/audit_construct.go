// Helpers for constructing audit GORM records from domain models (keeps Create* handlers small).
package database

import "github.com/cypher0n3/cynodeai/orchestrator/internal/models"

const opCreateAccessControlAuditLog = "create access control audit log"

func accessControlAuditRecordFrom(rec *models.AccessControlAuditLog) *AccessControlAuditLogRecord {
	return &AccessControlAuditLogRecord{
		AccessControlAuditLogBase: models.AccessControlAuditLogBase{
			SubjectType:  rec.SubjectType,
			SubjectID:    rec.SubjectID,
			Action:       rec.Action,
			ResourceType: rec.ResourceType,
			Resource:     rec.Resource,
			Decision:     rec.Decision,
			Reason:       rec.Reason,
			TaskID:       rec.TaskID,
		},
	}
}
