// Package database provides PostgreSQL database operations via GORM.
// See docs/tech_specs/postgres_schema.md for schema details.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("not found")

// ErrExists is returned when a create would violate a unique constraint (e.g. preference key already exists).
var ErrExists = errors.New("already exists")

// ErrConflict is returned when an update or delete fails due to version mismatch (expected_version).
var ErrConflict = errors.New("version conflict")

// ErrLeaseHeld is returned when a task workflow lease is held by another holder (or by same holder with different lease_id).
var ErrLeaseHeld = errors.New("lease held")

func wrapErr(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

// Store defines the interface for database operations.
// This interface enables unit testing with mock implementations.
type Store interface {
	// User operations
	CreateUser(ctx context.Context, handle string, email *string) (*models.User, error)
	GetUserByHandle(ctx context.Context, handle string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	// Password credential operations
	CreatePasswordCredential(ctx context.Context, userID uuid.UUID, passwordHash []byte, hashAlg string) (*models.PasswordCredential, error)
	GetPasswordCredentialByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordCredential, error)

	// Refresh session operations
	CreateRefreshSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*models.RefreshSession, error)
	GetActiveRefreshSession(ctx context.Context, tokenHash []byte) (*models.RefreshSession, error)
	InvalidateRefreshSession(ctx context.Context, sessionID uuid.UUID) error
	InvalidateAllUserSessions(ctx context.Context, userID uuid.UUID) error

	// Auth audit log operations (subjectHandle, reason per postgres_schema.md)
	CreateAuthAuditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddress, userAgent, subjectHandle, reason *string) error

	// Task operations (taskName optional; when set, normalized and made unique per user).
	CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string, taskName *string, projectID ...*uuid.UUID) (*models.Task, error)
	GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	// GetTaskBySummary looks up the most recently created task matching summary for the given user.
	// Returns ErrNotFound when no match exists.
	GetTaskBySummary(ctx context.Context, userID uuid.UUID, summary string) (*models.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error
	UpdateTaskSummary(ctx context.Context, taskID uuid.UUID, summary string) error
	ListTasksByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error)
	GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error)
	CreateJob(ctx context.Context, taskID uuid.UUID, payload string) (*models.Job, error)
	CreateJobWithID(ctx context.Context, taskID, jobID uuid.UUID, payload string) (*models.Job, error)
	CreateJobCompleted(ctx context.Context, taskID, jobID uuid.UUID, result string) (*models.Job, error)
	GetJobByID(ctx context.Context, id uuid.UUID) (*models.Job, error)
	UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error
	AssignJobToNode(ctx context.Context, jobID, nodeID uuid.UUID) error
	CompleteJob(ctx context.Context, jobID uuid.UUID, result, status string) error
	GetNextQueuedJob(ctx context.Context) (*models.Job, error)

	// Node operations
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
	// ListNodes lists registered worker nodes (non-deleted), ordered by node_slug, with limit/offset pagination.
	ListNodes(ctx context.Context, limit, offset int) ([]*models.Node, error)
	UpdateNodeConfigVersion(ctx context.Context, nodeID uuid.UUID, configVersion string) error
	UpdateNodeConfigAck(ctx context.Context, nodeID uuid.UUID, configVersion, status string, ackAt time.Time, errMsg *string) error
	UpdateNodeWorkerAPIConfig(ctx context.Context, nodeID uuid.UUID, targetURL, bearerToken string) error

	// MCP tool call audit (P2-02): write one record per routed tool call (allow/deny, success/error).
	CreateMcpToolCallAuditLog(ctx context.Context, rec *models.McpToolCallAuditLog) error

	// Preference operations (P2-03): db.preference.get, list, effective, create, update, delete.
	GetPreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key string) (*models.PreferenceEntry, error)
	ListPreferences(ctx context.Context, scopeType string, scopeID *uuid.UUID, keyPrefix string, limit int, cursor string) ([]*models.PreferenceEntry, string, error)
	GetEffectivePreferencesForTask(ctx context.Context, taskID uuid.UUID) (map[string]interface{}, error)
	CreatePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, reason, updatedBy *string) (*models.PreferenceEntry, error)
	UpdatePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.PreferenceEntry, error)
	DeletePreference(ctx context.Context, scopeType string, scopeID *uuid.UUID, key string, expectedVersion *int, reason *string) error

	// System settings (system_setting.* MCP tools; orchestrator_bootstrap.md).
	GetSystemSetting(ctx context.Context, key string) (*models.SystemSetting, error)
	ListSystemSettings(ctx context.Context, keyPrefix string, limit int, cursor string) ([]*models.SystemSetting, string, error)
	CreateSystemSetting(ctx context.Context, key, value, valueType string, reason, updatedBy *string) (*models.SystemSetting, error)
	UpdateSystemSetting(ctx context.Context, key, value, valueType string, expectedVersion *int, reason, updatedBy *string) (*models.SystemSetting, error)
	DeleteSystemSetting(ctx context.Context, key string, expectedVersion *int, reason *string) error

	// Task artifacts (artifact.get MCP tool; REQ-ORCHES-0127 attachment ingestion).
	GetArtifactByTaskIDAndPath(ctx context.Context, taskID uuid.UUID, path string) (*models.TaskArtifact, error)
	CreateTaskArtifact(ctx context.Context, taskID uuid.UUID, path, storageRef string, sizeBytes *int64) (*models.TaskArtifact, error)
	ListArtifactPathsByTaskID(ctx context.Context, taskID uuid.UUID) ([]string, error)
	GetOrCreateDefaultProjectForUser(ctx context.Context, userID uuid.UUID) (*models.Project, error)
	GetProjectByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
	GetProjectBySlug(ctx context.Context, slug string) (*models.Project, error)
	ListAuthorizedProjectsForUser(ctx context.Context, userID uuid.UUID, q string, limit, offset int) ([]*models.Project, error)

	// Chat threads and messages (OpenAI-compatible chat API).
	GetOrCreateActiveChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.ChatThread, error)
	// CreateChatThread unconditionally creates a new thread for (userID, projectID), bypassing the inactivity reuse window.
	// Title is optional. Use when the user explicitly requests a new conversation (e.g. /thread new or --thread-new).
	CreateChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, title *string) (*models.ChatThread, error)
	// GetThreadByResponseID resolves previous_response_id to the thread that owns that response (for continuation). Returns ErrNotFound if not found or not owned by userID.
	GetThreadByResponseID(ctx context.Context, responseID string, userID uuid.UUID) (*models.ChatThread, error)
	// ListChatThreads returns threads for the user, recent-first. Optional projectID filter. Pagination via limit and offset.
	ListChatThreads(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, limit, offset int) ([]*models.ChatThread, error)
	// GetChatThreadByID returns one thread if it belongs to the user; otherwise ErrNotFound.
	GetChatThreadByID(ctx context.Context, threadID, userID uuid.UUID) (*models.ChatThread, error)
	// UpdateChatThreadTitle updates the thread title (rename). Thread must belong to userID.
	UpdateChatThreadTitle(ctx context.Context, threadID, userID uuid.UUID, title string) error
	AppendChatMessage(ctx context.Context, threadID uuid.UUID, role, content string, metadata *string) (*models.ChatMessage, error)
	// ListChatMessages returns up to limit messages for the given thread, ordered oldest-first.
	// A limit of 0 returns all messages.
	ListChatMessages(ctx context.Context, threadID uuid.UUID, limit int) ([]*models.ChatMessage, error)
	CreateChatAuditLog(ctx context.Context, rec *models.ChatAuditLog) error

	// Skills (REQ-SKILLS-*, skills_storage_and_inference.md).
	CreateSkill(ctx context.Context, name, content, scope string, ownerID *uuid.UUID, isSystem bool) (*models.Skill, error)
	GetSkillByID(ctx context.Context, id uuid.UUID) (*models.Skill, error)
	ListSkillsForUser(ctx context.Context, userID uuid.UUID, scopeFilter, ownerFilter string) ([]*models.Skill, error)
	UpdateSkill(ctx context.Context, id uuid.UUID, name, content, scope *string) (*models.Skill, error)
	DeleteSkill(ctx context.Context, id uuid.UUID) error
	EnsureDefaultSkill(ctx context.Context, content string) error

	// Workflow start gate (REQ-ORCHES-0152, REQ-ORCHES-0153, langgraph_mvp.md WorkflowStartGatePlanApproved).
	EvaluateWorkflowStartGate(ctx context.Context, task *models.Task, requestedByPMA bool) (denyReason string, err error)

	// Workflow lease and checkpoint (REQ-ORCHES-0144--0147, langgraph_mvp.md).
	AcquireTaskWorkflowLease(ctx context.Context, taskID uuid.UUID, leaseID uuid.UUID, holderID string, expiresAt time.Time) (*models.TaskWorkflowLease, error)
	ReleaseTaskWorkflowLease(ctx context.Context, taskID uuid.UUID, leaseID uuid.UUID) error
	GetTaskWorkflowLease(ctx context.Context, taskID uuid.UUID) (*models.TaskWorkflowLease, error)
	GetWorkflowCheckpoint(ctx context.Context, taskID uuid.UUID) (*models.WorkflowCheckpoint, error)
	UpsertWorkflowCheckpoint(ctx context.Context, cp *models.WorkflowCheckpoint) error

	// Access control and API egress (REQ-APIEGR-0110--0113, access_control.md).
	ListAccessControlRulesForApiCall(ctx context.Context, subjectType string, subjectID *uuid.UUID, action, resourceType string) ([]*models.AccessControlRule, error)
	CreateAccessControlAuditLog(ctx context.Context, rec *models.AccessControlAuditLog) error
	HasActiveApiCredentialForUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (bool, error)
	HasAnyActiveApiCredential(ctx context.Context) (bool, error)
}

