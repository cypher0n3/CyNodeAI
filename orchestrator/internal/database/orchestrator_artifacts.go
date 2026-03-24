// Package database: CRUD for scope-partitioned orchestrator artifacts (artifacts table).
package database

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// ListOrchestratorArtifactsParams filters list/find (GET /v1/artifacts).
type ListOrchestratorArtifactsParams struct {
	ScopeLevel        string
	OwnerUserID       *uuid.UUID
	GroupID           *uuid.UUID
	ProjectID         *uuid.UUID
	CorrelationTaskID *uuid.UUID
	Limit             int
	Offset            int
}

// GetOrchestratorArtifactByID returns a non-deleted artifact by id or ErrNotFound.
func (db *DB) GetOrchestratorArtifactByID(ctx context.Context, id uuid.UUID) (*models.OrchestratorArtifact, error) {
	var rec OrchestratorArtifactRecord
	err := db.db.WithContext(ctx).Where("id = ?", id).First(&rec).Error
	if err != nil {
		return nil, wrapErr(err, "get orchestrator artifact by id")
	}
	return rec.ToOrchestratorArtifact(), nil
}

// GetOrchestratorArtifactByScopePartitionAndPath returns an artifact by partition key and path.
func (db *DB) GetOrchestratorArtifactByScopePartitionAndPath(ctx context.Context, partition, path string) (*models.OrchestratorArtifact, error) {
	var rec OrchestratorArtifactRecord
	err := db.db.WithContext(ctx).Where("scope_partition = ? AND path = ?", partition, path).First(&rec).Error
	if err != nil {
		return nil, wrapErr(err, "get orchestrator artifact by scope path")
	}
	return rec.ToOrchestratorArtifact(), nil
}

// CreateOrchestratorArtifact inserts a new artifact row (blob must already exist at StorageRef in S3).
func (db *DB) CreateOrchestratorArtifact(ctx context.Context, row *models.OrchestratorArtifact) (*models.OrchestratorArtifact, error) {
	if row == nil {
		return nil, errors.New("nil artifact")
	}
	id := row.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	rec := &OrchestratorArtifactRecord{
		GormModelUUID:            gormmodel.GormModelUUID{ID: id},
		OrchestratorArtifactBase: row.OrchestratorArtifactBase,
	}
	if err := db.db.WithContext(ctx).Create(rec).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return nil, ErrExists
		}
		return nil, wrapErr(err, "create orchestrator artifact")
	}
	return rec.ToOrchestratorArtifact(), nil
}

// UpdateOrchestratorArtifactMetadata updates size, content type, checksum, last_modified_by_job_id after blob write.
func (db *DB) UpdateOrchestratorArtifactMetadata(ctx context.Context, id uuid.UUID, sizeBytes *int64, contentType, checksum *string, lastModJob *uuid.UUID) error {
	updates := map[string]interface{}{}
	if sizeBytes != nil {
		updates["size_bytes"] = *sizeBytes
	}
	if contentType != nil {
		updates["content_type"] = *contentType
	}
	if checksum != nil {
		updates["checksum_sha256"] = *checksum
	}
	if lastModJob != nil {
		updates["last_modified_by_job_id"] = lastModJob
	}
	if len(updates) == 0 {
		return nil
	}
	res := db.db.WithContext(ctx).Model(&OrchestratorArtifactRecord{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return wrapErr(res.Error, "update orchestrator artifact metadata")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteOrchestratorArtifactByID soft-deletes the artifact row.
func (db *DB) DeleteOrchestratorArtifactByID(ctx context.Context, id uuid.UUID) error {
	res := db.db.WithContext(ctx).Where("id = ?", id).Delete(&OrchestratorArtifactRecord{})
	if res.Error != nil {
		return wrapErr(res.Error, "delete orchestrator artifact")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListOrchestratorArtifacts returns artifacts matching scope filters, newest first.
func (db *DB) ListOrchestratorArtifacts(ctx context.Context, p ListOrchestratorArtifactsParams) ([]*models.OrchestratorArtifact, error) {
	q := db.db.WithContext(ctx).Model(&OrchestratorArtifactRecord{})
	if p.ScopeLevel != "" {
		q = q.Where("scope_level = ?", p.ScopeLevel)
	}
	if p.OwnerUserID != nil {
		q = q.Where("owner_user_id = ?", *p.OwnerUserID)
	}
	if p.GroupID != nil {
		q = q.Where("group_id = ?", *p.GroupID)
	}
	if p.ProjectID != nil {
		q = q.Where("project_id = ?", *p.ProjectID)
	}
	if p.CorrelationTaskID != nil {
		q = q.Where("correlation_task_id = ?", *p.CorrelationTaskID)
	}
	limit := p.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}
	var recs []OrchestratorArtifactRecord
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&recs).Error
	if err != nil {
		return nil, wrapErr(err, "list orchestrator artifacts")
	}
	out := make([]*models.OrchestratorArtifact, 0, len(recs))
	for i := range recs {
		out = append(out, recs[i].ToOrchestratorArtifact())
	}
	return out, nil
}

// ListOrchestratorArtifactIDsMissingChecksum returns artifact ids with no checksum (for hash backfill job).
func (db *DB) ListOrchestratorArtifactIDsMissingChecksum(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var ids []uuid.UUID
	err := db.db.WithContext(ctx).Model(&OrchestratorArtifactRecord{}).
		Select("id").
		Where("checksum_sha256 IS NULL OR checksum_sha256 = ''").
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, wrapErr(err, "list artifacts missing checksum")
	}
	return ids, nil
}

// ListOrchestratorArtifactIDsCreatedBefore returns ids of artifacts older than cutoff (for stale cleanup job).
func (db *DB) ListOrchestratorArtifactIDsCreatedBefore(ctx context.Context, cutoff time.Time, limit int) ([]uuid.UUID, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var ids []uuid.UUID
	err := db.db.WithContext(ctx).Model(&OrchestratorArtifactRecord{}).
		Select("id").
		Where("created_at < ?", cutoff.UTC()).
		Order("created_at ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, wrapErr(err, "list artifacts created before cutoff")
	}
	return ids, nil
}

// DeleteVectorItemsForArtifact removes vector_items rows pointing at this artifact (best-effort if table missing).
func (db *DB) DeleteVectorItemsForArtifact(ctx context.Context, artifactID uuid.UUID) error {
	err := db.db.WithContext(ctx).Exec(
		`DELETE FROM vector_items WHERE source_type = ? AND source_ref = ?`,
		"artifact", artifactID.String(),
	).Error
	if err == nil {
		return nil
	}
	// Table may not exist until vector pipeline is enabled.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "vector_items") && (strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined table")) {
		return nil
	}
	return wrapErr(err, "delete vector items for artifact")
}
