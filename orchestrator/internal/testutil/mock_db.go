// Package testutil provides test utilities and mock implementations.
package testutil

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// Ensure MockDB implements database.Store and focused sub-interfaces.
var (
	_ database.Store                 = (*MockDB)(nil)
	_ database.UserStore             = (*MockDB)(nil)
	_ database.TaskStore             = (*MockDB)(nil)
	_ database.NodeStore             = (*MockDB)(nil)
	_ database.ChatStore             = (*MockDB)(nil)
	_ database.SessionBindingStore   = (*MockDB)(nil)
	_ database.PreferenceStore       = (*MockDB)(nil)
	_ database.SkillStore            = (*MockDB)(nil)
	_ database.WorkflowStore         = (*MockDB)(nil)
	_ database.SystemSettingsStore   = (*MockDB)(nil)
	_ database.Transactional         = (*MockDB)(nil)
	_ database.WorkflowHandlerDeps   = (*MockDB)(nil)
	_ database.OpenAIChatHandlerDeps = (*MockDB)(nil)
)

// MockDB provides a mock implementation for database operations.
type MockDB struct {
	mu sync.RWMutex

	Users                 map[uuid.UUID]*models.User
	UsersByHandle         map[string]*models.User
	PasswordCreds         map[uuid.UUID]*models.PasswordCredential
	RefreshSessions       map[uuid.UUID]*models.RefreshSession
	SessionsByHash        map[string]*models.RefreshSession
	Nodes                 map[uuid.UUID]*models.Node
	NodesBySlug           map[string]*models.Node
	Projects              map[uuid.UUID]*models.Project
	DefaultProjectsByUser map[uuid.UUID]*models.Project
	Tasks                 map[uuid.UUID]*models.Task
	Jobs                  map[uuid.UUID]*models.Job
	JobsByTask            map[uuid.UUID][]*models.Job
	CapabilityHistory     []*NodeCapabilitySnapshot
	AuditLogs             []*AuthAuditLog
	ChatThreads           map[uuid.UUID]*models.ChatThread
	ChatMessages          map[uuid.UUID][]*models.ChatMessage
	PreferenceEntries     []*models.PreferenceEntry
	SystemSettings        map[string]*models.SystemSetting
	TaskArtifacts         []*models.TaskArtifact
	Skills                map[uuid.UUID]*models.Skill
	TaskWorkflowLeases    map[uuid.UUID]*models.TaskWorkflowLease
	WorkflowCheckpoints   map[uuid.UUID]*models.WorkflowCheckpoint
	SessionBindingsByKey  map[string]*models.SessionBinding

	// Access control and API egress (for handler tests).
	AccessControlRules              []*models.AccessControlRule
	HasActiveApiCredential          bool
	HasAnyActiveApiCredentialResult bool // for control-plane inference-path readiness (external key)

	// Error injection
	ForceError error

	// GetPreferenceErr, when set, makes GetPreference return this error (for testing handler internal-error path).
	GetPreferenceErr error
	// ListPreferencesErr, when set, makes ListPreferences return this error.
	ListPreferencesErr error
	// GetEffectivePreferencesForTaskErr, when set, makes GetEffectivePreferencesForTask return this error.
	GetEffectivePreferencesForTaskErr error
	// CreatePreferenceErr, UpdatePreferenceErr, DeletePreferenceErr for testing MCP handler internal-error paths.
	CreatePreferenceErr error
	UpdatePreferenceErr error
	DeletePreferenceErr error
	GetTaskByIDErr      error
	// GetTaskWorkflowLeaseErr, when set, makes GetTaskWorkflowLease return this error (workflow start WithTx path).
	GetTaskWorkflowLeaseErr error
	UpdateTaskStatusErr     error
	// GetOrCreateDefaultProjectForUserErr, when set, makes GetOrCreateDefaultProjectForUser return this error.
	GetOrCreateDefaultProjectForUserErr error
	GetJobByIDErr                       error
	GetJobsByTaskIDErr                  error
	// UpdateJobStatusErr, when set, makes UpdateJobStatus return this error.
	UpdateJobStatusErr            error
	GetArtifactByTaskIDAndPathErr error
	// GetUserByIDErr, when set, makes GetUserByID return this error.
	GetUserByIDErr error
	// EnsureDefaultSkillErr, when set, makes EnsureDefaultSkill return this error (user-gateway run() path).
	EnsureDefaultSkillErr error
	// UpdateSkillErr, when set, makes UpdateSkill return this error (for handler tests).
	UpdateSkillErr error
	// DeleteSkillErr, when set, makes DeleteSkill return this error (for handler tests).
	DeleteSkillErr error
	// CreateSkillErr, when set, makes CreateSkill return this error (for MCP handler tests).
	CreateSkillErr error
	// ListSkillsForUserErr, when set, makes ListSkillsForUser return this error.
	ListSkillsForUserErr error
	// UpdateSystemSettingErr, when set, makes UpdateSystemSetting return this error.
	UpdateSystemSettingErr error
	// DeleteSystemSettingErr, when set, makes DeleteSystemSetting return this error.
	DeleteSystemSettingErr error
	// ListTasksByUserErr, when set, makes ListTasksByUser return this error.
	ListTasksByUserErr error
	// ListAuthorizedProjectsForUserErr, when set, makes ListAuthorizedProjectsForUser return this error.
	ListAuthorizedProjectsForUserErr error
	// EvaluateWorkflowStartGateDenyReason, when set, makes EvaluateWorkflowStartGate return (this, nil).
	EvaluateWorkflowStartGateDenyReason string
	// EvaluateWorkflowStartGateErr, when set, makes EvaluateWorkflowStartGate return ("", this).
	EvaluateWorkflowStartGateErr error
}