// DB wraps GORM database operations.
type DB struct {
	db *gorm.DB
}

// Ensure DB implements Store.
var _ Store = (*DB)(nil)

// firstByID loads a record by primary key into dest (e.g. &models.User{}). Returns wrapErr of the query error.
func (db *DB) firstByID(ctx context.Context, dest interface{}, id uuid.UUID, op string) error {
	return wrapErr(db.db.WithContext(ctx).First(dest, "id = ?", id).Error, op)
}

// getByID loads a record by ID and returns it or an error.
func getByID[T any](db *DB, ctx context.Context, id uuid.UUID, op string) (*T, error) {
	var dest T
	if err := db.firstByID(ctx, &dest, id, op); err != nil {
		return nil, err
	}
	return &dest, nil
}

// firstWhere loads the first record matching column=value into dest.
func (db *DB) firstWhere(ctx context.Context, dest interface{}, column string, value interface{}, op string) error {
	return wrapErr(db.db.WithContext(ctx).Where(column+" = ?", value).First(dest).Error, op)
}

// getWhere loads the first record matching column=value and returns it or an error.
func getWhere[T any](db *DB, ctx context.Context, column string, value interface{}, op string) (*T, error) {
	var dest T
	if err := db.firstWhere(ctx, &dest, column, value, op); err != nil {
		return nil, err
	}
	return &dest, nil
}

