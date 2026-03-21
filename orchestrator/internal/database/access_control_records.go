// Package database provides GORM record structs for access control-related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// AccessControlRuleRecord is the GORM record struct for the access_control_rules table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain AccessControlRuleBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type AccessControlRuleRecord struct {
	gormmodel.GormModelUUID
	models.AccessControlRuleBase
}

// TableName implements the GORM TableName interface.
func (AccessControlRuleRecord) TableName() string {
	return "access_control_rules"
}

// ToAccessControlRule converts an AccessControlRuleRecord to a domain AccessControlRule with all fields populated.
func (r *AccessControlRuleRecord) ToAccessControlRule() *models.AccessControlRule {
	return &models.AccessControlRule{
		AccessControlRuleBase: models.AccessControlRuleBase{
			SubjectType:     r.SubjectType,
			SubjectID:       r.SubjectID,
			Action:          r.Action,
			ResourceType:    r.ResourceType,
			ResourcePattern: r.ResourcePattern,
			Effect:          r.Effect,
			Priority:        r.Priority,
			Conditions:      r.Conditions,
			UpdatedBy:       r.UpdatedBy,
		},
		ID:        r.ID,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
