// Package testutil provides test utilities and mock implementations.
package testutil

import (
	"context"
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

	Users             map[uuid.UUID]*models.User
	UsersByHandle     map[string]*models.User
	PasswordCreds     map[uuid.UUID]*models.PasswordCredential
	RefreshSessions   map[uuid.UUID]*models.RefreshSession
	SessionsByHash    map[string]*models.RefreshSession
	Nodes             map[uuid.UUID]*models.Node
	NodesBySlug       map[string]*models.Node
	Tasks             map[uuid.UUID]*models.Task
	Jobs              map[uuid.UUID]*models.Job
	JobsByTask        map[uuid.UUID][]*models.Job
	CapabilityHistory []*NodeCapabilitySnapshot
	AuditLogs         []*AuthAuditLog

	// Error injection
	ForceError error
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
		Users:           make(map[uuid.UUID]*models.User),
		UsersByHandle:   make(map[string]*models.User),
		PasswordCreds:   make(map[uuid.UUID]*models.PasswordCredential),
		RefreshSessions: make(map[uuid.UUID]*models.RefreshSession),
		SessionsByHash:  make(map[string]*models.RefreshSession),
		Nodes:           make(map[uuid.UUID]*models.Node),
		NodesBySlug:     make(map[string]*models.Node),
		Tasks:           make(map[uuid.UUID]*models.Task),
		Jobs:            make(map[uuid.UUID]*models.Job),
		JobsByTask:      make(map[uuid.UUID][]*models.Job),
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

func (m *MockDB) setStatusAndUpdatedAt(id uuid.UUID, status string, forTask bool) error {
	return runWithWLockErr(m, func() error {
		now := time.Now().UTC()
		if forTask {
			if t, ok := m.Tasks[id]; ok {
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

// CreateTask creates a new task.
func (m *MockDB) CreateTask(_ context.Context, createdBy *uuid.UUID, prompt string) (*models.Task, error) {
	return runWithLock(m, true, func() (*models.Task, error) {
		task := &models.Task{
			ID:        uuid.New(),
			CreatedBy: createdBy,
			Status:    models.TaskStatusPending,
			Prompt:    &prompt,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		m.Tasks[task.ID] = task
		return task, nil
	})
}

// GetTaskByID retrieves a task by ID.
func (m *MockDB) GetTaskByID(_ context.Context, id uuid.UUID) (*models.Task, error) {
	return getByKeyLocked(m, m.Tasks, id)
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
