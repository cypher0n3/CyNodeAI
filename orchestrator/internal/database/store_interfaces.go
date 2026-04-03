package database

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// UserStore covers authentication identity, sessions, API egress policy checks, and MCP audit hooks tied to user-gateway auth.
type UserStore interface {
	CreateUser(ctx context.Context, handle string, email *string) (*models.User, error)
	GetUserByHandle(ctx context.Context, handle string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	CreatePasswordCredential(ctx context.Context, userID uuid.UUID, passwordHash []byte, hashAlg string) (*models.PasswordCredential, error)
	GetPasswordCredentialByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordCredential, error)

	CreateRefreshSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*models.RefreshSession, error)
	GetActiveRefreshSession(ctx context.Context, tokenHash []byte) (*models.RefreshSession, error)
	GetRefreshSessionByID(ctx context.Context, sessionID uuid.UUID) (*models.RefreshSession, error)
	InvalidateRefreshSession(ctx context.Context, sessionID uuid.UUID) error
	InvalidateAllUserSessions(ctx context.Context, userID uuid.UUID) error
	// InvalidateAllRefreshSessions marks every refresh session inactive (dev reset / emergency).
	InvalidateAllRefreshSessions(ctx context.Context) error

	CreateAuthAuditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddress, userAgent, subjectHandle, reason *string) error

	ListAccessControlRulesForApiCall(ctx context.Context, subjectType string, subjectID *uuid.UUID, action, resourceType string) ([]*models.AccessControlRule, error)
	CreateAccessControlAuditLog(ctx context.Context, rec *models.AccessControlAuditLog) error
	HasActiveApiCredentialForUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (bool, error)
	HasAnyActiveApiCredential(ctx context.Context) (bool, error)
}

// TaskStore covers tasks, jobs, dispatcher queue, MCP tool-call audit, artifacts, and projects.
type TaskStore interface {
	CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string, taskName *string, projectID ...*uuid.UUID) (*models.Task, error)
	GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	GetTaskBySummary(ctx context.Context, userID uuid.UUID, summary string) (*models.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error
	UpdateTaskSummary(ctx context.Context, taskID uuid.UUID, summary string) error
	UpdateTaskMetadata(ctx context.Context, taskID uuid.UUID, metadata *string) error
	UpdateTaskPlanningState(ctx context.Context, taskID uuid.UUID, planningState string) error
	ListTasksByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error)
	GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error)
	ListJobsForTask(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.Job, int64, error)
	CreateJob(ctx context.Context, taskID uuid.UUID, payload string) (*models.Job, error)
	CreateJobWithID(ctx context.Context, taskID, jobID uuid.UUID, payload string) (*models.Job, error)
	CreateJobCompleted(ctx context.Context, taskID, jobID uuid.UUID, result string) (*models.Job, error)
	GetJobByID(ctx context.Context, id uuid.UUID) (*models.Job, error)
	UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error
	AssignJobToNode(ctx context.Context, jobID, nodeID uuid.UUID) error
	CompleteJob(ctx context.Context, jobID uuid.UUID, result, status string) error
	GetNextQueuedJob(ctx context.Context) (*models.Job, error)

	CreateMcpToolCallAuditLog(ctx context.Context, rec *models.McpToolCallAuditLog) error

	GetArtifactByTaskIDAndPath(ctx context.Context, taskID uuid.UUID, path string) (*models.TaskArtifact, error)
	CreateTaskArtifact(ctx context.Context, taskID uuid.UUID, path, storageRef string, sizeBytes *int64) (*models.TaskArtifact, error)
	ListArtifactPathsByTaskID(ctx context.Context, taskID uuid.UUID) ([]string, error)
	GetOrCreateDefaultProjectForUser(ctx context.Context, userID uuid.UUID) (*models.Project, error)
	GetProjectByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
	GetProjectBySlug(ctx context.Context, slug string) (*models.Project, error)
	ListAuthorizedProjectsForUser(ctx context.Context, userID uuid.UUID, q string, limit, offset int) ([]*models.Project, error)
}

// NodeStore covers worker node registration, capability snapshots, and dispatch lists.
type NodeStore interface {
	CreateNode(ctx context.Context, nodeSlug string) (*models.Node, error)
	GetNodeBySlug(ctx context.Context, slug string) (*models.Node, error)
	GetNodeByID(ctx context.Context, id uuid.UUID) (*models.Node, error)
	UpdateNodeStatus(ctx context.Context, nodeID uuid.UUID, status string) error
	UpdateNodeLastSeen(ctx context.Context, nodeID uuid.UUID) error
	SaveNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID, capJSON string) error
	GetLatestNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID) (string, error)
	UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capHash string) error
	ListActiveNodes(ctx context.Context) ([]*models.Node, error)
	ListDispatchableNodes(ctx context.Context) ([]*models.Node, error)
	ListNodes(ctx context.Context, limit, offset int) ([]*models.Node, error)
	UpdateNodeConfigVersion(ctx context.Context, nodeID uuid.UUID, configVersion string) error
	UpdateNodeConfigAck(ctx context.Context, nodeID uuid.UUID, configVersion, status string, ackAt time.Time, errMsg *string) error
	UpdateNodeWorkerAPIConfig(ctx context.Context, nodeID uuid.UUID, targetURL, bearerToken string) error
}

// PreferenceStore covers user/project/task preference entries (MCP preference.* tools).
type PreferenceStore interface {
	GetPreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key string) (*models.PreferenceEntry, error)
	ListPreferences(ctx context.Context, scopeType string, scopeID *uuid.UUID, keyPrefix string, limit int, cursor string) ([]*models.PreferenceEntry, string, error)
	GetEffectivePreferencesForTask(ctx context.Context, taskID uuid.UUID) (map[string]interface{}, error)
	CreatePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, reason, updatedBy *string) (*models.PreferenceEntry, error)
	UpdatePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.PreferenceEntry, error)
	DeletePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key string, expectedVersion *int, reason *string) error
}

