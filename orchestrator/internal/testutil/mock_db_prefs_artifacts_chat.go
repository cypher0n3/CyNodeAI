// Package testutil provides test utilities and mock implementations.
package testutil

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// CreatePreference creates a preference in the mock; returns database.ErrExists if key already exists.
func (m *MockDB) CreatePreference(_ context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, reason, updatedBy *string) (*models.PreferenceEntry, error) {
	_ = reason
	if m.CreatePreferenceErr != nil {
		return nil, m.CreatePreferenceErr
	}
	return runWithLock(m, true, func() (*models.PreferenceEntry, error) {
		for _, e := range m.PreferenceEntries {
			if matchPreferenceGet(e, scopeType, scopeID, key) {
				return nil, database.ErrExists
			}
		}
		valPtr := (*string)(nil)
		if value != "" {
			valPtr = &value
		}
		ent := &models.PreferenceEntry{
			PreferenceEntryBase: models.PreferenceEntryBase{
				ScopeType: scopeType,
				ScopeID:   scopeID,
				Key:       key,
				Value:     valPtr,
				ValueType: valueType,
				Version:   1,
				UpdatedBy: updatedBy,
			},
			ID:        uuid.New(),
			UpdatedAt: time.Now().UTC(),
		}
		m.PreferenceEntries = append(m.PreferenceEntries, ent)
		return ent, nil
	})
}

// UpdatePreference updates a preference in the mock; returns database.ErrNotFound or database.ErrConflict as appropriate.
func (m *MockDB) UpdatePreference(_ context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.PreferenceEntry, error) {
	_ = reason
	if m.UpdatePreferenceErr != nil {
		return nil, m.UpdatePreferenceErr
	}
	return runWithLock(m, true, func() (*models.PreferenceEntry, error) {
		for _, e := range m.PreferenceEntries {
			if !matchPreferenceGet(e, scopeType, scopeID, key) {
				continue
			}
			if expectedVersion != nil && e.Version != *expectedVersion {
				return nil, database.ErrConflict
			}
			valPtr := (*string)(nil)
			if value != "" {
				valPtr = &value
			}
			e.Value = valPtr
			e.ValueType = valueType
			e.Version++
			e.UpdatedAt = time.Now().UTC()
			e.UpdatedBy = updatedBy
			return e, nil
		}
		return nil, database.ErrNotFound
	})
}

// DeletePreference deletes a preference in the mock; returns database.ErrNotFound or database.ErrConflict as appropriate.
func (m *MockDB) DeletePreference(_ context.Context, scopeType string, scopeID *uuid.UUID, key string, expectedVersion *int, reason *string) error {
	_ = reason
	if m.DeletePreferenceErr != nil {
		return m.DeletePreferenceErr
	}
	return runWithWLockErr(m, func() error {
		for i, e := range m.PreferenceEntries {
			if matchPreferenceGet(e, scopeType, scopeID, key) {
				if expectedVersion != nil && e.Version != *expectedVersion {
					return database.ErrConflict
				}
				m.PreferenceEntries = append(m.PreferenceEntries[:i], m.PreferenceEntries[i+1:]...)
				return nil
			}
		}
		return database.ErrNotFound
	})
}

// GetSystemSetting returns a system setting by key or ErrNotFound.
func (m *MockDB) GetSystemSetting(_ context.Context, key string) (*models.SystemSetting, error) {
	return runWithLock(m, false, func() (*models.SystemSetting, error) {
		s, ok := m.SystemSettings[key]
		if !ok {
			return nil, database.ErrNotFound
		}
		return s, nil
	})
}