// updateWhere updates model by whereCol=whereVal, setting updated_at and the given updates.
func (db *DB) updateWhere(ctx context.Context, model interface{}, whereCol string, whereVal interface{}, updates map[string]interface{}, op string) error {
	now := time.Now().UTC()
	updates["updated_at"] = now
	return wrapErr(db.db.WithContext(ctx).Model(model).Where(whereCol+" = ?", whereVal).Updates(updates).Error, op)
}

// createRecord creates a single record and returns wrapErr of the result.
func (db *DB) createRecord(ctx context.Context, record interface{}, op string) error {
	return wrapErr(db.db.WithContext(ctx).Create(record).Error, op)
}

// ensureAuditIDAndTime sets id and createdAt to new values if they are zero. Used by audit log creators.
func ensureAuditIDAndTime(id *uuid.UUID, createdAt *time.Time) {
	if *id == uuid.Nil {
		*id = uuid.New()
	}
	if createdAt.IsZero() {
		*createdAt = time.Now().UTC()
	}
}

// getSQLDB is overridden in tests to inject failures for coverage.
var getSQLDB = func(gormDB *gorm.DB) (*sql.DB, error) { return gormDB.DB() }

// pingDB is overridden in tests to inject ping failures for coverage.
var pingDB = func(ctx context.Context, sqlDB *sql.DB) error { return sqlDB.PingContext(ctx) }

