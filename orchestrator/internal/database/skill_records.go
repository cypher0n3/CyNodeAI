// Package database provides GORM record structs for skill-related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// SkillRecord is the GORM record struct for the skills table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain SkillBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type SkillRecord struct {
	gormmodel.GormModelUUID
	models.SkillBase
}

// TableName implements the GORM TableName interface.
func (SkillRecord) TableName() string {
	return "skills"
}

// ToSkill converts a SkillRecord to a domain Skill with all fields populated.
func (r *SkillRecord) ToSkill() *models.Skill {
	return &models.Skill{
		SkillBase: models.SkillBase{
			Name:     r.Name,
			Content:  r.Content,
			Scope:    r.Scope,
			OwnerID:  r.OwnerID,
			IsSystem: r.IsSystem,
		},
		ID:        r.ID,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
