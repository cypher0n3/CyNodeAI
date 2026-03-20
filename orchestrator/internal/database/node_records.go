// Package database provides PostgreSQL database operations via GORM.
// This file defines GORM record structs for node-related tables.
// See docs/tech_specs/go_sql_database_standards.md (CYNAI.STANDS.GormModelStructure).
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

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
		NodeBase: models.NodeBase{
			NodeSlug:             r.NodeBase.NodeSlug,
			Status:               r.NodeBase.Status,
			CapabilityHash:       r.NodeBase.CapabilityHash,
			ConfigVersion:        r.NodeBase.ConfigVersion,
			WorkerAPITargetURL:   r.NodeBase.WorkerAPITargetURL,
			WorkerAPIBearerToken: r.NodeBase.WorkerAPIBearerToken,
			ConfigAckAt:          r.NodeBase.ConfigAckAt,
			ConfigAckStatus:      r.NodeBase.ConfigAckStatus,
			ConfigAckError:       r.NodeBase.ConfigAckError,
			LastSeenAt:           r.NodeBase.LastSeenAt,
			LastCapabilityAt:     r.NodeBase.LastCapabilityAt,
			Metadata:             r.NodeBase.Metadata,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
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
		NodeCapabilityBase: models.NodeCapabilityBase{
			NodeID:             r.NodeCapabilityBase.NodeID,
			ReportedAt:         r.NodeCapabilityBase.ReportedAt,
			CapabilitySnapshot: r.NodeCapabilityBase.CapabilitySnapshot,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}
