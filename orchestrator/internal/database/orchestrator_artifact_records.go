// Package database: GORM record for scope-partitioned orchestrator artifacts (S3-backed).
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// OrchestratorArtifactRecord is the GORM record for the artifacts table.
type OrchestratorArtifactRecord struct {
	gormmodel.GormModelUUID
	models.OrchestratorArtifactBase
}

// TableName implements gorm.Tabler.
func (OrchestratorArtifactRecord) TableName() string {
	return "artifacts"
}

// ToOrchestratorArtifact maps the record to the domain type.
func (r *OrchestratorArtifactRecord) ToOrchestratorArtifact() *models.OrchestratorArtifact {
	if r == nil {
		return nil
	}
	return &models.OrchestratorArtifact{
		OrchestratorArtifactBase: r.OrchestratorArtifactBase,
		ID:                       r.ID,
		CreatedAt:                r.CreatedAt,
		UpdatedAt:                r.UpdatedAt,
	}
}