// ListSystemSettings lists mock system settings by key prefix with pagination.
func (m *MockDB) ListSystemSettings(_ context.Context, keyPrefix string, limit int, cursor string) ([]*models.SystemSetting, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return nil, "", m.ForceError
	}
	if limit <= 0 || limit > database.MaxSystemSettingListLimit {
		limit = database.MaxSystemSettingListLimit
	}
	offset := 0
	if cursor != "" {
		if n, err := parseInt(cursor); err == nil && n >= 0 {
			offset = n
		}
	}
	var keys []string
	for k := range m.SystemSettings {
		if keyPrefix == "" || strings.HasPrefix(k, keyPrefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if offset >= len(keys) {
		return nil, "", nil
	}
	slice := keys[offset:]
	nextCursor := ""
	if len(slice) > limit {
		slice = slice[:limit]
		nextCursor = strconv.Itoa(offset + limit)
	}
	out := make([]*models.SystemSetting, 0, len(slice))
	for _, k := range slice {
		out = append(out, m.SystemSettings[k])
	}
	return out, nextCursor, nil
}

// CreateSystemSetting creates a system setting or returns ErrExists.
func (m *MockDB) CreateSystemSetting(_ context.Context, key, value, valueType string, reason, updatedBy *string) (*models.SystemSetting, error) {
	_ = reason
	return runWithLock(m, true, func() (*models.SystemSetting, error) {
		if _, ok := m.SystemSettings[key]; ok {
			return nil, database.ErrExists
		}
		now := time.Now().UTC()
		var valPtr *string
		if value != "" {
			valPtr = &value
		}
		s := &models.SystemSetting{
			Key: key, Value: valPtr, ValueType: valueType, Version: 1,
			UpdatedAt: now, UpdatedBy: updatedBy,
		}
		m.SystemSettings[key] = s
		return s, nil
	})
}

// UpdateSystemSetting updates a system setting in the mock.
func (m *MockDB) UpdateSystemSetting(_ context.Context, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.SystemSetting, error) {
	_ = reason
	if m.UpdateSystemSettingErr != nil {
		return nil, m.UpdateSystemSettingErr
	}
	return runWithLock(m, true, func() (*models.SystemSetting, error) {
		s, ok := m.SystemSettings[key]
		if !ok {
			return nil, database.ErrNotFound
		}
		if expectedVersion != nil && s.Version != *expectedVersion {
			return nil, database.ErrConflict
		}
		var valPtr *string
		if value != "" {
			valPtr = &value
		}
		s.Value = valPtr
		s.ValueType = valueType
		s.Version++
		s.UpdatedAt = time.Now().UTC()
		s.UpdatedBy = updatedBy
		return s, nil
	})
}

// DeleteSystemSetting deletes a system setting from the mock.
func (m *MockDB) DeleteSystemSetting(_ context.Context, key string, expectedVersion *int, reason *string) error {
	_ = reason
	if m.DeleteSystemSettingErr != nil {
		return m.DeleteSystemSettingErr
	}
	return runWithWLockErr(m, func() error {
		s, ok := m.SystemSettings[key]
		if !ok {
			return database.ErrNotFound
		}
		if expectedVersion != nil && s.Version != *expectedVersion {
			return database.ErrConflict
		}
		delete(m.SystemSettings, key)
		return nil
	})
}

// GetArtifactByTaskIDAndPath returns a matching task artifact or database.ErrNotFound.
func (m *MockDB) GetArtifactByTaskIDAndPath(_ context.Context, taskID uuid.UUID, path string) (*models.TaskArtifact, error) {
	if m.GetArtifactByTaskIDAndPathErr != nil {
		return nil, m.GetArtifactByTaskIDAndPathErr
	}
	return runWithLock(m, false, func() (*models.TaskArtifact, error) {
		for _, a := range m.TaskArtifacts {
			if a.TaskID == taskID && a.Path == path {
				return a, nil
			}
		}
		return nil, database.ErrNotFound
	})
}

// CreateTaskArtifact appends an artifact to the mock slice.
func (m *MockDB) CreateTaskArtifact(_ context.Context, taskID uuid.UUID, path, storageRef string, sizeBytes *int64) (*models.TaskArtifact, error) {
	return runWithLock(m, true, func() (*models.TaskArtifact, error) {
		now := time.Now().UTC()
		ent := &models.TaskArtifact{
			TaskArtifactBase: models.TaskArtifactBase{
				TaskID:     taskID,
				Path:       path,
				StorageRef: storageRef,
				SizeBytes:  sizeBytes,
			},
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.TaskArtifacts = append(m.TaskArtifacts, ent)
		return ent, nil
	})
}

// ListArtifactPathsByTaskID returns paths of artifacts for the task.
func (m *MockDB) ListArtifactPathsByTaskID(_ context.Context, taskID uuid.UUID) ([]string, error) {
	return runWithLock(m, false, func() ([]string, error) {
		var paths []string
		for _, a := range m.TaskArtifacts {
			if a.TaskID == taskID {
				paths = append(paths, a.Path)
			}
		}
		return paths, nil
	})
}

// findActiveThread scans m.ChatThreads for the most recent thread matching (userID, projectID)
// that was updated within cutoff. Returns nil if none found. Must be called with lock held.
func (m *MockDB) findActiveThread(userID uuid.UUID, projectID *uuid.UUID, cutoff time.Time) *models.ChatThread {
	var best *models.ChatThread
	for _, t := range m.ChatThreads {
		if !threadMatches(t, userID, projectID, cutoff) {
			continue
		}
		if best == nil || t.UpdatedAt.After(best.UpdatedAt) {
			best = t
		}
	}
	return best
}

func threadMatches(t *models.ChatThread, userID uuid.UUID, projectID *uuid.UUID, cutoff time.Time) bool {
	if t.UserID != userID || t.UpdatedAt.Before(cutoff) {
		return false
	}
	if (projectID == nil) != (t.ProjectID == nil) {
		return false
	}
	return projectID == nil || *projectID == *t.ProjectID
}

// GetOrCreateActiveChatThread returns the most recent active thread for (userID, projectID)
// updated within 2 hours, or creates a new one. Mirrors the real DB behaviour.
func (m *MockDB) GetOrCreateActiveChatThread(_ context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.ChatThread, error) {
	return runWithLock(m, true, func() (*models.ChatThread, error) {
		cutoff := time.Now().UTC().Add(-2 * time.Hour)
		if best := m.findActiveThread(userID, projectID, cutoff); best != nil {
			return best, nil
		}
		now := time.Now().UTC()
		thread := &models.ChatThread{
			ChatThreadBase: models.ChatThreadBase{
				UserID:    userID,
				ProjectID: projectID,
			},
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.ChatThreads[thread.ID] = thread
		m.ChatMessages[thread.ID] = nil
		return thread, nil
	})
}

// CreateChatThread unconditionally creates a new thread.
func (m *MockDB) CreateChatThread(_ context.Context, userID uuid.UUID, projectID *uuid.UUID, title *string) (*models.ChatThread, error) {
	return runWithLock(m, true, func() (*models.ChatThread, error) {
		now := time.Now().UTC()
		thread := &models.ChatThread{
			ChatThreadBase: models.ChatThreadBase{
				UserID:    userID,
				ProjectID: projectID,
				Title:     title,
			},
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.ChatThreads[thread.ID] = thread
		m.ChatMessages[thread.ID] = nil
		return thread, nil
	})
}

// AppendChatMessage appends a message to the thread.
func (m *MockDB) AppendChatMessage(_ context.Context, threadID uuid.UUID, role, content string, metadata *string) (*models.ChatMessage, error) {
	return runWithLock(m, true, func() (*models.ChatMessage, error) {
		msg := &models.ChatMessage{
			ChatMessageBase: models.ChatMessageBase{
				ThreadID: threadID,
				Role:     role,
				Content:  content,
				Metadata: metadata,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
		}
		m.ChatMessages[threadID] = append(m.ChatMessages[threadID], msg)
		if t, ok := m.ChatThreads[threadID]; ok {
			t.UpdatedAt = time.Now().UTC()
		}
		return msg, nil
	})
}

// ListChatMessages returns one page of messages for the thread, oldest-first, and the total count.
func (m *MockDB) ListChatMessages(_ context.Context, threadID uuid.UUID, limit, offset int) ([]*models.ChatMessage, int64, error) {
	type out struct {
		msgs  []*models.ChatMessage
		total int64
	}
	res, err := runWithLock(m, false, func() (out, error) {
		msgs := m.ChatMessages[threadID]
		total := int64(len(msgs))
		if limit <= 0 {
			limit = database.DefaultChatMessagePageLimit
		}
		if limit > database.MaxChatMessagePageLimit {
			limit = database.MaxChatMessagePageLimit
		}
		if offset < 0 {
			offset = 0
		}
		if offset >= len(msgs) {
			return out{msgs: nil, total: total}, nil
		}
		end := offset + limit
		if end > len(msgs) {
			end = len(msgs)
		}
		slice := msgs[offset:end]
		cp := make([]*models.ChatMessage, len(slice))
		copy(cp, slice)
		return out{msgs: cp, total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return res.msgs, res.total, nil
}

// CreateChatAuditLog writes a chat audit log entry (no-op storage for mock).
func (m *MockDB) CreateChatAuditLog(_ context.Context, _ *models.ChatAuditLog) error {
	return runWithWLockErr(m, func() error { return nil })
}

// GetThreadByResponseID finds a thread by response_id stored in assistant message metadata.
func (m *MockDB) GetThreadByResponseID(_ context.Context, responseID string, userID uuid.UUID) (*models.ChatThread, error) {
	return runWithLock(m, false, func() (*models.ChatThread, error) {
		if responseID == "" {
			return nil, database.ErrNotFound
		}
		threadID := m.findThreadIDByResponseID(responseID)
		if threadID == uuid.Nil {
			return nil, database.ErrNotFound
		}
		t, ok := m.ChatThreads[threadID]
		if !ok || t.UserID != userID {
			return nil, database.ErrNotFound
		}
		return t, nil
	})
}

func (m *MockDB) findThreadIDByResponseID(responseID string) uuid.UUID {
	for threadID, msgs := range m.ChatMessages {
		for _, msg := range msgs {
			if msg.Metadata != nil && strings.Contains(*msg.Metadata, responseID) {
				return threadID
			}
		}
	}
	return uuid.Nil
}

// ListChatThreads returns threads for the user, recent-first, optionally filtered by projectID.
func (m *MockDB) ListChatThreads(_ context.Context, userID uuid.UUID, projectID *uuid.UUID, limit, offset int) ([]*models.ChatThread, error) {
	return runWithLock(m, false, func() ([]*models.ChatThread, error) {
		list := m.filterThreadsByUserAndProject(userID, projectID)
		sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		if offset < 0 {
			offset = 0
		}
		if offset >= len(list) {
			return nil, nil
		}
		list = list[offset:]
		if len(list) > limit {
			list = list[:limit]
		}
		return list, nil
	})
}

func (m *MockDB) filterThreadsByUserAndProject(userID uuid.UUID, projectID *uuid.UUID) []*models.ChatThread {
	var list []*models.ChatThread
	for _, t := range m.ChatThreads {
		if t.UserID != userID {
			continue
		}
		if projectID != nil && (t.ProjectID == nil || *t.ProjectID != *projectID) {
			continue
		}
		list = append(list, t)
	}
	return list
}

// GetChatThreadByID returns the thread if it belongs to the user.
func (m *MockDB) GetChatThreadByID(_ context.Context, threadID, userID uuid.UUID) (*models.ChatThread, error) {
	return runWithLock(m, false, func() (*models.ChatThread, error) {
		t, ok := m.ChatThreads[threadID]
		if !ok || t.UserID != userID {
			return nil, database.ErrNotFound
		}
		return t, nil
	})
}

// UpdateChatThreadTitle updates the thread title.
func (m *MockDB) UpdateChatThreadTitle(_ context.Context, threadID, userID uuid.UUID, title string) error {
	return runWithWLockErr(m, func() error {
		t, ok := m.ChatThreads[threadID]
		if !ok || t.UserID != userID {
			return database.ErrNotFound
		}
		t.Title = &title
		t.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// CreateSkill stores a skill in the mock.
func (m *MockDB) CreateSkill(_ context.Context, name, content, scope string, ownerID *uuid.UUID, isSystem bool) (*models.Skill, error) {
	if m.CreateSkillErr != nil {
		return nil, m.CreateSkillErr
	}
	return runWithLock(m, true, func() (*models.Skill, error) {
		id := uuid.New()
		now := time.Now().UTC()
		s := &models.Skill{
			SkillBase: models.SkillBase{
				Name:     name,
				Content:  content,
				Scope:    scope,
				OwnerID:  ownerID,
				IsSystem: isSystem,
			},
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.Skills[id] = s
		return s, nil
	})
}

// GetSkillByID returns a skill by id from the mock.
func (m *MockDB) GetSkillByID(_ context.Context, id uuid.UUID) (*models.Skill, error) {
	return getByKeyLocked(m, m.Skills, id)
}

// ListSkillsForUser returns one page of skills visible to user (owner_id = userID or is_system).
//
//nolint:gocognit // mirrors DB filtering branches for mock parity.
func (m *MockDB) ListSkillsForUser(_ context.Context, userID uuid.UUID, scopeFilter, ownerFilter string, limit, offset int) ([]*models.Skill, int64, error) {
	if m.ListSkillsForUserErr != nil {
		return nil, 0, m.ListSkillsForUserErr
	}
	type out struct {
		skills []*models.Skill
		total  int64
	}
	res, err := runWithLock(m, false, func() (out, error) {
		var list []*models.Skill
		for _, s := range m.Skills {
			if mockSkillVisible(s, userID, scopeFilter, ownerFilter) {
				list = append(list, s)
			}
		}
		total := int64(len(list))
		if limit <= 0 {
			limit = database.DefaultSkillPageLimit
		}
		if limit > database.MaxSkillPageLimit {
			limit = database.MaxSkillPageLimit
		}
		if offset < 0 {
			offset = 0
		}
		if offset >= len(list) {
			return out{skills: nil, total: total}, nil
		}
		end := offset + limit
		if end > len(list) {
			end = len(list)
		}
		return out{skills: list[offset:end], total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return res.skills, res.total, nil
}

func mockSkillVisible(s *models.Skill, userID uuid.UUID, scopeFilter, ownerFilter string) bool {
	if scopeFilter != "" && s.Scope != scopeFilter {
		return false
	}
	if ownerFilter != "" && (s.OwnerID == nil || s.OwnerID.String() != ownerFilter) {
		return false
	}
	if s.IsSystem {
		return true
	}
	return s.OwnerID != nil && *s.OwnerID == userID
}

// UpdateSkill updates a skill in the mock.
func (m *MockDB) UpdateSkill(ctx context.Context, id uuid.UUID, name, content, scope *string) (*models.Skill, error) {
	if m.UpdateSkillErr != nil {
		return nil, m.UpdateSkillErr
	}
	return runWithLock(m, true, func() (*models.Skill, error) {
		s, ok := m.Skills[id]
		if !ok {
			return nil, database.ErrNotFound
		}
		if s.IsSystem {
			return nil, fmt.Errorf("cannot update system skill")
		}
		if name != nil {
			s.Name = *name
		}
		if content != nil {
			s.Content = *content
		}
		if scope != nil {
			s.Scope = *scope
		}
		s.UpdatedAt = time.Now().UTC()
		return s, nil
	})
}

// DeleteSkill removes a skill from the mock.
func (m *MockDB) DeleteSkill(_ context.Context, id uuid.UUID) error {
	if m.DeleteSkillErr != nil {
		return m.DeleteSkillErr
	}
	return runWithWLockErr(m, func() error {
		s, ok := m.Skills[id]
		if !ok {
			return database.ErrNotFound
		}
		if s.IsSystem {
			return fmt.Errorf("cannot delete system skill")
		}
		delete(m.Skills, id)
		return nil
	})
}

// EnsureDefaultSkill creates or updates the default skill in the mock.
func (m *MockDB) EnsureDefaultSkill(_ context.Context, content string) error {
	if m.EnsureDefaultSkillErr != nil {
		return m.EnsureDefaultSkillErr
	}
	return runWithWLockErr(m, func() error {
		id := database.DefaultSkillID
		if s, ok := m.Skills[id]; ok {
			s.Content = content
			s.UpdatedAt = time.Now().UTC()
			return nil
		}
		now := time.Now().UTC()
		m.Skills[id] = &models.Skill{
			SkillBase: models.SkillBase{
				Name:     "CyNodeAI interaction",
				Content:  content,
				Scope:    "global",
				IsSystem: true,
			},
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return nil
	})
}

func (m *MockDB) EvaluateWorkflowStartGate(_ context.Context, _ *models.Task, _ bool) (string, error) {
	if m.EvaluateWorkflowStartGateErr != nil {
		return "", m.EvaluateWorkflowStartGateErr
	}
	return m.EvaluateWorkflowStartGateDenyReason, nil
}

func (m *MockDB) AcquireTaskWorkflowLease(_ context.Context, taskID, leaseID uuid.UUID, holderID string, expiresAt time.Time) (*models.TaskWorkflowLease, error) {
	return runWithLock(m, true, func() (*models.TaskWorkflowLease, error) {
		now := time.Now().UTC()
		existing, ok := m.TaskWorkflowLeases[taskID]
		if ok && existing.ExpiresAt != nil && existing.ExpiresAt.Before(now) {
			existing.LeaseID = leaseID
			h := holderID
			existing.HolderID = &h
			existing.ExpiresAt = &expiresAt
			existing.UpdatedAt = now
			return existing, nil
		}
		if ok && existing.HolderID != nil && *existing.HolderID == holderID && existing.LeaseID == leaseID {
			return existing, nil
		}
		if ok {
			return nil, database.ErrLeaseHeld
		}
		row := &models.TaskWorkflowLease{
			TaskWorkflowLeaseBase: models.TaskWorkflowLeaseBase{
				TaskID:    taskID,
				LeaseID:   leaseID,
				HolderID:  &holderID,
				ExpiresAt: &expiresAt,
			},
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.TaskWorkflowLeases[taskID] = row
		return row, nil
	})
}

func (m *MockDB) ReleaseTaskWorkflowLease(_ context.Context, taskID, leaseID uuid.UUID) error {
	return runWithWLockErr(m, func() error {
		row, ok := m.TaskWorkflowLeases[taskID]
		if !ok || row.LeaseID != leaseID {
			return nil
		}
		row.HolderID = nil
		row.ExpiresAt = nil
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
}

func (m *MockDB) GetTaskWorkflowLease(_ context.Context, taskID uuid.UUID) (*models.TaskWorkflowLease, error) {
	return runWithLock(m, false, func() (*models.TaskWorkflowLease, error) {
		if m.GetTaskWorkflowLeaseErr != nil {
			return nil, m.GetTaskWorkflowLeaseErr
		}
		return getByKey(m.TaskWorkflowLeases, taskID)
	})
}

func (m *MockDB) GetWorkflowCheckpoint(_ context.Context, taskID uuid.UUID) (*models.WorkflowCheckpoint, error) {
	return getByKeyLocked(m, m.WorkflowCheckpoints, taskID)
}

func (m *MockDB) UpsertWorkflowCheckpoint(_ context.Context, cp *models.WorkflowCheckpoint) error {
	return runWithWLockErr(m, func() error {
		now := time.Now().UTC()
		cp.UpdatedAt = now
		if existing, ok := m.WorkflowCheckpoints[cp.TaskID]; ok {
			existing.State = cp.State
			existing.LastNodeID = cp.LastNodeID
			existing.UpdatedAt = now
			return nil
		}
		if cp.ID == uuid.Nil {
			cp.ID = uuid.New()
		}
		m.WorkflowCheckpoints[cp.TaskID] = cp
		return nil
	})
}

func (m *MockDB) ListAccessControlRulesForApiCall(_ context.Context, subjectType string, subjectID *uuid.UUID, action, resourceType string) ([]*models.AccessControlRule, error) {
	return runWithLock(m, false, func() ([]*models.AccessControlRule, error) {
		if m.AccessControlRules == nil {
			return nil, nil
		}
		out := make([]*models.AccessControlRule, 0, len(m.AccessControlRules))
		for _, r := range m.AccessControlRules {
			if r != nil && r.Action == action && r.ResourceType == resourceType {
				out = append(out, r)
			}
		}
		return out, nil
	})
}

func (m *MockDB) CreateAccessControlAuditLog(_ context.Context, _ *models.AccessControlAuditLog) error {
	return nil
}

func (m *MockDB) HasActiveApiCredentialForUserAndProvider(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return runWithLock(m, false, func() (bool, error) {
		return m.HasActiveApiCredential, nil
	})
}

func (m *MockDB) HasAnyActiveApiCredential(_ context.Context) (bool, error) {
	return runWithLock(m, false, func() (bool, error) {
		return m.HasAnyActiveApiCredentialResult, nil
	})
}

// AddUser adds a pre-created user to the mock database.
func (m *MockDB) AddUser(user *models.User) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Users[user.ID] = user
	m.UsersByHandle[user.Handle] = user
}

// AddPasswordCredential adds a pre-created credential to the mock database.
func (m *MockDB) AddPasswordCredential(cred *models.PasswordCredential) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PasswordCreds[cred.UserID] = cred
}

// AddTask adds a pre-created task to the mock database.
func (m *MockDB) AddTask(task *models.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Tasks[task.ID] = task
}

// AddJob adds a pre-created job to the mock database.
func (m *MockDB) AddJob(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Jobs[job.ID] = job
	m.JobsByTask[job.TaskID] = append(m.JobsByTask[job.TaskID], job)
}

// AddNode adds a pre-created node to the mock database.
func (m *MockDB) AddNode(node *models.Node) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Nodes[node.ID] = node
	m.NodesBySlug[node.NodeSlug] = node
}

// AddRefreshSession adds a pre-created session to the mock database.
func (m *MockDB) AddRefreshSession(session *models.RefreshSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RefreshSessions[session.ID] = session
	m.SessionsByHash[string(session.RefreshTokenHash)] = session
}
