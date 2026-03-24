package database

import (
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// SystemSettingRecord is the GORM model for the system_settings table (key PK).
// See docs/tech_specs/orchestrator_bootstrap.md#spec-cynai-schema-systemsettingstable.
type SystemSettingRecord struct {
	Key       string    `gorm:"column:key;primaryKey"`
	Value     *string   `gorm:"column:value;type:jsonb"`
	ValueType string    `gorm:"column:value_type"`
	Version   int       `gorm:"column:version"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	UpdatedBy *string   `gorm:"column:updated_by"`
}

// TableName implements gorm.Tabler.
func (SystemSettingRecord) TableName() string {
	return "system_settings"
}

// ToSystemSetting maps the record to the domain type.
func (r *SystemSettingRecord) ToSystemSetting() *models.SystemSetting {
	if r == nil {
		return nil
	}
	return &models.SystemSetting{
		Key:       r.Key,
		Value:     r.Value,
		ValueType: r.ValueType,
		Version:   r.Version,
		UpdatedAt: r.UpdatedAt,
		UpdatedBy: r.UpdatedBy,
	}
}
