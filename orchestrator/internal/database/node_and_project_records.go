// Package database provides PostgreSQL database operations via GORM.
// This file defines GORM record structs for node and project tables.
// See docs/tech_specs/go_sql_database_standards.md (CYNAI.STANDS.GormModelStructure).
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// --- Nodes ---

// NodeRecord is the GORM record struct for the nodes table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain NodeBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type NodeRecord struct {
	gormmodel.GormModelUUID
	models.NodeBase
}

// TableName implements the GORM TableName interface.
func (NodeRecord) TableName() string {
	return "nodes"
}

// ToNode converts a NodeRecord to a domain Node with all fields populated.
func (r *NodeRecord) ToNode() *models.Node {
	return &models.Node{
		NodeBase:  r.NodeBase,
		ID:        r.ID,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// NodeCapabilityRecord is the GORM record struct for the node_capabilities table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain NodeCapabilityBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type NodeCapabilityRecord struct {
	gormmodel.GormModelUUID
	models.NodeCapabilityBase
}

// TableName implements the GORM TableName interface.
func (NodeCapabilityRecord) TableName() string {
	return "node_capabilities"
}

// ToNodeCapability converts a NodeCapabilityRecord to a domain NodeCapability with all fields populated.
func (r *NodeCapabilityRecord) ToNodeCapability() *models.NodeCapability {
	return &models.NodeCapability{
		NodeCapabilityBase: r.NodeCapabilityBase,
		ID:                 r.ID,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

// --- Projects ---

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
		ProjectBase: r.ProjectBase,
		ID:          r.ID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
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
		ProjectPlanBase: r.ProjectPlanBase,
		ID:              r.ID,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}