// Open opens a database connection using GORM with the pgx driver.
func Open(ctx context.Context, dataSourceName string) (*DB, error) {
	gormDB, err := gorm.Open(postgres.Open(dataSourceName), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := getSQLDB(gormDB)
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pingDB(pingCtx, sqlDB); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{db: gormDB}, nil
}

// getSQLDBFromDB is overridden in tests to inject failures for coverage.
var getSQLDBFromDB = func(db *DB) (*sql.DB, error) { return db.db.DB() }

// Close closes the underlying database connection.
func (db *DB) Close() error {
	sqlDB, err := getSQLDBFromDB(db)
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GORM returns the underlying *gorm.DB for migrations and raw operations.
func (db *DB) GORM() *gorm.DB {
	return db.db
}

// --- User operations ---

// CreateUser creates a new user.
func (db *DB) CreateUser(ctx context.Context, handle string, email *string) (*models.User, error) {
	record := &UserRecord{
		GormModelUUID: newGormModelUUIDNow(),
		UserBase: models.UserBase{
			Handle:   handle,
			Email:    email,
			IsActive: true,
		},
	}
	if err := db.createRecord(ctx, record, "create user"); err != nil {
		return nil, err
	}
	return record.ToUser(), nil
}

// GetUserByHandle retrieves a user by handle.
func (db *DB) GetUserByHandle(ctx context.Context, handle string) (*models.User, error) {
	return getDomainWhere(db, ctx, "handle", handle, "get user by handle", (*UserRecord).ToUser)
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return getDomainByID(db, ctx, id, "get user by id", (*UserRecord).ToUser)
}

// --- Password credential operations ---

// CreatePasswordCredential creates a password credential for a user.
func (db *DB) CreatePasswordCredential(ctx context.Context, userID uuid.UUID, passwordHash []byte, hashAlg string) (*models.PasswordCredential, error) {
	record := &PasswordCredentialRecord{
		GormModelUUID: newGormModelUUIDNow(),
		PasswordCredentialBase: models.PasswordCredentialBase{
			UserID:       userID,
			PasswordHash: passwordHash,
			HashAlg:      hashAlg,
		},
	}
	if err := db.createRecord(ctx, record, "create password credential"); err != nil {
		return nil, err
	}
	return record.ToPasswordCredential(), nil
}

// GetPasswordCredentialByUserID retrieves password credential for a user.
func (db *DB) GetPasswordCredentialByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordCredential, error) {
	return getDomainWhere(db, ctx, "user_id", userID, "get password credential", (*PasswordCredentialRecord).ToPasswordCredential)
}

// --- Refresh session operations ---

// CreateRefreshSession creates a new refresh session.
func (db *DB) CreateRefreshSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*models.RefreshSession, error) {
	record := &RefreshSessionRecord{
		GormModelUUID: newGormModelUUIDNow(),
		RefreshSessionBase: models.RefreshSessionBase{
			UserID:           userID,
			RefreshTokenHash: tokenHash,
			IsActive:         true,
			ExpiresAt:        expiresAt,
		},
	}
	if err := db.createRecord(ctx, record, "create refresh session"); err != nil {
		return nil, err
	}
	return record.ToRefreshSession(), nil
}

// GetActiveRefreshSession retrieves an active refresh session by token hash.
func (db *DB) GetActiveRefreshSession(ctx context.Context, tokenHash []byte) (*models.RefreshSession, error) {
	var record RefreshSessionRecord
	err := db.db.WithContext(ctx).
		Where("refresh_token_hash = ? AND is_active = ? AND expires_at > ?", tokenHash, true, time.Now().UTC()).
		First(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get refresh session")
	}
	return record.ToRefreshSession(), nil
}

// InvalidateRefreshSession invalidates a refresh session.
func (db *DB) InvalidateRefreshSession(ctx context.Context, sessionID uuid.UUID) error {
	return db.updateWhere(ctx, &RefreshSessionRecord{}, "id", sessionID,
		map[string]interface{}{"is_active": false}, "invalidate refresh session")
}

// InvalidateAllUserSessions invalidates all sessions for a user.
func (db *DB) InvalidateAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	return db.updateWhere(ctx, &RefreshSessionRecord{}, "user_id", userID,
		map[string]interface{}{"is_active": false}, "invalidate all user sessions")
}

// --- Auth audit log operations ---

// CreateAuthAuditLog creates an auth audit log entry (subject_handle, reason per postgres_schema.md).
func (db *DB) CreateAuthAuditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddress, userAgent, subjectHandle, reason *string) error {
	record := &AuthAuditLogRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(), // Not used by AuthAuditLog but included for consistency
		},
		AuthAuditLogBase: models.AuthAuditLogBase{
			UserID:        userID,
			EventType:     eventType,
			Success:       success,
			IPAddress:     ipAddress,
			UserAgent:     userAgent,
			SubjectHandle: subjectHandle,
			Reason:        reason,
		},
	}
	return db.createRecord(ctx, record, "create auth audit log")
}
