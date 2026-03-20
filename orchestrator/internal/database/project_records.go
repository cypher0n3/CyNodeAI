// Package database provides GORM record structs for project-related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// ProjectRecord is the GORM record struct for the projects table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain ProjectBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type ProjectRecord struct {
	gormmodel.GormModelUUID
	models.ProjectBase
}

// TableName implements the GORM TableName interface.
func (ProjectRecord) TableName() string {
	return "projects"
}

// ToProject converts a ProjectRecord to a domain Project with all fields populated.
func (r *ProjectRecord) ToProject() *models.Project {
	return &models.Project{
		ProjectBase: models.ProjectBase{
			Slug:        r.ProjectBase.Slug,
			DisplayName: r.ProjectBase.DisplayName,
			Description: r.ProjectBase.Description,
			IsActive:    r.ProjectBase.IsActive,
			UpdatedBy:   r.ProjectBase.UpdatedBy,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// ProjectPlanRecord is the GORM record struct for the project_plans table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain ProjectPlanBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type ProjectPlanRecord struct {
	gormmodel.GormModelUUID
	models.ProjectPlanBase
}

// TableName implements the GORM TableName interface.
func (ProjectPlanRecord) TableName() string {
	return "project_plans"
}

// ToProjectPlan converts a ProjectPlanRecord to a domain ProjectPlan with all fields populated.
func (r *ProjectPlanRecord) ToProjectPlan() *models.ProjectPlan {
	return &models.ProjectPlan{
		ProjectPlanBase: models.ProjectPlanBase{
			ProjectID: r.ProjectPlanBase.ProjectID,
			State:     r.ProjectPlanBase.State,
			Archived:  r.ProjectPlanBase.Archived,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}