// NodeCapabilitySnapshot represents a stored capability snapshot.
type NodeCapabilitySnapshot struct {
	NodeID         uuid.UUID
	CapabilityJSON string
	CreatedAt      time.Time
}

// AuthAuditLog represents an auth audit log entry.
type AuthAuditLog struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	EventType string
	Success   bool
	IPAddress *string
	UserAgent *string
	Details   *string
	CreatedAt time.Time
}

// NewMockDB creates a new mock database.
func NewMockDB() *MockDB {
	return &MockDB{
		Users:                 make(map[uuid.UUID]*models.User),
		UsersByHandle:         make(map[string]*models.User),
		PasswordCreds:         make(map[uuid.UUID]*models.PasswordCredential),
		RefreshSessions:       make(map[uuid.UUID]*models.RefreshSession),
		SessionsByHash:        make(map[string]*models.RefreshSession),
		Nodes:                 make(map[uuid.UUID]*models.Node),
		NodesBySlug:           make(map[string]*models.Node),
		Projects:              make(map[uuid.UUID]*models.Project),
		DefaultProjectsByUser: make(map[uuid.UUID]*models.Project),
		Tasks:                 make(map[uuid.UUID]*models.Task),
		Jobs:                  make(map[uuid.UUID]*models.Job),
		JobsByTask:            make(map[uuid.UUID][]*models.Job),
		ChatThreads:           make(map[uuid.UUID]*models.ChatThread),
		ChatMessages:          make(map[uuid.UUID][]*models.ChatMessage),
		SystemSettings:        make(map[string]*models.SystemSetting),
		Skills:                make(map[uuid.UUID]*models.Skill),
		TaskWorkflowLeases:    make(map[uuid.UUID]*models.TaskWorkflowLease),
		WorkflowCheckpoints:   make(map[uuid.UUID]*models.WorkflowCheckpoint),
		SessionBindingsByKey:  make(map[string]*models.SessionBinding),
	}
}

// runWithLock runs fn with the lock held (write if write true, else read); returns ForceError if set.
func runWithLock[T any](m *MockDB, write bool, fn func() (T, error)) (T, error) {
	if write {
		m.mu.Lock()
		defer m.mu.Unlock()
	} else {
		m.mu.RLock()
		defer m.mu.RUnlock()
	}
	if m.ForceError != nil {
		var zero T
		return zero, m.ForceError
	}
	return fn()
}

// getByKey returns the value for key in m, or ErrNotFound if missing.
func getByKey[K comparable, V any](m map[K]V, key K) (V, error) {
	v, ok := m[key]
	if !ok {
		var zero V
		return zero, database.ErrNotFound
	}
	return v, nil
}

// getByKeyLocked runs getByKey inside a read lock; used by Get* methods to avoid duplication.
func getByKeyLocked[K comparable, V any](m *MockDB, mp map[K]V, key K) (V, error) {
	return runWithLock(m, false, func() (V, error) { return getByKey(mp, key) })
}

// runWithWLockErr runs fn with the write lock held; returns ForceError if set.
func runWithWLockErr(m *MockDB, fn func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	return fn()
}

func isTerminalTaskStatus(status string) bool {
	switch status {
	case models.TaskStatusCompleted, models.TaskStatusFailed,
		models.TaskStatusCanceled, models.TaskStatusSuperseded:
		return true
	default:
		return false
	}
}

