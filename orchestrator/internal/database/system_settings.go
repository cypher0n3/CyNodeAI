// Package database: system_settings table and MCP system_setting.* tools (docs/tech_specs/mcp_tools/system_setting_tools.md).
package database

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// MaxSystemSettingListLimit caps list results for system_setting.list.
const MaxSystemSettingListLimit = 100

// GetSystemSetting returns one setting by key or ErrNotFound.
func (db *DB) GetSystemSetting(ctx context.Context, key string) (*models.SystemSetting, error) {
	var rec SystemSettingRecord
	err := db.db.WithContext(ctx).Where("key = ?", key).First(&rec).Error
	if err != nil {
		return nil, wrapErr(err, "get system setting")
	}
	return rec.ToSystemSetting(), nil
}

// ListSystemSettings lists settings ordered by key with optional key_prefix and pagination (cursor = numeric offset).
func (db *DB) ListSystemSettings(ctx context.Context, keyPrefix string, limit int, cursor string) ([]*models.SystemSetting, string, error) {
	if limit <= 0 || limit > MaxSystemSettingListLimit {
		limit = MaxSystemSettingListLimit
	}
	offset := 0
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil && n >= 0 {
			offset = n
		}
	}
	q := db.db.WithContext(ctx).Model(&SystemSettingRecord{})
	if keyPrefix != "" {
		q = q.Where("key LIKE ?", keyPrefix+"%")
	}
	var records []SystemSettingRecord
	err := q.Order("key ASC").Offset(offset).Limit(limit + 1).Find(&records).Error
	if err != nil {
		return nil, "", wrapErr(err, "list system settings")
	}
	out := make([]*models.SystemSetting, 0, len(records))
	for i := range records {
		out = append(out, records[i].ToSystemSetting())
	}
	nextCursor := ""
	if len(out) > limit {
		out = out[:limit]
		nextCursor = strconv.Itoa(offset + limit)
	}
	return out, nextCursor, nil
}

// CreateSystemSetting inserts a row. Returns ErrExists if key is already present.
func (db *DB) CreateSystemSetting(ctx context.Context, key, value, valueType string, reason, updatedBy *string) (*models.SystemSetting, error) {
	_, err := db.GetSystemSetting(ctx, key)
	if err == nil {
		return nil, ErrExists
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	var valPtr *string
	if value != "" {
		valPtr = &value
	}
	rec := SystemSettingRecord{
		Key: key, Value: valPtr, ValueType: valueType, Version: 1,
		UpdatedAt: now, UpdatedBy: updatedBy,
	}
	if err := db.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return nil, wrapErr(err, "create system setting")
	}
	_ = reason
	return rec.ToSystemSetting(), nil
}

// UpdateSystemSetting updates a row. Returns ErrNotFound, ErrConflict when expected_version mismatches.
func (db *DB) UpdateSystemSetting(ctx context.Context, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.SystemSetting, error) {
	ent, err := db.GetSystemSetting(ctx, key)
	if err != nil {
		return nil, err
	}
	if expectedVersion != nil && ent.Version != *expectedVersion {
		return nil, ErrConflict
	}
	now := time.Now().UTC()
	var valPtr *string
	if value != "" {
		valPtr = &value
	}
	var rec SystemSettingRecord
	if err := db.db.WithContext(ctx).Where("key = ?", key).First(&rec).Error; err != nil {
		return nil, wrapErr(err, "load system setting for update")
	}
	updates := map[string]interface{}{
		"value":      valPtr,
		"value_type": valueType,
		"version":    rec.Version + 1,
		"updated_at": now,
		"updated_by": updatedBy,
	}
	if err := db.db.WithContext(ctx).Model(&SystemSettingRecord{}).Where("key = ?", key).Updates(updates).Error; err != nil {
		return nil, wrapErr(err, "update system setting")
	}
	_ = reason
	if err := db.db.WithContext(ctx).Where("key = ?", key).First(&rec).Error; err != nil {
		return nil, wrapErr(err, "reload system setting")
	}
	return rec.ToSystemSetting(), nil
}

// DeleteSystemSetting deletes a row by key.
func (db *DB) DeleteSystemSetting(ctx context.Context, key string, expectedVersion *int, reason *string) error {
	ent, err := db.GetSystemSetting(ctx, key)
	if err != nil {
		return err
	}
	if expectedVersion != nil && ent.Version != *expectedVersion {
		return ErrConflict
	}
	res := db.db.WithContext(ctx).Where("key = ?", key).Delete(&SystemSettingRecord{})
	if res.Error != nil {
		return wrapErr(res.Error, "delete system setting")
	}
	_ = reason
	return nil
}
