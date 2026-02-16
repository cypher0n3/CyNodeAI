// Package testutil provides test utilities and mock implementations.
package testutil

import (
        "context"
        "sync"
        "time"

        "github.com/google/uuid"

        "github.com/cypher0n3/cynodeai/internal/database"
        "github.com/cypher0n3/cynodeai/internal/models"
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
        NodeID      uuid.UUID
        CapabilityJSON string
        CreatedAt   time.Time
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

// CreateUser creates a new user.
func (m *MockDB) CreateUser(_ context.Context, handle string, email *string) (*models.User, error) {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

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
}

// GetUserByHandle retrieves a user by handle.
func (m *MockDB) GetUserByHandle(_ context.Context, handle string) (*models.User, error) {
        m.mu.RLock()
        defer m.mu.RUnlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        user, ok := m.UsersByHandle[handle]
        if !ok {
                return nil, database.ErrNotFound
        }
        return user, nil
}

// GetUserByID retrieves a user by ID.
func (m *MockDB) GetUserByID(_ context.Context, id uuid.UUID) (*models.User, error) {
        m.mu.RLock()
        defer m.mu.RUnlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        user, ok := m.Users[id]
        if !ok {
                return nil, database.ErrNotFound
        }
        return user, nil
}

// CreatePasswordCredential creates a password credential.
func (m *MockDB) CreatePasswordCredential(_ context.Context, userID uuid.UUID, passwordHash []byte, hashAlg string) (*models.PasswordCredential, error) {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

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
}

// GetPasswordCredentialByUserID retrieves password credential by user ID.
func (m *MockDB) GetPasswordCredentialByUserID(_ context.Context, userID uuid.UUID) (*models.PasswordCredential, error) {
        m.mu.RLock()
        defer m.mu.RUnlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        cred, ok := m.PasswordCreds[userID]
        if !ok {
                return nil, database.ErrNotFound
        }
        return cred, nil
}

// CreateRefreshSession creates a refresh session.
func (m *MockDB) CreateRefreshSession(_ context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*models.RefreshSession, error) {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

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
}

// GetActiveRefreshSession retrieves an active session by token hash.
func (m *MockDB) GetActiveRefreshSession(_ context.Context, tokenHash []byte) (*models.RefreshSession, error) {
        m.mu.RLock()
        defer m.mu.RUnlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        session, ok := m.SessionsByHash[string(tokenHash)]
        if !ok || !session.IsActive || session.ExpiresAt.Before(time.Now()) {
                return nil, database.ErrNotFound
        }
        return session, nil
}

// InvalidateRefreshSession invalidates a refresh session.
func (m *MockDB) InvalidateRefreshSession(_ context.Context, sessionID uuid.UUID) error {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return m.ForceError
        }

        session, ok := m.RefreshSessions[sessionID]
        if ok {
                session.IsActive = false
                session.UpdatedAt = time.Now().UTC()
        }
        return nil
}

// InvalidateAllUserSessions invalidates all sessions for a user.
func (m *MockDB) InvalidateAllUserSessions(_ context.Context, userID uuid.UUID) error {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return m.ForceError
        }

        for _, session := range m.RefreshSessions {
                if session.UserID == userID {
                        session.IsActive = false
                        session.UpdatedAt = time.Now().UTC()
                }
        }
        return nil
}

// CreateAuthAuditLog creates an auth audit log entry.
func (m *MockDB) CreateAuthAuditLog(_ context.Context, userID *uuid.UUID, eventType string, success bool, ipAddr, userAgent, details *string) error {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return m.ForceError
        }

        entry := &AuthAuditLog{
                ID:        uuid.New(),
                UserID:    userID,
                EventType: eventType,
                Success:   success,
                IPAddress: ipAddr,
                UserAgent: userAgent,
                Details:   details,
                CreatedAt: time.Now().UTC(),
        }
        m.AuditLogs = append(m.AuditLogs, entry)
        return nil
}

// CreateTask creates a new task.
func (m *MockDB) CreateTask(_ context.Context, createdBy *uuid.UUID, prompt string) (*models.Task, error) {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

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
}

// GetTaskByID retrieves a task by ID.
func (m *MockDB) GetTaskByID(_ context.Context, id uuid.UUID) (*models.Task, error) {
        m.mu.RLock()
        defer m.mu.RUnlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        task, ok := m.Tasks[id]
        if !ok {
                return nil, database.ErrNotFound
        }
        return task, nil
}

// GetJobsByTaskID retrieves jobs by task ID.
func (m *MockDB) GetJobsByTaskID(_ context.Context, taskID uuid.UUID) ([]*models.Job, error) {
        m.mu.RLock()
        defer m.mu.RUnlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        jobs := m.JobsByTask[taskID]
        if jobs == nil {
                return []*models.Job{}, nil
        }
        return jobs, nil
}

// CreateNode creates a new node.
func (m *MockDB) CreateNode(_ context.Context, nodeSlug string) (*models.Node, error) {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        node := &models.Node{
                ID:        uuid.New(),
                NodeSlug:  nodeSlug,
                Status:    "pending",
                CreatedAt: time.Now().UTC(),
                UpdatedAt: time.Now().UTC(),
        }
        m.Nodes[node.ID] = node
        m.NodesBySlug[nodeSlug] = node
        return node, nil
}

// GetNodeBySlug retrieves a node by slug.
func (m *MockDB) GetNodeBySlug(_ context.Context, slug string) (*models.Node, error) {
        m.mu.RLock()
        defer m.mu.RUnlock()

        if m.ForceError != nil {
                return nil, m.ForceError
        }

        node, ok := m.NodesBySlug[slug]
        if !ok {
                return nil, database.ErrNotFound
        }
        return node, nil
}

// UpdateNodeStatus updates node status.
func (m *MockDB) UpdateNodeStatus(_ context.Context, nodeID uuid.UUID, status string) error {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return m.ForceError
        }

        node, ok := m.Nodes[nodeID]
        if ok {
                node.Status = status
                node.UpdatedAt = time.Now().UTC()
        }
        return nil
}

// UpdateNodeLastSeen updates node last seen timestamp.
func (m *MockDB) UpdateNodeLastSeen(_ context.Context, nodeID uuid.UUID) error {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return m.ForceError
        }

        now := time.Now().UTC()
        node, ok := m.Nodes[nodeID]
        if ok {
                node.LastSeenAt = &now
                node.UpdatedAt = now
        }
        return nil
}

// SaveNodeCapabilitySnapshot saves a capability snapshot.
func (m *MockDB) SaveNodeCapabilitySnapshot(_ context.Context, nodeID uuid.UUID, capJSON string) error {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return m.ForceError
        }

        m.CapabilityHistory = append(m.CapabilityHistory, &NodeCapabilitySnapshot{
                NodeID:         nodeID,
                CapabilityJSON: capJSON,
                CreatedAt:      time.Now().UTC(),
        })
        return nil
}

// UpdateNodeCapability updates node capability hash.
func (m *MockDB) UpdateNodeCapability(_ context.Context, nodeID uuid.UUID, capHash string) error {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.ForceError != nil {
                return m.ForceError
        }

        node, ok := m.Nodes[nodeID]
        if ok {
                node.CapabilityHash = &capHash
                node.UpdatedAt = time.Now().UTC()
        }
        return nil
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
