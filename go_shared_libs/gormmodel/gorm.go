// Package gormmodel provides shared GORM base types for CyNodeAI database models.
// See docs/tech_specs/go_sql_database_standards.md (CYNAI.STANDS.GormModelStructure).
package gormmodel

import (
	"time"

	"github.com/google/uuid"
	gorm "gorm.io/gorm"
)

// GormModelUUID is the shared base struct for all UUID-keyed tables.
// It provides ID (uuid primary key), CreatedAt, UpdatedAt, and DeletedAt (soft delete).
// Embed this in GORM record structs in the database package.
// Domain base structs should NOT include these fields; they are added by the record struct.
//
// Example:
//
//	type UserRecord struct {
//		GormModelUUID
//		models.User  // domain base struct without ID/timestamps
//	}
type GormModelUUID struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}
