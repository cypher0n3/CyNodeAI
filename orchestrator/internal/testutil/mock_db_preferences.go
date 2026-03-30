package testutil

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func matchPreferenceGet(e *models.PreferenceEntry, scopeType string, scopeID *uuid.UUID, key string) bool {
	if e.ScopeType != scopeType || e.Key != key {
		return false
	}
	if (scopeID == nil) != (e.ScopeID == nil) {
		return false
	}
	if scopeID != nil && e.ScopeID != nil && *e.ScopeID != *scopeID {
		return false
	}
	return true
}

// GetPreference returns a matching preference entry or ErrNotFound.
func (m *MockDB) GetPreference(_ context.Context, scopeType string, scopeID *uuid.UUID, key string) (*models.PreferenceEntry, error) {
	if m.GetPreferenceErr != nil {
		return nil, m.GetPreferenceErr
	}
	return runWithLock(m, false, func() (*models.PreferenceEntry, error) {
		for _, e := range m.PreferenceEntries {
			if matchPreferenceGet(e, scopeType, scopeID, key) {
				return e, nil
			}
		}
		return nil, database.ErrNotFound
	})
}

func matchPreferenceEntry(e *models.PreferenceEntry, scopeType string, scopeID *uuid.UUID, keyPrefix string) bool {
	if e.ScopeType != scopeType {
		return false
	}
	if (scopeID == nil) != (e.ScopeID == nil) {
		return false
	}
	if scopeID != nil && e.ScopeID != nil && *e.ScopeID != *scopeID {
		return false
	}
	if keyPrefix != "" && !strings.HasPrefix(e.Key, keyPrefix) {
		return false
	}
	return true
}

// ListPreferences returns entries for scope, optionally filtered by key prefix; cursor/limit simulated with offset.
func (m *MockDB) ListPreferences(_ context.Context, scopeType string, scopeID *uuid.UUID, keyPrefix string, limit int, cursor string) ([]*models.PreferenceEntry, string, error) {
	if m.ListPreferencesErr != nil {
		return nil, "", m.ListPreferencesErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return nil, "", m.ForceError
	}
	if limit <= 0 || limit > database.MaxPreferenceListLimit {
		limit = database.MaxPreferenceListLimit
	}
	offset := 0
	if cursor != "" {
		if n, err := parseInt(cursor); err == nil && n >= 0 {
			offset = n
		}
	}
	var out []*models.PreferenceEntry
	for _, e := range m.PreferenceEntries {
		if matchPreferenceEntry(e, scopeType, scopeID, keyPrefix) {
			out = append(out, e)
		}
	}
	if offset > len(out) {
		return nil, "", nil
	}
	out = out[offset:]
	nextCursor := ""
	if len(out) > limit {
		out = out[:limit]
		nextCursor = strconv.Itoa(offset + limit)
	}
	return out, nextCursor, nil
}

func parseInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	return n, err
}

// GetEffectivePreferencesForTask merges preferences by scope precedence (task > project > user > system); group skipped in mock.
func (m *MockDB) GetEffectivePreferencesForTask(ctx context.Context, taskID uuid.UUID) (map[string]interface{}, error) {
	if m.GetEffectivePreferencesForTaskErr != nil {
		return nil, m.GetEffectivePreferencesForTaskErr
	}
	task, err := m.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	scopes := []struct {
		t  string
		id *uuid.UUID
	}{{"system", nil}}
	if task.CreatedBy != nil {
		scopes = append(scopes, struct {
			t  string
			id *uuid.UUID
		}{"user", task.CreatedBy})
	}
	if task.ProjectID != nil {
		scopes = append(scopes, struct {
			t  string
			id *uuid.UUID
		}{"project", task.ProjectID})
	}
	scopes = append(scopes, struct {
		t  string
		id *uuid.UUID
	}{"task", &taskID})
	effective := make(map[string]interface{})
	for _, s := range scopes {
		entries, _, err := m.ListPreferences(ctx, s.t, s.id, "", database.MaxPreferenceListLimit, "")
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			v, _ := database.ParsePreferenceValue(e.Value)
			effective[e.Key] = v
		}
	}
	return effective, nil
}
