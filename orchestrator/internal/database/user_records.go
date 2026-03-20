// Package database provides PostgreSQL database operations via GORM.
// This file defines GORM record structs for user and identity/auth-related tables.
// See docs/tech_specs/go_sql_database_standards.md (CYNAI.STANDS.GormModelStructure).
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// UserRecord is the GORM record struct for the users table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain UserBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type UserRecord struct {
	gormmodel.GormModelUUID
	models.UserBase
}

// TableName implements the GORM TableName interface.
func (UserRecord) TableName() string {
	return "users"
}

// ToUser converts a UserRecord to a domain User with all fields populated.
func (r *UserRecord) ToUser() *models.User {
	user := &models.User{
		UserBase: models.UserBase{
			Handle:         r.UserBase.Handle,
			Email:          r.UserBase.Email,
			IsActive:       r.UserBase.IsActive,
			ExternalSource: r.UserBase.ExternalSource,
			ExternalID:     r.UserBase.ExternalID,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
	return user
}

// FromUser creates a UserRecord from a domain User.
// Caller should set ID, CreatedAt, UpdatedAt if creating a new record.
func FromUser(user *models.User) *UserRecord {
	return &UserRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		UserBase: models.UserBase{
			Handle:         user.Handle,
			Email:          user.Email,
			IsActive:       user.IsActive,
			ExternalSource: user.ExternalSource,
			ExternalID:     user.ExternalID,
		},
	}
}

// PasswordCredentialRecord is the GORM record struct for the password_credentials table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain PasswordCredentialBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type PasswordCredentialRecord struct {
	gormmodel.GormModelUUID
	models.PasswordCredentialBase
}

// TableName implements the GORM TableName interface.
func (PasswordCredentialRecord) TableName() string {
	return "password_credentials"
}

// ToPasswordCredential converts a PasswordCredentialRecord to a domain PasswordCredential with all fields populated.
func (r *PasswordCredentialRecord) ToPasswordCredential() *models.PasswordCredential {
	return &models.PasswordCredential{
		PasswordCredentialBase: models.PasswordCredentialBase{
			UserID:       r.PasswordCredentialBase.UserID,
			PasswordHash: r.PasswordCredentialBase.PasswordHash,
			HashAlg:      r.PasswordCredentialBase.HashAlg,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// RefreshSessionRecord is the GORM record struct for the refresh_sessions table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain RefreshSessionBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type RefreshSessionRecord struct {
	gormmodel.GormModelUUID
	models.RefreshSessionBase
}

// TableName implements the GORM TableName interface.
func (RefreshSessionRecord) TableName() string {
	return "refresh_sessions"
}

// ToRefreshSession converts a RefreshSessionRecord to a domain RefreshSession with all fields populated.
func (r *RefreshSessionRecord) ToRefreshSession() *models.RefreshSession {
	return &models.RefreshSession{
		RefreshSessionBase: models.RefreshSessionBase{
			UserID:           r.RefreshSessionBase.UserID,
			RefreshTokenHash: r.RefreshSessionBase.RefreshTokenHash,
			RefreshTokenKID:  r.RefreshSessionBase.RefreshTokenKID,
			IsActive:         r.RefreshSessionBase.IsActive,
			ExpiresAt:        r.RefreshSessionBase.ExpiresAt,
			LastUsedAt:       r.RefreshSessionBase.LastUsedAt,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}

// AuthAuditLogRecord is the GORM record struct for the auth_audit_log table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain AuthAuditLogBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
// Note: AuthAuditLog only uses CreatedAt (not UpdatedAt), but GormModelUUID includes UpdatedAt for consistency.
type AuthAuditLogRecord struct {
	gormmodel.GormModelUUID
	models.AuthAuditLogBase
}

// TableName implements the GORM TableName interface.
func (AuthAuditLogRecord) TableName() string {
	return "auth_audit_log"
}

// ToAuthAuditLog converts an AuthAuditLogRecord to a domain AuthAuditLog with all fields populated.
func (r *AuthAuditLogRecord) ToAuthAuditLog() *models.AuthAuditLog {
	return &models.AuthAuditLog{
		AuthAuditLogBase: models.AuthAuditLogBase{
			UserID:        r.AuthAuditLogBase.UserID,
			EventType:     r.AuthAuditLogBase.EventType,
			Success:       r.AuthAuditLogBase.Success,
			SubjectHandle: r.AuthAuditLogBase.SubjectHandle,
			IPAddress:     r.AuthAuditLogBase.IPAddress,
			UserAgent:     r.AuthAuditLogBase.UserAgent,
			Reason:        r.AuthAuditLogBase.Reason,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
	}
}
