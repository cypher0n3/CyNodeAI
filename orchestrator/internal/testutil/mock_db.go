// Package testutil provides test utilities and mock implementations.
package testutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// Ensure MockDB implements database.Store interface
var _ database.Store = (*MockDB)(nil)

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
	TaskArtifacts         []*models.TaskArtifact
	Skills                map[uuid.UUID]*models.Skill
	TaskWorkflowLeases    map[uuid.UUID]*models.TaskWorkflowLease
	WorkflowCheckpoints   map[uuid.UUID]*models.WorkflowCheckpoint

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
	CreatePreferenceErr           error
	UpdatePreferenceErr           error
	DeletePreferenceErr           error
	GetTaskByIDErr                error
	GetJobByIDErr                 error
	GetArtifactByTaskIDAndPathErr error
	// UpdateSkillErr, when set, makes UpdateSkill return this error (for handler tests).
	UpdateSkillErr error
	// DeleteSkillErr, when set, makes DeleteSkill return this error (for handler tests).
	DeleteSkillErr error
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
		Skills:                make(map[uuid.UUID]*models.Skill),
		TaskWorkflowLeases:    make(map[uuid.UUID]*models.TaskWorkflowLease),
		WorkflowCheckpoints:   make(map[uuid.UUID]*models.WorkflowCheckpoint),
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
			ID:        uuid.New(),
			Handle:    handle,
			Email:     email,
			IsActive:  true,
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
	return getByKeyLocked(m, m.Users, id)
}

