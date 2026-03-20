// Package database provides GORM record structs for sandbox image-related tables.
package database

import (
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// SandboxImageRecord is the GORM record struct for the sandbox_images table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain SandboxImageBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type SandboxImageRecord struct {
	gormmodel.GormModelUUID
	models.SandboxImageBase
}

// TableName implements the GORM TableName interface.
func (SandboxImageRecord) TableName() string {
	return "sandbox_images"
}

// ToSandboxImage converts a SandboxImageRecord to a domain SandboxImage with all fields populated.
func (r *SandboxImageRecord) ToSandboxImage() *models.SandboxImage {
	return &models.SandboxImage{
		SandboxImageBase: models.SandboxImageBase{
			Name:        r.SandboxImageBase.Name,
			Description: r.SandboxImageBase.Description,
			UpdatedBy:   r.SandboxImageBase.UpdatedBy,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// SandboxImageVersionRecord is the GORM record struct for the sandbox_image_versions table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain SandboxImageVersionBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type SandboxImageVersionRecord struct {
	gormmodel.GormModelUUID
	models.SandboxImageVersionBase
}

// TableName implements the GORM TableName interface.
func (SandboxImageVersionRecord) TableName() string {
	return "sandbox_image_versions"
}

// ToSandboxImageVersion converts a SandboxImageVersionRecord to a domain SandboxImageVersion with all fields populated.
func (r *SandboxImageVersionRecord) ToSandboxImageVersion() *models.SandboxImageVersion {
	return &models.SandboxImageVersion{
		SandboxImageVersionBase: models.SandboxImageVersionBase{
			SandboxImageID: r.SandboxImageVersionBase.SandboxImageID,
			Version:        r.SandboxImageVersionBase.Version,
			ImageRef:       r.SandboxImageVersionBase.ImageRef,
			ImageDigest:    r.SandboxImageVersionBase.ImageDigest,
			Capabilities:   r.SandboxImageVersionBase.Capabilities,
			IsAllowed:      r.SandboxImageVersionBase.IsAllowed,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// NodeSandboxImageAvailabilityRecord is the GORM record struct for the node_sandbox_image_availability table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain NodeSandboxImageAvailabilityBase struct.
// Note: The domain type uses LastCheckedAt which is stored in a separate column (not from GormModelUUID).
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type NodeSandboxImageAvailabilityRecord struct {
	gormmodel.GormModelUUID
	models.NodeSandboxImageAvailabilityBase
	LastCheckedAt time.Time `gorm:"column:last_checked_at" json:"last_checked_at"`
}

// TableName implements the GORM TableName interface.
func (NodeSandboxImageAvailabilityRecord) TableName() string {
	return "node_sandbox_image_availability"
}

// ToNodeSandboxImageAvailability converts a NodeSandboxImageAvailabilityRecord to a domain NodeSandboxImageAvailability with all fields populated.
func (r *NodeSandboxImageAvailabilityRecord) ToNodeSandboxImageAvailability() *models.NodeSandboxImageAvailability {
	return &models.NodeSandboxImageAvailability{
		NodeSandboxImageAvailabilityBase: models.NodeSandboxImageAvailabilityBase{
			NodeID:                r.NodeSandboxImageAvailabilityBase.NodeID,
			SandboxImageVersionID: r.NodeSandboxImageAvailabilityBase.SandboxImageVersionID,
			Status:                r.NodeSandboxImageAvailabilityBase.Status,
			Details:               r.NodeSandboxImageAvailabilityBase.Details,
		},
		ID:            r.GormModelUUID.ID,
		LastCheckedAt: r.LastCheckedAt,
	}
}
