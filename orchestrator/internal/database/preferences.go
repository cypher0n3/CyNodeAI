// Package database provides preference storage per docs/tech_specs/user_preferences.md and docs/tech_specs/mcp_tools/preference_tools.md (P2-03).
package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// MaxPreferenceListLimit caps list results (mcp_tools/preference_tools: size-limited).
const MaxPreferenceListLimit = 100

// GetPreference returns the preference entry for the given scope and key, or ErrNotFound.
func (db *DB) GetPreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key string) (*models.PreferenceEntry, error) {
	var ent models.PreferenceEntry
	q := db.db.WithContext(ctx).Where("scope_type = ? AND key = ?", scopeType, key)
	if scopeID == nil {
		q = q.Where("scope_id IS NULL")
	} else {
		q = q.Where("scope_id = ?", *scopeID)
	}
	err := q.First(&ent).Error
	if err != nil {
		return nil, wrapErr(err, "get preference")
	}
	return &ent, nil
}

// ListPreferences returns preference entries for the scope, optionally filtered by key prefix, with pagination.
// limit is capped at MaxPreferenceListLimit. cursor is an optional offset (numeric string); nextCursor is the next offset or empty.
func (db *DB) ListPreferences(ctx context.Context, scopeType string, scopeID *uuid.UUID, keyPrefix string, limit int, cursor string) ([]*models.PreferenceEntry, string, error) {
	if limit <= 0 || limit > MaxPreferenceListLimit {
		limit = MaxPreferenceListLimit
	}
	offset := 0
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil && n >= 0 {
			offset = n
		}
	}
	q := db.db.WithContext(ctx).Model(&models.PreferenceEntry{}).Where("scope_type = ?", scopeType)
	if scopeID == nil {
		q = q.Where("scope_id IS NULL")
	} else {
		q = q.Where("scope_id = ?", *scopeID)
	}
	if keyPrefix != "" {
		q = q.Where("key LIKE ?", keyPrefix+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, "", wrapErr(err, "count preferences")
	}
	var entries []*models.PreferenceEntry
	err := q.Order("key ASC").Offset(offset).Limit(limit + 1).Find(&entries).Error
	if err != nil {
		return nil, "", wrapErr(err, "list preferences")
	}
	nextCursor := ""
	if len(entries) > limit {
		entries = entries[:limit]
		nextCursor = strconv.Itoa(offset + limit)
	} else if offset+len(entries) < int(total) {
		nextCursor = strconv.Itoa(offset + len(entries))
	}
	return entries, nextCursor, nil
}

type prefScope struct {
	scopeType string
	scopeID   *uuid.UUID
}

func (db *DB) effectiveScopesForTask(ctx context.Context, taskID uuid.UUID) ([]prefScope, error) {
	task, err := db.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	scopes := []prefScope{{scopeType: "system", scopeID: nil}}
	if task.CreatedBy != nil {
		scopes = append(scopes, prefScope{scopeType: "user", scopeID: task.CreatedBy})
	}
	if task.ProjectID != nil {
		scopes = append(scopes, prefScope{scopeType: "project", scopeID: task.ProjectID})
	}
	scopes = append(scopes, prefScope{scopeType: "task", scopeID: &taskID})
	return scopes, nil
}

// GetEffectivePreferencesForTask computes effective preferences for a task (task > project > user > system; group skipped when no membership data).
// Returns a map key -> JSON value (parsed). Per user_preferences.md resolution: collect by scope precedence, then fold.
func (db *DB) GetEffectivePreferencesForTask(ctx context.Context, taskID uuid.UUID) (map[string]interface{}, error) {
	scopes, err := db.effectiveScopesForTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	effective := make(map[string]interface{})
	for _, s := range scopes {
		entries, _, err := db.ListPreferences(ctx, s.scopeType, s.scopeID, "", MaxPreferenceListLimit, "")
		if err != nil {
			return nil, fmt.Errorf("list preferences %s: %w", s.scopeType, err)
		}
		for _, e := range entries {
			var v interface{}
			if e.Value != nil && *e.Value != "" {
				if err := json.Unmarshal([]byte(*e.Value), &v); err != nil {
					continue
				}
			}
			effective[e.Key] = v
		}
	}
	return effective, nil
}

// CreatePreference creates a preference entry. Returns ErrExists if (scope_type, scope_id, key) already exists.
// value should be JSON-encoded; valueType is e.g. string, number, boolean, object, array. Per mcp_tools/preference_tools.md.
func (db *DB) CreatePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, reason, updatedBy *string) (*models.PreferenceEntry, error) {
	existing, err := db.GetPreference(ctx, scopeType, scopeID, key)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrExists
	}
	now := time.Now().UTC()
	var valPtr *string
	if value != "" {
		valPtr = &value
	}
	ent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: scopeType,
		ScopeID:   scopeID,
		Key:       key,
		Value:     valPtr,
		ValueType: valueType,
		Version:   1,
		UpdatedAt: now,
		UpdatedBy: updatedBy,
	}
	if err := db.db.WithContext(ctx).Create(ent).Error; err != nil {
		return nil, wrapErr(err, "create preference")
	}
	// Optionally write to preference_audit_log (reason/updatedBy); schema supports it; minimal MVP we skip for now.
	_ = reason
	return ent, nil
}

// UpdatePreference updates a preference entry. Returns ErrNotFound if not found, ErrConflict if expected_version is set and does not match.
func (db *DB) UpdatePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.PreferenceEntry, error) {
	ent, err := db.GetPreference(ctx, scopeType, scopeID, key)
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
	updates := map[string]interface{}{
		"value":      valPtr,
		"value_type": valueType,
		"version":    ent.Version + 1,
		"updated_at": now,
		"updated_by": updatedBy,
	}
	if err := db.db.WithContext(ctx).Model(ent).Updates(updates).Error; err != nil {
		return nil, wrapErr(err, "update preference")
	}
	_ = reason
	ent.Value = valPtr
	ent.ValueType = valueType
	ent.Version++
	ent.UpdatedAt = now
	ent.UpdatedBy = updatedBy
	return ent, nil
}

// DeletePreference deletes a preference entry. Returns ErrNotFound if not found, ErrConflict if expected_version is set and does not match.
func (db *DB) DeletePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key string, expectedVersion *int, reason *string) error {
	ent, err := db.GetPreference(ctx, scopeType, scopeID, key)
	if err != nil {
		return err
	}
	if expectedVersion != nil && ent.Version != *expectedVersion {
		return ErrConflict
	}
	if err := db.db.WithContext(ctx).Delete(ent).Error; err != nil {
		return wrapErr(err, "delete preference")
	}
	_ = reason
	return nil
}

// ParsePreferenceValue parses the stored JSON value into a generic value. Returns nil for nil or empty value.
func ParsePreferenceValue(raw *string) (interface{}, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(*raw), &v); err != nil {
		return nil, err
	}
	return v, nil
}