// CreatePasswordCredential creates a password credential.
func (m *MockDB) CreatePasswordCredential(_ context.Context, userID uuid.UUID, passwordHash []byte, hashAlg string) (*models.PasswordCredential, error) {
	return runWithLock(m, true, func() (*models.PasswordCredential, error) {
		cred := &models.PasswordCredential{
			ID:           uuid.New(),
			UserID:       userID,
			PasswordHash: passwordHash,
			HashAlg:      hashAlg,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
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
			ID:               uuid.New(),
			UserID:           userID,
			RefreshTokenHash: tokenHash,
			IsActive:         true,
			ExpiresAt:        expiresAt,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
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
			ID:        uuid.New(),
			CreatedBy: createdBy,
			ProjectID: effectiveProjectID,
			Status:    models.TaskStatusPending,
			Prompt:    &prompt,
			Summary:   &summary,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.Tasks[task.ID] = task
		return task, nil
	})
}

// GetOrCreateDefaultProjectForUser returns a deterministic per-user default project from the mock.
func (m *MockDB) GetOrCreateDefaultProjectForUser(_ context.Context, userID uuid.UUID) (*models.Project, error) {
	return runWithLock(m, true, func() (*models.Project, error) {
		if p, ok := m.DefaultProjectsByUser[userID]; ok {
			return p, nil
		}
		now := time.Now().UTC()
		p := &models.Project{
			ID:          uuid.New(),
			Slug:        "default-" + userID.String(),
			DisplayName: "Default Project",
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		m.Projects[p.ID] = p
		m.DefaultProjectsByUser[userID] = p
		return p, nil
	})
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

// ListTasksByUser lists tasks created by a user.
func (m *MockDB) ListTasksByUser(_ context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
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
	return runWithLock(m, false, func() ([]*models.Job, error) {
		jobs := m.JobsByTask[taskID]
		if jobs == nil {
			return []*models.Job{}, nil
		}
		return jobs, nil
	})
}

// CreateJob creates a new job for a task.
func (m *MockDB) CreateJob(_ context.Context, taskID uuid.UUID, payload string) (*models.Job, error) {
	return runWithLock(m, true, func() (*models.Job, error) {
		job := &models.Job{
			ID:        uuid.New(),
			TaskID:    taskID,
			Status:    models.JobStatusQueued,
			Payload:   models.NewJSONBString(&payload),
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
			ID:        jobID,
			TaskID:    taskID,
			Status:    models.JobStatusQueued,
			Payload:   models.NewJSONBString(&payload),
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
			ID:        jobID,
			TaskID:    taskID,
			Status:    models.JobStatusCompleted,
			Payload:   models.NewJSONBString(&emptyPayload),
			Result:    models.NewJSONBString(&result),
			EndedAt:   &now,
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
			ID:        uuid.New(),
			NodeSlug:  nodeSlug,
			Status:    models.NodeStatusRegistered,
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
			ID:        uuid.New(),
			ScopeType: scopeType,
			ScopeID:   scopeID,
			Key:       key,
			Value:     valPtr,
			ValueType: valueType,
			Version:   1,
			UpdatedAt: time.Now().UTC(),
			UpdatedBy: updatedBy,
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
			ID:         uuid.New(),
			TaskID:     taskID,
			Path:       path,
			StorageRef: storageRef,
			SizeBytes:  sizeBytes,
			CreatedAt:  now,
			UpdatedAt:  now,
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
			ID:        uuid.New(),
			UserID:    userID,
			ProjectID: projectID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.ChatThreads[thread.ID] = thread
		m.ChatMessages[thread.ID] = nil
		return thread, nil
	})
}

// CreateChatThread unconditionally creates a new thread.
func (m *MockDB) CreateChatThread(_ context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.ChatThread, error) {
	return runWithLock(m, true, func() (*models.ChatThread, error) {
		now := time.Now().UTC()
		thread := &models.ChatThread{
			ID:        uuid.New(),
			UserID:    userID,
			ProjectID: projectID,
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
			ID:        uuid.New(),
			ThreadID:  threadID,
			Role:      role,
			Content:   content,
			Metadata:  metadata,
			CreatedAt: time.Now().UTC(),
		}
		m.ChatMessages[threadID] = append(m.ChatMessages[threadID], msg)
		if t, ok := m.ChatThreads[threadID]; ok {
			t.UpdatedAt = time.Now().UTC()
		}
		return msg, nil
	})
}

// ListChatMessages returns stored messages for the thread, oldest-first, up to limit (0 = all).
func (m *MockDB) ListChatMessages(_ context.Context, threadID uuid.UUID, limit int) ([]*models.ChatMessage, error) {
	return runWithLock(m, false, func() ([]*models.ChatMessage, error) {
		msgs := m.ChatMessages[threadID]
		if limit > 0 && len(msgs) > limit {
			msgs = msgs[len(msgs)-limit:]
		}
		out := make([]*models.ChatMessage, len(msgs))
		copy(out, msgs)
		return out, nil
	})
}

// CreateChatAuditLog writes a chat audit log entry (no-op storage for mock).
func (m *MockDB) CreateChatAuditLog(_ context.Context, _ *models.ChatAuditLog) error {
	return runWithWLockErr(m, func() error { return nil })
}

// CreateSkill stores a skill in the mock.
func (m *MockDB) CreateSkill(_ context.Context, name, content, scope string, ownerID *uuid.UUID, isSystem bool) (*models.Skill, error) {
	return runWithLock(m, true, func() (*models.Skill, error) {
		id := uuid.New()
		now := time.Now().UTC()
		s := &models.Skill{ID: id, Name: name, Content: content, Scope: scope, OwnerID: ownerID, IsSystem: isSystem, CreatedAt: now, UpdatedAt: now}
		m.Skills[id] = s
		return s, nil
	})
}

// GetSkillByID returns a skill by id from the mock.
func (m *MockDB) GetSkillByID(_ context.Context, id uuid.UUID) (*models.Skill, error) {
	return getByKeyLocked(m, m.Skills, id)
}

// ListSkillsForUser returns skills visible to user (owner_id = userID or is_system).
func (m *MockDB) ListSkillsForUser(_ context.Context, userID uuid.UUID, scopeFilter, ownerFilter string) ([]*models.Skill, error) {
	return runWithLock(m, false, func() ([]*models.Skill, error) {
		var out []*models.Skill
		for _, s := range m.Skills {
			if mockSkillVisible(s, userID, scopeFilter) {
				out = append(out, s)
			}
		}
		return out, nil
	})
}

func mockSkillVisible(s *models.Skill, userID uuid.UUID, scopeFilter string) bool {
	if scopeFilter != "" && s.Scope != scopeFilter {
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
	return runWithWLockErr(m, func() error {
		id := database.DefaultSkillID
		if s, ok := m.Skills[id]; ok {
			s.Content = content
			s.UpdatedAt = time.Now().UTC()
			return nil
		}
		now := time.Now().UTC()
		m.Skills[id] = &models.Skill{ID: id, Name: "CyNodeAI interaction", Content: content, Scope: "global", IsSystem: true, CreatedAt: now, UpdatedAt: now}
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
			ID:        uuid.New(),
			TaskID:    taskID,
			LeaseID:   leaseID,
			HolderID:  &holderID,
			ExpiresAt: &expiresAt,
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
	return getByKeyLocked(m, m.TaskWorkflowLeases, taskID)
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
