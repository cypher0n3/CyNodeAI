// Package database provides GORM record structs for preference and related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// PreferenceEntryRecord is the GORM record struct for the preference_entries table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain PreferenceEntryBase struct.
// Note: PreferenceEntry domain type uses UpdatedAt from GormModelUUID.UpdatedAt.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type PreferenceEntryRecord struct {
	gormmodel.GormModelUUID
	models.PreferenceEntryBase
}

// TableName implements the GORM TableName interface.
func (PreferenceEntryRecord) TableName() string {
	return "preference_entries"
}

// ToPreferenceEntry converts a PreferenceEntryRecord to a domain PreferenceEntry with all fields populated.
func (r *PreferenceEntryRecord) ToPreferenceEntry() *models.PreferenceEntry {
	return &models.PreferenceEntry{
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: r.PreferenceEntryBase.ScopeType,
			ScopeID:   r.PreferenceEntryBase.ScopeID,
			Key:       r.PreferenceEntryBase.Key,
			Value:     r.PreferenceEntryBase.Value,
			ValueType: r.PreferenceEntryBase.ValueType,
			Version:   r.PreferenceEntryBase.Version,
			UpdatedBy: r.PreferenceEntryBase.UpdatedBy,
		},
		ID:        r.GormModelUUID.ID,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}
