// Package database: explicit read grants for user-scoped orchestrator artifacts (cross-principal read).
package database

import (
	"context"
	"errors"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/google/uuid"
)

// ArtifactReadGrantRecord grants a user read access to an artifact they do not own (user scope).
type ArtifactReadGrantRecord struct {
	gormmodel.GormModelUUID
	ArtifactID    uuid.UUID `gorm:"column:artifact_id;uniqueIndex:idx_artifact_read_grant_pair;not null"`
	GranteeUserID uuid.UUID `gorm:"column:grantee_user_id;uniqueIndex:idx_artifact_read_grant_pair;not null"`
}

// TableName implements gorm.Tabler.
func (ArtifactReadGrantRecord) TableName() string {
	return "artifact_read_grants"
}

// GrantArtifactRead inserts an idempotent read grant (artifact_id, grantee_user_id).
func (db *DB) GrantArtifactRead(ctx context.Context, artifactID, granteeUserID uuid.UUID) error {
	if artifactID == uuid.Nil || granteeUserID == uuid.Nil {
		return errors.New("artifact_id and grantee_user_id required")
	}
	var rec ArtifactReadGrantRecord
	err := db.db.WithContext(ctx).
		Where(ArtifactReadGrantRecord{ArtifactID: artifactID, GranteeUserID: granteeUserID}).
		FirstOrCreate(&rec).Error
	return wrapErr(err, "grant artifact read")
}

// HasArtifactReadGrant returns true if grantee may read the artifact via explicit grant.
func (db *DB) HasArtifactReadGrant(ctx context.Context, artifactID, granteeUserID uuid.UUID) (bool, error) {
	if artifactID == uuid.Nil || granteeUserID == uuid.Nil {
		return false, nil
	}
	var n int64
	err := db.db.WithContext(ctx).Model(&ArtifactReadGrantRecord{}).
		Where("artifact_id = ? AND grantee_user_id = ?", artifactID, granteeUserID).
		Count(&n).Error
	if err != nil {
		return false, wrapErr(err, "has artifact read grant")
	}
	return n > 0, nil
}