func (m *MockDB) setStatusAndUpdatedAt(id uuid.UUID, status string, forTask bool) error {
	return runWithWLockErr(m, func() error {
		now := time.Now().UTC()
		if forTask {
			if t, ok := m.Tasks[id]; ok {
				// Mirror DB guard: do not overwrite a terminal status.
				if isTerminalTaskStatus(t.Status) {
					return nil
				}
				t.Status = status
				t.UpdatedAt = now
			}
		} else {
			if j, ok := m.Jobs[id]; ok {
				j.Status = status
				j.UpdatedAt = now
			}
		}
		return nil
	})
}

// CreateUser creates a new user.
func (m *MockDB) CreateUser(_ context.Context, handle string, email *string) (*models.User, error) {
	return runWithLock(m, true, func() (*models.User, error) {
		user := &models.User{
			UserBase: models.UserBase{
				Handle:   handle,
				Email:    email,
				IsActive: true,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.Users[user.ID] = user
		m.UsersByHandle[handle] = user
		return user, nil
	})
}

// GetUserByHandle retrieves a user by handle.
func (m *MockDB) GetUserByHandle(_ context.Context, handle string) (*models.User, error) {
	return getByKeyLocked(m, m.UsersByHandle, handle)
}

// GetUserByID retrieves a user by ID.
func (m *MockDB) GetUserByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if m.GetUserByIDErr != nil {
		return nil, m.GetUserByIDErr
	}
	return getByKeyLocked(m, m.Users, id)
}

// CreatePasswordCredential creates a password credential.
func (m *MockDB) CreatePasswordCredential(_ context.Context, userID uuid.UUID, passwordHash []byte, hashAlg string) (*models.PasswordCredential, error) {
	return runWithLock(m, true, func() (*models.PasswordCredential, error) {
		cred := &models.PasswordCredential{
			PasswordCredentialBase: models.PasswordCredentialBase{
				UserID:       userID,
				PasswordHash: passwordHash,
				HashAlg:      hashAlg,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.PasswordCreds[userID] = cred
		return cred, nil
	})
}

// GetPasswordCredentialByUserID retrieves password credential by user ID.
func (m *MockDB) GetPasswordCredentialByUserID(_ context.Context, userID uuid.UUID) (*models.PasswordCredential, error) {
	return getByKeyLocked(m, m.PasswordCreds, userID)
}

// CreateRefreshSession creates a refresh session.
func (m *MockDB) CreateRefreshSession(_ context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*models.RefreshSession, error) {
	return runWithLock(m, true, func() (*models.RefreshSession, error) {
		session := &models.RefreshSession{
			RefreshSessionBase: models.RefreshSessionBase{
				UserID:           userID,
				RefreshTokenHash: tokenHash,
				IsActive:         true,
				ExpiresAt:        expiresAt,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.RefreshSessions[session.ID] = session
		m.SessionsByHash[string(tokenHash)] = session
		return session, nil
	})
}

// GetActiveRefreshSession retrieves an active session by token hash.
func (m *MockDB) GetActiveRefreshSession(_ context.Context, tokenHash []byte) (*models.RefreshSession, error) {
	return runWithLock(m, false, func() (*models.RefreshSession, error) {
		session, ok := m.SessionsByHash[string(tokenHash)]
		if !ok || !session.IsActive || session.ExpiresAt.Before(time.Now()) {
			return nil, database.ErrNotFound
		}
		return session, nil
	})
}

// GetRefreshSessionByID returns a refresh session by id (any status).
func (m *MockDB) GetRefreshSessionByID(_ context.Context, sessionID uuid.UUID) (*models.RefreshSession, error) {
	return runWithLock(m, false, func() (*models.RefreshSession, error) {
		return getByKey(m.RefreshSessions, sessionID)
	})
}

// invalidateSessionsWhere marks sessions matching pred as inactive. Caller must hold m.mu.
func (m *MockDB) invalidateSessionsWhere(pred func(*models.RefreshSession) bool) {
	now := time.Now().UTC()
	for _, session := range m.RefreshSessions {
		if pred(session) {
			session.IsActive = false
			session.UpdatedAt = now
		}
	}
}

// invalidateSessionsWithPred invalidates sessions matching pred.
func (m *MockDB) invalidateSessionsWithPred(pred func(*models.RefreshSession) bool) error {
	return runWithWLockErr(m, func() error {
		m.invalidateSessionsWhere(pred)
		return nil
	})
}

// InvalidateRefreshSession invalidates a refresh session.
func (m *MockDB) InvalidateRefreshSession(_ context.Context, sessionID uuid.UUID) error {
	return m.invalidateSessionsWithPred(func(s *models.RefreshSession) bool { return s.ID == sessionID })
}

// InvalidateAllUserSessions invalidates all sessions for a user.
func (m *MockDB) InvalidateAllUserSessions(_ context.Context, userID uuid.UUID) error {
	return m.invalidateSessionsWithPred(func(s *models.RefreshSession) bool { return s.UserID == userID })
}

// InvalidateAllRefreshSessions invalidates every refresh session.
func (m *MockDB) InvalidateAllRefreshSessions(_ context.Context) error {
	return m.invalidateSessionsWithPred(func(s *models.RefreshSession) bool { return s.IsActive })
}

// CreateAuthAuditLog creates an auth audit log entry (subjectHandle, reason per Store).
func (m *MockDB) CreateAuthAuditLog(_ context.Context, userID *uuid.UUID, eventType string, success bool, ipAddr, userAgent, subjectHandle, reason *string) error {
	return runWithWLockErr(m, func() error {
		entry := &AuthAuditLog{
			ID:        uuid.New(),
			UserID:    userID,
			EventType: eventType,
			Success:   success,
			IPAddress: ipAddr,
			UserAgent: userAgent,
			Details:   reason,
			CreatedAt: time.Now().UTC(),
		}
		m.AuditLogs = append(m.AuditLogs, entry)
		return nil
	})
}

// GetNodeByID retrieves a node by ID.
func (m *MockDB) GetNodeByID(_ context.Context, id uuid.UUID) (*models.Node, error) {
	return getByKeyLocked(m, m.Nodes, id)
}

// ListActiveNodes lists all active nodes.
func (m *MockDB) ListActiveNodes(_ context.Context) ([]*models.Node, error) {
	return runWithLock(m, false, func() ([]*models.Node, error) {
		var out []*models.Node
		for _, n := range m.Nodes {
			if n.Status == models.NodeStatusActive {
				out = append(out, n)
			}
		}
		return out, nil
	})
}

// ListNodes lists registered nodes ordered by node_slug with limit/offset pagination.
func (m *MockDB) ListNodes(_ context.Context, limit, offset int) ([]*models.Node, error) {
	return runWithLock(m, false, func() ([]*models.Node, error) {
		if limit <= 0 {
			limit = 50
		}
		if offset < 0 {
			offset = 0
		}
		var all []*models.Node
		for _, n := range m.Nodes {
			all = append(all, n)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].NodeSlug < all[j].NodeSlug })
		if offset >= len(all) {
			return []*models.Node{}, nil
		}
		end := offset + limit
		if end > len(all) {
			end = len(all)
		}
		return all[offset:end], nil
	})
}

// ApplyDispatchableWorkerFields sets config ack and worker API fields so ListDispatchableNodes includes the node.
// Pass empty targetURL or bearerToken for defaults used in handler tests.
func ApplyDispatchableWorkerFields(nb *models.NodeBase, targetURL, bearerToken string) {
	ack := "applied"
	url := targetURL
	if url == "" {
		url = "http://worker.test:8080"
	}
	tok := bearerToken
	if tok == "" {
		tok = "bearer-default"
	}
	nb.Status = models.NodeStatusActive
	nb.ConfigAckStatus = &ack
	nb.WorkerAPITargetURL = &url
	nb.WorkerAPIBearerToken = &tok
}

func isDispatchableNode(n *models.Node) bool {
	if n.Status != models.NodeStatusActive {
		return false
	}
	if n.ConfigAckStatus == nil || *n.ConfigAckStatus != "applied" {
		return false
	}
	if n.WorkerAPITargetURL == nil || *n.WorkerAPITargetURL == "" {
		return false
	}
	if n.WorkerAPIBearerToken == nil || *n.WorkerAPIBearerToken == "" {
		return false
	}
	return true
}

// ListDispatchableNodes lists active nodes with config ack applied and Worker API URL and token set.
func (m *MockDB) ListDispatchableNodes(_ context.Context) ([]*models.Node, error) {
	return runWithLock(m, false, func() ([]*models.Node, error) {
		var out []*models.Node
		for _, n := range m.Nodes {
			if isDispatchableNode(n) {
				out = append(out, n)
			}
		}
		return out, nil
	})
}

// CreateTask creates a new task. When taskName is set, mock sets Summary to it for response tests.
func (m *MockDB) CreateTask(_ context.Context, createdBy *uuid.UUID, prompt string, taskName *string, projectID ...*uuid.UUID) (*models.Task, error) {
	var effectiveProjectID *uuid.UUID
	if len(projectID) > 0 {
		effectiveProjectID = projectID[0]
	}
	return runWithLock(m, true, func() (*models.Task, error) {
		var summary string
		if taskName != nil {
			summary = *taskName
		} else {
			summary = fmt.Sprintf("task_name_%03d", len(m.Tasks)+1)
		}
		task := &models.Task{
			TaskBase: models.TaskBase{
				CreatedBy:     createdBy,
				ProjectID:     effectiveProjectID,
				Status:        models.TaskStatusPending,
				Prompt:        &prompt,
				Summary:       &summary,
				PlanningState: models.PlanningStateDraft,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.Tasks[task.ID] = task
		return task, nil
	})
}

// GetOrCreateDefaultProjectForUser returns a deterministic per-user default project from the mock.
func (m *MockDB) GetOrCreateDefaultProjectForUser(_ context.Context, userID uuid.UUID) (*models.Project, error) {
	if m.GetOrCreateDefaultProjectForUserErr != nil {
		return nil, m.GetOrCreateDefaultProjectForUserErr
	}
	return runWithLock(m, true, func() (*models.Project, error) {
		if p, ok := m.DefaultProjectsByUser[userID]; ok {
			return p, nil
		}
		now := time.Now().UTC()
		p := &models.Project{
			ProjectBase: models.ProjectBase{
				Slug:        "default-" + userID.String(),
				DisplayName: "Default Project",
				IsActive:    true,
			},
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.Projects[p.ID] = p
		m.DefaultProjectsByUser[userID] = p
		return p, nil
	})
}

// GetProjectByID returns a project by id.
func (m *MockDB) GetProjectByID(_ context.Context, id uuid.UUID) (*models.Project, error) {
	return runWithLock(m, false, func() (*models.Project, error) {
		p, ok := m.Projects[id]
		if !ok {
			return nil, database.ErrNotFound
		}
		cp := *p
		return &cp, nil
	})
}

// GetProjectBySlug returns a project by slug.
func (m *MockDB) GetProjectBySlug(_ context.Context, slug string) (*models.Project, error) {
	return runWithLock(m, false, func() (*models.Project, error) {
		for _, p := range m.Projects {
			if p.Slug == slug {
				cp := *p
				return &cp, nil
			}
		}
		return nil, database.ErrNotFound
	})
}

// ListAuthorizedProjectsForUser returns the default project for the user (MVP), optionally filtered by q.
func (m *MockDB) ListAuthorizedProjectsForUser(ctx context.Context, userID uuid.UUID, q string, limit, offset int) ([]*models.Project, error) {
	if m.ListAuthorizedProjectsForUserErr != nil {
		return nil, m.ListAuthorizedProjectsForUserErr
	}
	p, err := m.GetOrCreateDefaultProjectForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		return []*models.Project{}, nil
	}
	q = strings.TrimSpace(strings.ToLower(q))
	if q != "" {
		sl := strings.ToLower(p.Slug)
		dn := strings.ToLower(p.DisplayName)
		desc := ""
		if p.Description != nil {
			desc = strings.ToLower(*p.Description)
		}
		if !strings.Contains(sl, q) && !strings.Contains(dn, q) && !strings.Contains(desc, q) {
			return []*models.Project{}, nil
		}
	}
	return []*models.Project{p}, nil
}

// GetTaskByID retrieves a task by ID.
func (m *MockDB) GetTaskByID(_ context.Context, id uuid.UUID) (*models.Task, error) {
	if m.GetTaskByIDErr != nil {
		return nil, m.GetTaskByIDErr
	}
	return getByKeyLocked(m, m.Tasks, id)
}

// GetTaskBySummary returns the most recently created task with the given summary for the user.
func (m *MockDB) GetTaskBySummary(_ context.Context, userID uuid.UUID, summary string) (*models.Task, error) {
	return runWithLock(m, false, func() (*models.Task, error) {
		return getTaskBySummaryLocked(m.Tasks, userID, summary)
	})
}

func getTaskBySummaryLocked(tasks map[uuid.UUID]*models.Task, userID uuid.UUID, summary string) (*models.Task, error) {
	var found *models.Task
	for _, t := range tasks {
		if !taskMatchesUserAndSummary(t, userID, summary) {
			continue
		}
		if found == nil || t.CreatedAt.After(found.CreatedAt) {
			cp := *t
			found = &cp
		}
	}
	if found == nil {
		return nil, database.ErrNotFound
	}
	return found, nil
}

func taskMatchesUserAndSummary(t *models.Task, userID uuid.UUID, summary string) bool {
	return t.CreatedBy != nil && *t.CreatedBy == userID &&
		t.Summary != nil && *t.Summary == summary
}

// UpdateTaskStatus updates a task's status.
func (m *MockDB) UpdateTaskStatus(_ context.Context, taskID uuid.UUID, status string) error {
	if m.UpdateTaskStatusErr != nil {
		return m.UpdateTaskStatusErr
	}
	return m.setStatusAndUpdatedAt(taskID, status, true)
}

// UpdateTaskSummary updates a task's summary.
func (m *MockDB) UpdateTaskSummary(_ context.Context, taskID uuid.UUID, summary string) error {
	return runWithWLockErr(m, func() error {
		if task, ok := m.Tasks[taskID]; ok {
			task.Summary = &summary
			task.UpdatedAt = time.Now().UTC()
		}
		return nil
	})
}

// UpdateTaskMetadata updates task metadata.
func (m *MockDB) UpdateTaskMetadata(_ context.Context, taskID uuid.UUID, metadata *string) error {
	return runWithWLockErr(m, func() error {
		if task, ok := m.Tasks[taskID]; ok {
			task.Metadata = metadata
			task.UpdatedAt = time.Now().UTC()
		}
		return nil
	})
}

// UpdateTaskPlanningState sets planning_state.
func (m *MockDB) UpdateTaskPlanningState(_ context.Context, taskID uuid.UUID, planningState string) error {
	return runWithWLockErr(m, func() error {
		if task, ok := m.Tasks[taskID]; ok {
			task.PlanningState = planningState
			task.UpdatedAt = time.Now().UTC()
		}
		return nil
	})
}

// ListTasksByUser lists tasks created by a user.
func (m *MockDB) ListTasksByUser(_ context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	if m.ListTasksByUserErr != nil {
		return nil, m.ListTasksByUserErr
	}
	return runWithLock(m, false, func() ([]*models.Task, error) {
		var out []*models.Task
		for _, t := range m.Tasks {
			if t.CreatedBy != nil && *t.CreatedBy == userID {
				out = append(out, t)
			}
		}
		// simple desc by created_at and limit/offset (no sort for mock)
		if offset > len(out) {
			return nil, nil
		}
		out = out[offset:]
		if limit < len(out) {
			out = out[:limit]
		}
		return out, nil
	})
}

// GetJobsByTaskID retrieves jobs by task ID.
func (m *MockDB) GetJobsByTaskID(_ context.Context, taskID uuid.UUID) ([]*models.Job, error) {
	if m.GetJobsByTaskIDErr != nil {
		return nil, m.GetJobsByTaskIDErr
	}
	return runWithLock(m, false, func() ([]*models.Job, error) {
		jobs := m.JobsByTask[taskID]
		if jobs == nil {
			return []*models.Job{}, nil
		}
		out := append([]*models.Job(nil), jobs...)
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
		return out, nil
	})
}

// ListJobsForTask returns one page of jobs for a task (created_at ascending) and the total count.
func (m *MockDB) ListJobsForTask(_ context.Context, taskID uuid.UUID, limit, offset int) ([]*models.Job, int64, error) {
	if m.GetJobsByTaskIDErr != nil {
		return nil, 0, m.GetJobsByTaskIDErr
	}
	type out struct {
		jobs  []*models.Job
		total int64
	}
	res, err := runWithLock(m, false, func() (out, error) {
		jobs := m.JobsByTask[taskID]
		if jobs == nil {
			return out{jobs: []*models.Job{}, total: 0}, nil
		}
		sorted := append([]*models.Job(nil), jobs...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })
		total := int64(len(sorted))
		if limit <= 0 {
			limit = database.DefaultJobPageLimit
		}
		if limit > database.MaxJobPageLimit {
			limit = database.MaxJobPageLimit
		}
		if offset < 0 {
			offset = 0
		}
		if offset >= len(sorted) {
			return out{jobs: []*models.Job{}, total: total}, nil
		}
		end := offset + limit
		if end > len(sorted) {
			end = len(sorted)
		}
		return out{jobs: sorted[offset:end], total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return res.jobs, res.total, nil
}

// CreateJob creates a new job for a task.
func (m *MockDB) CreateJob(_ context.Context, taskID uuid.UUID, payload string) (*models.Job, error) {
	return runWithLock(m, true, func() (*models.Job, error) {
		job := &models.Job{
			JobBase: models.JobBase{
				TaskID:  taskID,
				Status:  models.JobStatusQueued,
				Payload: models.NewJSONBString(&payload),
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.Jobs[job.ID] = job
		m.JobsByTask[taskID] = append(m.JobsByTask[taskID], job)
		return job, nil
	})
}

// CreateJobWithID creates a new job with the given ID (e.g. for SBA job spec).
func (m *MockDB) CreateJobWithID(_ context.Context, taskID, jobID uuid.UUID, payload string) (*models.Job, error) {
	return runWithLock(m, true, func() (*models.Job, error) {
		job := &models.Job{
			JobBase: models.JobBase{
				TaskID:  taskID,
				Status:  models.JobStatusQueued,
				Payload: models.NewJSONBString(&payload),
			},
			ID:        jobID,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.Jobs[job.ID] = job
		m.JobsByTask[taskID] = append(m.JobsByTask[taskID], job)
		return job, nil
	})
}

// CreateJobCompleted creates a job that is already completed (orchestrator-side inference).
func (m *MockDB) CreateJobCompleted(_ context.Context, taskID, jobID uuid.UUID, result string) (*models.Job, error) {
	return runWithLock(m, true, func() (*models.Job, error) {
		now := time.Now().UTC()
		emptyPayload := "{}"
		job := &models.Job{
			JobBase: models.JobBase{
				TaskID:  taskID,
				Status:  models.JobStatusCompleted,
				Payload: models.NewJSONBString(&emptyPayload),
				Result:  models.NewJSONBString(&result),
				EndedAt: &now,
			},
			ID:        jobID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.Jobs[job.ID] = job
		m.JobsByTask[taskID] = append(m.JobsByTask[taskID], job)
		return job, nil
	})
}

// GetJobByID retrieves a job by ID.
func (m *MockDB) GetJobByID(_ context.Context, id uuid.UUID) (*models.Job, error) {
	if m.GetJobByIDErr != nil {
		return nil, m.GetJobByIDErr
	}
	return getByKeyLocked(m, m.Jobs, id)
}

// UpdateJobStatus updates a job's status.
func (m *MockDB) UpdateJobStatus(_ context.Context, jobID uuid.UUID, status string) error {
	if m.UpdateJobStatusErr != nil {
		return m.UpdateJobStatusErr
	}
	return m.setStatusAndUpdatedAt(jobID, status, false)
}

// AssignJobToNode assigns a job to a node.
func (m *MockDB) AssignJobToNode(_ context.Context, jobID, nodeID uuid.UUID) error {
	return runWithWLockErr(m, func() error {
		if job, ok := m.Jobs[jobID]; ok {
			now := time.Now().UTC()
			job.NodeID = &nodeID
			job.Status = models.JobStatusRunning
			job.StartedAt = &now
			job.UpdatedAt = now
		}
		return nil
	})
}

// CompleteJob marks a job as completed with a result.
func (m *MockDB) CompleteJob(_ context.Context, jobID uuid.UUID, result, status string) error {
	return runWithWLockErr(m, func() error {
		if job, ok := m.Jobs[jobID]; ok {
			now := time.Now().UTC()
			job.Result = models.NewJSONBString(&result)
			job.Status = status
			job.EndedAt = &now
			job.UpdatedAt = now
		}
		return nil
	})
}

// GetNextQueuedJob retrieves the next queued job.
func (m *MockDB) GetNextQueuedJob(_ context.Context) (*models.Job, error) {
	return runWithLock(m, false, func() (*models.Job, error) {
		for _, job := range m.Jobs {
			if job.Status == models.JobStatusQueued {
				return job, nil
			}
		}
		return nil, database.ErrNotFound
	})
}

// CreateNode creates a new node.
func (m *MockDB) CreateNode(_ context.Context, nodeSlug string) (*models.Node, error) {
	return runWithLock(m, true, func() (*models.Node, error) {
		node := &models.Node{
			NodeBase: models.NodeBase{
				NodeSlug: nodeSlug,
				Status:   models.NodeStatusRegistered,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.Nodes[node.ID] = node
		m.NodesBySlug[nodeSlug] = node
		return node, nil
	})
}

// GetNodeBySlug retrieves a node by slug.
func (m *MockDB) GetNodeBySlug(_ context.Context, slug string) (*models.Node, error) {
	return getByKeyLocked(m, m.NodesBySlug, slug)
}

// UpdateNodeStatus updates node status.
func (m *MockDB) UpdateNodeStatus(_ context.Context, nodeID uuid.UUID, status string) error {
	return runWithWLockErr(m, func() error {
		node, ok := m.Nodes[nodeID]
		if ok {
			node.Status = status
			node.UpdatedAt = time.Now().UTC()
		}
		return nil
	})
}

// UpdateNodeLastSeen updates node last seen timestamp.
func (m *MockDB) UpdateNodeLastSeen(_ context.Context, nodeID uuid.UUID) error {
	return runWithWLockErr(m, func() error {
		now := time.Now().UTC()
		node, ok := m.Nodes[nodeID]
		if ok {
			node.LastSeenAt = &now
			node.UpdatedAt = now
		}
		return nil
	})
}

// SaveNodeCapabilitySnapshot saves a capability snapshot.
func (m *MockDB) SaveNodeCapabilitySnapshot(_ context.Context, nodeID uuid.UUID, capJSON string) error {
	return runWithWLockErr(m, func() error {
		m.CapabilityHistory = append(m.CapabilityHistory, &NodeCapabilitySnapshot{
			NodeID:         nodeID,
			CapabilityJSON: capJSON,
			CreatedAt:      time.Now().UTC(),
		})
		return nil
	})
}

// GetLatestNodeCapabilitySnapshot returns the most recent capability snapshot for the node.
func (m *MockDB) GetLatestNodeCapabilitySnapshot(_ context.Context, nodeID uuid.UUID) (string, error) {
	return runWithLock(m, false, func() (string, error) {
		var latest *NodeCapabilitySnapshot
		for _, cap := range m.CapabilityHistory {
			if cap.NodeID != nodeID {
				continue
			}
			if latest == nil || cap.CreatedAt.After(latest.CreatedAt) {
				latest = cap
			}
		}
		if latest == nil {
			return "", database.ErrNotFound
		}
		return latest.CapabilityJSON, nil
	})
}

// updateNodeWith runs fn on the node identified by nodeID and sets UpdatedAt. Caller must use runWithWLockErr.
func (m *MockDB) updateNodeWith(nodeID uuid.UUID, fn func(*models.Node)) {
	node, ok := m.Nodes[nodeID]
	if ok {
		fn(node)
		now := time.Now().UTC()
		node.UpdatedAt = now
	}
}

// updateNode runs fn on the node under write lock; used by node update methods.
func (m *MockDB) updateNode(_ context.Context, nodeID uuid.UUID, fn func(*models.Node)) error {
	return runWithWLockErr(m, func() error {
		m.updateNodeWith(nodeID, fn)
		return nil
	})
}

// UpdateNodeCapability updates node capability hash.
func (m *MockDB) UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capHash string) error {
	return m.updateNode(ctx, nodeID, func(n *models.Node) { n.CapabilityHash = &capHash })
}

// UpdateNodeConfigVersion sets the node's config_version.
func (m *MockDB) UpdateNodeConfigVersion(ctx context.Context, nodeID uuid.UUID, configVersion string) error {
	return m.updateNode(ctx, nodeID, func(n *models.Node) { n.ConfigVersion = &configVersion })
}

// UpdateNodeConfigAck records the node's configuration acknowledgement.
func (m *MockDB) UpdateNodeConfigAck(_ context.Context, nodeID uuid.UUID, configVersion, status string, ackAt time.Time, errMsg *string) error {
	return runWithWLockErr(m, func() error {
		m.updateNodeWith(nodeID, func(n *models.Node) {
			n.ConfigAckAt = &ackAt
			n.ConfigAckStatus = &status
			n.ConfigAckError = errMsg
		})
		return nil
	})
}

// UpdateNodeWorkerAPIConfig stores the Worker API target URL and bearer token for the node.
func (m *MockDB) UpdateNodeWorkerAPIConfig(_ context.Context, nodeID uuid.UUID, targetURL, bearerToken string) error {
	return runWithWLockErr(m, func() error {
		m.updateNodeWith(nodeID, func(n *models.Node) {
			n.WorkerAPITargetURL = &targetURL
			n.WorkerAPIBearerToken = &bearerToken
		})
		return nil
	})
}

// CreateMcpToolCallAuditLog is a no-op for the mock unless ForceError is set; satisfies database.Store.
func (m *MockDB) CreateMcpToolCallAuditLog(_ context.Context, _ *models.McpToolCallAuditLog) error {
	return runWithWLockErr(m, func() error { return m.ForceError })
}

// WithTx runs fn with this mock as the Store. There is no SQL transaction; handlers use it for API symmetry with database.DB.WithTx.
func (m *MockDB) WithTx(ctx context.Context, fn func(ctx context.Context, tx database.Store) error) error {
	return fn(ctx, m)
}
