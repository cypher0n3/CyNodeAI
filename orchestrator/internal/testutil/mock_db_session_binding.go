package testutil

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// UpsertSessionBinding inserts or updates a session binding keyed by lineage hash.
func (m *MockDB) UpsertSessionBinding(_ context.Context, lineage models.SessionBindingLineage, serviceID, state string) (*models.SessionBinding, error) {
	return runWithLock(m, true, func() (*models.SessionBinding, error) {
		if m.ForceError != nil {
			return nil, m.ForceError
		}
		key := models.DeriveSessionBindingKey(lineage)
		now := time.Now().UTC()
		if existing, ok := m.SessionBindingsByKey[key]; ok {
			existing.ServiceID = serviceID
			existing.State = state
			existing.UserID = lineage.UserID
			existing.SessionID = lineage.SessionID
			existing.ThreadID = lineage.ThreadID
			existing.UpdatedAt = now
			if state == models.SessionBindingStateActive {
				existing.LastActivityAt = &now
			} else {
				existing.LastActivityAt = nil
			}
			return existing, nil
		}
		id := uuid.New()
		var lastAct *time.Time
		if state == models.SessionBindingStateActive {
			lastAct = &now
		}
		out := &models.SessionBinding{
			SessionBindingBase: models.SessionBindingBase{
				BindingKey:     key,
				UserID:         lineage.UserID,
				SessionID:      lineage.SessionID,
				ThreadID:       lineage.ThreadID,
				ServiceID:      serviceID,
				State:          state,
				LastActivityAt: lastAct,
			},
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.SessionBindingsByKey[key] = out
		return out, nil
	})
}

// GetSessionBindingByKey returns a binding by opaque key.
func (m *MockDB) GetSessionBindingByKey(_ context.Context, bindingKey string) (*models.SessionBinding, error) {
	return runWithLock(m, false, func() (*models.SessionBinding, error) {
		return getByKey(m.SessionBindingsByKey, bindingKey)
	})
}

// ListActiveBindingsForUser returns active bindings for the user, sorted by binding key for stability.
func (m *MockDB) ListActiveBindingsForUser(_ context.Context, userID uuid.UUID) ([]*models.SessionBinding, error) {
	return runWithLock(m, false, func() ([]*models.SessionBinding, error) {
		if m.ForceError != nil {
			return nil, m.ForceError
		}
		var keys []string
		for k := range m.SessionBindingsByKey {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out []*models.SessionBinding
		for _, k := range keys {
			b := m.SessionBindingsByKey[k]
			if b.UserID == userID && b.State == models.SessionBindingStateActive {
				out = append(out, b)
			}
		}
		return out, nil
	})
}

// ListAllActiveSessionBindings returns all active bindings (all users), sorted by binding key.
func (m *MockDB) ListAllActiveSessionBindings(_ context.Context) ([]*models.SessionBinding, error) {
	return runWithLock(m, false, func() ([]*models.SessionBinding, error) {
		if m.ForceError != nil {
			return nil, m.ForceError
		}
		var keys []string
		for k := range m.SessionBindingsByKey {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out []*models.SessionBinding
		for _, k := range keys {
			b := m.SessionBindingsByKey[k]
			if b.State == models.SessionBindingStateActive {
				out = append(out, b)
			}
		}
		return out, nil
	})
}

// TouchActiveSessionBindingsForUser sets last_activity_at on active bindings for the user.
func (m *MockDB) TouchActiveSessionBindingsForUser(_ context.Context, userID uuid.UUID, at time.Time) error {
	return runWithWLockErr(m, func() error {
		if m.ForceError != nil {
			return m.ForceError
		}
		for _, b := range m.SessionBindingsByKey {
			if b.UserID == userID && b.State == models.SessionBindingStateActive {
				b.LastActivityAt = &at
				b.UpdatedAt = at
			}
		}
		return nil
	})
}

// TouchSessionBindingByKey sets last_activity_at on the active binding for binding_key.
func (m *MockDB) TouchSessionBindingByKey(_ context.Context, bindingKey string, at time.Time) error {
	return runWithWLockErr(m, func() error {
		if m.ForceError != nil {
			return m.ForceError
		}
		b, ok := m.SessionBindingsByKey[bindingKey]
		if !ok || b.State != models.SessionBindingStateActive {
			return nil
		}
		b.LastActivityAt = &at
		b.UpdatedAt = at
		return nil
	})
}
