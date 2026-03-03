package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// AcquireTaskWorkflowLease acquires or idempotently re-acquires the task workflow lease.
// If the lease is held by another holder (or expired and we are re-acquiring), returns ErrLeaseHeld when
// a different holder has a non-expired lease. When the same holder re-requests with the same lease_id, returns
// the existing lease (idempotent). When the lease row is missing or expired, creates or updates the row.
// Per REQ-ORCHES-0145, REQ-ORCHES-0146 and CYNAI.ORCHES.TaskWorkflowLeaseLifecycle.
func (db *DB) AcquireTaskWorkflowLease(ctx context.Context, taskID, leaseID uuid.UUID, holderID string, expiresAt time.Time) (*models.TaskWorkflowLease, error) {
	now := time.Now().UTC()
	var row models.TaskWorkflowLease
	err := db.db.WithContext(ctx).Where("task_id = ?", taskID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No existing lease: create.
			row = models.TaskWorkflowLease{
				ID:        uuid.New(),
				TaskID:    taskID,
				LeaseID:   leaseID,
				HolderID:  &holderID,
				ExpiresAt: &expiresAt,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := db.db.WithContext(ctx).Create(&row).Error; err != nil {
				return nil, wrapErr(err, "acquire task workflow lease create")
			}
			return &row, nil
		}
		return nil, wrapErr(err, "acquire task workflow lease get")
	}
	// Existing row: treat as released if holder_id is nil (explicit release) or expired.
	if row.HolderID == nil || (row.ExpiresAt != nil && row.ExpiresAt.Before(now)) {
		row.LeaseID = leaseID
		h := holderID
		row.HolderID = &h
		row.ExpiresAt = &expiresAt
		row.UpdatedAt = now
		if err := db.db.WithContext(ctx).Save(&row).Error; err != nil {
			return nil, wrapErr(err, "acquire task workflow lease renew")
		}
		return &row, nil
	}
	// Non-expired and held: same holder + same lease_id -> idempotent success.
	if *row.HolderID == holderID && row.LeaseID == leaseID {
		return &row, nil
	}
	// Different holder or different lease_id: conflict.
	return nil, ErrLeaseHeld
}

// ReleaseTaskWorkflowLease releases the lease for the task when lease_id matches. Idempotent when already released.
func (db *DB) ReleaseTaskWorkflowLease(ctx context.Context, taskID, leaseID uuid.UUID) error {
	res := db.db.WithContext(ctx).Model(&models.TaskWorkflowLease{}).
		Where("task_id = ? AND lease_id = ?", taskID, leaseID).
		Updates(map[string]interface{}{
			"holder_id":  nil,
			"expires_at": nil,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return wrapErr(res.Error, "release task workflow lease")
	}
	return nil
}

// GetTaskWorkflowLease returns the task workflow lease row if any, or ErrNotFound.
func (db *DB) GetTaskWorkflowLease(ctx context.Context, taskID uuid.UUID) (*models.TaskWorkflowLease, error) {
	return getWhere[models.TaskWorkflowLease](db, ctx, "task_id", taskID, "get task workflow lease")
}

// GetWorkflowCheckpoint returns the current checkpoint for the task, or ErrNotFound.
func (db *DB) GetWorkflowCheckpoint(ctx context.Context, taskID uuid.UUID) (*models.WorkflowCheckpoint, error) {
	return getWhere[models.WorkflowCheckpoint](db, ctx, "task_id", taskID, "get workflow checkpoint")
}

// UpsertWorkflowCheckpoint inserts or updates the single checkpoint row for the task (unique on task_id).
func (db *DB) UpsertWorkflowCheckpoint(ctx context.Context, cp *models.WorkflowCheckpoint) error {
	now := time.Now().UTC()
	cp.UpdatedAt = now
	var existing models.WorkflowCheckpoint
	err := db.db.WithContext(ctx).Where("task_id = ?", cp.TaskID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if cp.ID == uuid.Nil {
				cp.ID = uuid.New()
			}
			return wrapErr(db.db.WithContext(ctx).Create(cp).Error, "upsert workflow checkpoint create")
		}
		return wrapErr(err, "upsert workflow checkpoint get")
	}
	return wrapErr(db.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"state":        cp.State,
		"last_node_id": cp.LastNodeID,
		"updated_at":   now,
	}).Error, "upsert workflow checkpoint update")
}