// SystemSettingsStore covers orchestrator system_setting.* keys.
type SystemSettingsStore interface {
	GetSystemSetting(ctx context.Context, key string) (*models.SystemSetting, error)
	ListSystemSettings(ctx context.Context, keyPrefix string, limit int, cursor string) ([]*models.SystemSetting, string, error)
	CreateSystemSetting(ctx context.Context, key, value, valueType string, reason, updatedBy *string) (*models.SystemSetting, error)
	UpdateSystemSetting(ctx context.Context, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.SystemSetting, error)
	DeleteSystemSetting(ctx context.Context, key string, expectedVersion *int, reason *string) error
}

// ChatStore covers OpenAI-compatible chat threads and messages.
type ChatStore interface {
	GetOrCreateActiveChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.ChatThread, error)
	CreateChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, title *string) (*models.ChatThread, error)
	GetThreadByResponseID(ctx context.Context, responseID string, userID uuid.UUID) (*models.ChatThread, error)
	ListChatThreads(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, limit, offset int) ([]*models.ChatThread, error)
	GetChatThreadByID(ctx context.Context, threadID, userID uuid.UUID) (*models.ChatThread, error)
	UpdateChatThreadTitle(ctx context.Context, threadID, userID uuid.UUID, title string) error
	AppendChatMessage(ctx context.Context, threadID uuid.UUID, role, content string, metadata *string) (*models.ChatMessage, error)
	ListChatMessages(ctx context.Context, threadID uuid.UUID, limit, offset int) ([]*models.ChatMessage, int64, error)
	CreateChatAuditLog(ctx context.Context, rec *models.ChatAuditLog) error
}

// SessionBindingStore covers per-session-binding PMA control-plane rows (REQ-ORCHES-0188).
type SessionBindingStore interface {
	UpsertSessionBinding(ctx context.Context, lineage models.SessionBindingLineage, serviceID, state string) (*models.SessionBinding, error)
	GetSessionBindingByKey(ctx context.Context, bindingKey string) (*models.SessionBinding, error)
	ListActiveBindingsForUser(ctx context.Context, userID uuid.UUID) ([]*models.SessionBinding, error)
	ListAllActiveSessionBindings(ctx context.Context) ([]*models.SessionBinding, error)
	TouchActiveSessionBindingsForUser(ctx context.Context, userID uuid.UUID, at time.Time) error
	TouchSessionBindingByKey(ctx context.Context, bindingKey string, at time.Time) error
}

// SkillStore covers user and system skills.
type SkillStore interface {
	CreateSkill(ctx context.Context, name, content, scope string, ownerID *uuid.UUID, isSystem bool) (*models.Skill, error)
	GetSkillByID(ctx context.Context, id uuid.UUID) (*models.Skill, error)
	ListSkillsForUser(ctx context.Context, userID uuid.UUID, scopeFilter, ownerFilter string, limit, offset int) ([]*models.Skill, int64, error)
	UpdateSkill(ctx context.Context, id uuid.UUID, name, content, scope *string) (*models.Skill, error)
	DeleteSkill(ctx context.Context, id uuid.UUID) error
	EnsureDefaultSkill(ctx context.Context, content string) error
}

// WorkflowStore covers workflow start gate, lease lifecycle, and checkpoints.
type WorkflowStore interface {
	EvaluateWorkflowStartGate(ctx context.Context, task *models.Task, requestedByPMA bool) (denyReason string, err error)
	AcquireTaskWorkflowLease(ctx context.Context, taskID uuid.UUID, leaseID uuid.UUID, holderID string, expiresAt time.Time) (*models.TaskWorkflowLease, error)
	ReleaseTaskWorkflowLease(ctx context.Context, taskID uuid.UUID, leaseID uuid.UUID) error
	GetTaskWorkflowLease(ctx context.Context, taskID uuid.UUID) (*models.TaskWorkflowLease, error)
	GetWorkflowCheckpoint(ctx context.Context, taskID uuid.UUID) (*models.WorkflowCheckpoint, error)
	UpsertWorkflowCheckpoint(ctx context.Context, cp *models.WorkflowCheckpoint) error
}

// Transactional runs work inside a single SQL transaction. The callback receives a full Store backed by the transaction.
type Transactional interface {
	WithTx(ctx context.Context, fn func(ctx context.Context, tx Store) error) error
}

// WorkflowHandlerDeps is the minimal store surface for workflow HTTP handlers.
type WorkflowHandlerDeps interface {
	TaskStore
	WorkflowStore
	Transactional
}

// OpenAIChatHandlerDeps is the minimal store surface for OpenAI-compatible chat handlers.
type OpenAIChatHandlerDeps interface {
	ChatStore
	NodeStore
	SessionBindingStore
}

// Compile-time checks: *DB implements each composed interface (review report: focused sub-stores).
var (
	_ UserStore             = (*DB)(nil)
	_ TaskStore             = (*DB)(nil)
	_ NodeStore             = (*DB)(nil)
	_ ChatStore             = (*DB)(nil)
	_ SessionBindingStore   = (*DB)(nil)
	_ PreferenceStore       = (*DB)(nil)
	_ SkillStore            = (*DB)(nil)
	_ WorkflowStore         = (*DB)(nil)
	_ SystemSettingsStore   = (*DB)(nil)
	_ Transactional         = (*DB)(nil)
	_ OpenAIChatHandlerDeps = (*DB)(nil)
	_ Store                 = (*DB)(nil)
)
