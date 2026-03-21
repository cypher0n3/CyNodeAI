package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// AcquireTaskWorkflowLease acquires or idempotently re-acquires the task workflow lease.
// If the lease is held by another holder (or expired and we are re-acquiring), returns ErrLeaseHeld when
// a different holder has a non-expired lease. When the same holder re-requests with the same lease_id, returns
// the existing lease (idempotent). When the lease row is missing or expired, creates or updates the row.
// Per REQ-ORCHES-0145, REQ-ORCHES-0146 and CYNAI.ORCHES.TaskWorkflowLeaseLifecycle.
func (db *DB) AcquireTaskWorkflowLease(ctx context.Context, taskID, leaseID uuid.UUID, holderID string, expiresAt time.Time) (*models.TaskWorkflowLease, error) {
	now := time.Now().UTC()
	var record TaskWorkflowLeaseRecord
	err := db.db.WithContext(ctx).Where("task_id = ?", taskID).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No existing lease: create.
			record = TaskWorkflowLeaseRecord{
				GormModelUUID: gormmodel.GormModelUUID{
					ID:        uuid.New(),
					CreatedAt: now,
					UpdatedAt: now,
				},
				TaskWorkflowLeaseBase: models.TaskWorkflowLeaseBase{
					TaskID:    taskID,
					LeaseID:   leaseID,
					HolderID:  &holderID,
					ExpiresAt: &expiresAt,
				},
			}
			if err := db.db.WithContext(ctx).Create(&record).Error; err != nil {
				return nil, wrapErr(err, "acquire task workflow lease create")
			}
			return record.ToTaskWorkflowLease(), nil
		}
		return nil, wrapErr(err, "acquire task workflow lease get")
	}
	// Existing row: treat as released if holder_id is nil (explicit release) or expired.
	if record.HolderID == nil || (record.ExpiresAt != nil && record.ExpiresAt.Before(now)) {
		record.LeaseID = leaseID
		h := holderID
		record.HolderID = &h
		record.ExpiresAt = &expiresAt
		record.UpdatedAt = now
		if err := db.db.WithContext(ctx).Save(&record).Error; err != nil {
			return nil, wrapErr(err, "acquire task workflow lease renew")
		}
		return record.ToTaskWorkflowLease(), nil
	}
	// Non-expired and held: same holder + same lease_id -> idempotent success.
	if *record.HolderID == holderID && record.LeaseID == leaseID {
		return record.ToTaskWorkflowLease(), nil
	}
	// Different holder or different lease_id: conflict.
	return nil, ErrLeaseHeld
}

// ReleaseTaskWorkflowLease releases the lease for the task when lease_id matches. Idempotent when already released.
func (db *DB) ReleaseTaskWorkflowLease(ctx context.Context, taskID, leaseID uuid.UUID) error {
	res := db.db.WithContext(ctx).Model(&TaskWorkflowLeaseRecord{}).
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
	return getDomainWhere(db, ctx, "task_id", taskID, "get task workflow lease", (*TaskWorkflowLeaseRecord).ToTaskWorkflowLease)
}

// GetWorkflowCheckpoint returns the current checkpoint for the task, or ErrNotFound.
func (db *DB) GetWorkflowCheckpoint(ctx context.Context, taskID uuid.UUID) (*models.WorkflowCheckpoint, error) {
	return getDomainWhere(db, ctx, "task_id", taskID, "get workflow checkpoint", (*WorkflowCheckpointRecord).ToWorkflowCheckpoint)
}

// UpsertWorkflowCheckpoint inserts or updates the single checkpoint row for the task (unique on task_id).
func (db *DB) UpsertWorkflowCheckpoint(ctx context.Context, cp *models.WorkflowCheckpoint) error {
	now := time.Now().UTC()
	var existing WorkflowCheckpointRecord
	err := db.db.WithContext(ctx).Where("task_id = ?", cp.TaskID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			record := &WorkflowCheckpointRecord{
				GormModelUUID: gormmodel.GormModelUUID{
					ID:        cp.ID,
					UpdatedAt: now,
				},
				WorkflowCheckpointBase: models.WorkflowCheckpointBase{
					TaskID:     cp.TaskID,
					State:      cp.State,
					LastNodeID: cp.LastNodeID,
				},
			}
			if record.ID == uuid.Nil {
				record.ID = uuid.New()
			}
			if err := db.db.WithContext(ctx).Create(record).Error; err != nil {
				return wrapErr(err, "upsert workflow checkpoint create")
			}
			cp.ID = record.ID
			cp.UpdatedAt = record.UpdatedAt
			return nil
		}
		return wrapErr(err, "upsert workflow checkpoint get")
	}
	if err := db.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"state":        cp.State,
		"last_node_id": cp.LastNodeID,
		"updated_at":   now,
	}).Error; err != nil {
		return wrapErr(err, "upsert workflow checkpoint update")
	}
	cp.ID = existing.ID
	cp.UpdatedAt = now
	return nil
}
