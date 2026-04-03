// Package database: PMA session binding persistence (REQ-ORCHES-0188).
package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// UpsertSessionBinding inserts or updates a row keyed by DeriveSessionBindingKey(lineage).
func (db *DB) UpsertSessionBinding(ctx context.Context, lineage models.SessionBindingLineage, serviceID, state string) (*models.SessionBinding, error) {
	key := models.DeriveSessionBindingKey(lineage)
	var rec SessionBindingRecord
	err := db.db.WithContext(ctx).Where("binding_key = ?", key).First(&rec).Error
	if err == nil {
		now := time.Now().UTC()
		updates := map[string]interface{}{
			"service_id": serviceID,
			"state":      state,
			"user_id":    lineage.UserID,
			"session_id": lineage.SessionID,
			"thread_id":  lineage.ThreadID,
		}
		if state == models.SessionBindingStateActive {
			updates["last_activity_at"] = now
		}
		if uerr := db.db.WithContext(ctx).Model(&SessionBindingRecord{}).Where("id = ?", rec.ID).Updates(updates).Error; uerr != nil {
			return nil, wrapErr(uerr, "upsert session binding")
		}
		var updated SessionBindingRecord
		if ferr := db.db.WithContext(ctx).First(&updated, "id = ?", rec.ID).Error; ferr != nil {
			return nil, wrapErr(ferr, "reload session binding")
		}
		return updated.ToSessionBinding(), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wrapErr(err, "get session binding")
	}
	now := time.Now().UTC()
	var lastAct *time.Time
	if state == models.SessionBindingStateActive {
		lastAct = &now
	}
	rec = SessionBindingRecord{
		GormModelUUID: newGormModelUUIDNow(),
		SessionBindingBase: models.SessionBindingBase{
			BindingKey:     key,
			UserID:         lineage.UserID,
			SessionID:      lineage.SessionID,
			ThreadID:       lineage.ThreadID,
			ServiceID:      serviceID,
			State:          state,
			LastActivityAt: lastAct,
		},
	}
	if cerr := db.createRecord(ctx, &rec, "create session binding"); cerr != nil {
		return nil, cerr
	}
	return rec.ToSessionBinding(), nil
}

// GetSessionBindingByKey returns a binding by its opaque binding key.
func (db *DB) GetSessionBindingByKey(ctx context.Context, bindingKey string) (*models.SessionBinding, error) {
	return getDomainWhere(db, ctx, "binding_key", bindingKey, "get session binding by key", (*SessionBindingRecord).ToSessionBinding)
}

// ListActiveBindingsForUser returns bindings in active state for the user.
func (db *DB) ListActiveBindingsForUser(ctx context.Context, userID uuid.UUID) ([]*models.SessionBinding, error) {
	var rows []SessionBindingRecord
	err := db.db.WithContext(ctx).
		Where("user_id = ? AND state = ?", userID, models.SessionBindingStateActive).
		Find(&rows).Error
	if err != nil {
		return nil, wrapErr(err, "list active session bindings")
	}
	out := make([]*models.SessionBinding, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].ToSessionBinding())
	}
	return out, nil
}

// ListAllActiveSessionBindings returns every binding in active state (all users).
func (db *DB) ListAllActiveSessionBindings(ctx context.Context) ([]*models.SessionBinding, error) {
	var rows []SessionBindingRecord
	err := db.db.WithContext(ctx).
		Where("state = ?", models.SessionBindingStateActive).
		Find(&rows).Error
	if err != nil {
		return nil, wrapErr(err, "list all active session bindings")
	}
	out := make([]*models.SessionBinding, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].ToSessionBinding())
	}
	return out, nil
}

// TouchActiveSessionBindingsForUser sets last_activity_at (and updated_at) for all active bindings for the user.
func (db *DB) TouchActiveSessionBindingsForUser(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return wrapErr(db.db.WithContext(ctx).Model(&SessionBindingRecord{}).
		Where("user_id = ? AND state = ?", userID, models.SessionBindingStateActive).
		Updates(map[string]interface{}{
			"last_activity_at": at,
			"updated_at":       at,
		}).Error, "touch session binding activity")
}

// TouchSessionBindingByKey sets last_activity_at for the active binding with the given binding_key.
func (db *DB) TouchSessionBindingByKey(ctx context.Context, bindingKey string, at time.Time) error {
	return wrapErr(db.db.WithContext(ctx).Model(&SessionBindingRecord{}).
		Where("binding_key = ? AND state = ?", bindingKey, models.SessionBindingStateActive).
		Updates(map[string]interface{}{
			"last_activity_at": at,
			"updated_at":       at,
		}).Error, "touch session binding by key")
}
