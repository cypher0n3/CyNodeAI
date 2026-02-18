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

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("not found")

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

	// Task operations
	CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string) (*models.Task, error)
	GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error
	UpdateTaskSummary(ctx context.Context, taskID uuid.UUID, summary string) error
	ListTasksByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error)
	GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error)
	CreateJob(ctx context.Context, taskID uuid.UUID, payload string) (*models.Job, error)
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
	UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capHash string) error
	ListActiveNodes(ctx context.Context) ([]*models.Node, error)
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

// createReturning creates record and returns it or an error.
func createReturning[T any](db *DB, ctx context.Context, record *T, op string) (*T, error) {
	return record, db.createRecord(ctx, record, op)
}

// getSQLDB is overridden in tests to inject failures for coverage.
var getSQLDB = func(gormDB *gorm.DB) (*sql.DB, error) { return gormDB.DB() }

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

	if err := sqlDB.PingContext(pingCtx); err != nil {
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
	user := &models.User{
		ID:        uuid.New(),
		Handle:    handle,
		Email:     email,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return createReturning(db, ctx, user, "create user")
}

// GetUserByHandle retrieves a user by handle.
func (db *DB) GetUserByHandle(ctx context.Context, handle string) (*models.User, error) {
	return getWhere[models.User](db, ctx, "handle", handle, "get user by handle")
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return getByID[models.User](db, ctx, id, "get user by id")
}

// --- Password credential operations ---

// CreatePasswordCredential creates a password credential for a user.
func (db *DB) CreatePasswordCredential(ctx context.Context, userID uuid.UUID, passwordHash []byte, hashAlg string) (*models.PasswordCredential, error) {
	cred := &models.PasswordCredential{
		ID:           uuid.New(),
		UserID:       userID,
		PasswordHash: passwordHash,
		HashAlg:      hashAlg,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return createReturning(db, ctx, cred, "create password credential")
}

// GetPasswordCredentialByUserID retrieves password credential for a user.
func (db *DB) GetPasswordCredentialByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordCredential, error) {
	return getWhere[models.PasswordCredential](db, ctx, "user_id", userID, "get password credential")
}

// --- Refresh session operations ---

// CreateRefreshSession creates a new refresh session.
func (db *DB) CreateRefreshSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*models.RefreshSession, error) {
	session := &models.RefreshSession{
		ID:               uuid.New(),
		UserID:           userID,
		RefreshTokenHash: tokenHash,
		IsActive:         true,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := db.createRecord(ctx, session, "create refresh session"); err != nil {
		return nil, err
	}
	return session, nil
}

// GetActiveRefreshSession retrieves an active refresh session by token hash.
func (db *DB) GetActiveRefreshSession(ctx context.Context, tokenHash []byte) (*models.RefreshSession, error) {
	var session models.RefreshSession
	err := db.db.WithContext(ctx).
		Where("refresh_token_hash = ? AND is_active = ? AND expires_at > ?", tokenHash, true, time.Now().UTC()).
		First(&session).Error
	if err != nil {
		return nil, wrapErr(err, "get refresh session")
	}
	return &session, nil
}

// InvalidateRefreshSession invalidates a refresh session.
func (db *DB) InvalidateRefreshSession(ctx context.Context, sessionID uuid.UUID) error {
	return db.updateWhere(ctx, &models.RefreshSession{}, "id", sessionID,
		map[string]interface{}{"is_active": false}, "invalidate refresh session")
}

// InvalidateAllUserSessions invalidates all sessions for a user.
func (db *DB) InvalidateAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	return db.updateWhere(ctx, &models.RefreshSession{}, "user_id", userID,
		map[string]interface{}{"is_active": false}, "invalidate all user sessions")
}

// --- Auth audit log operations ---

// CreateAuthAuditLog creates an auth audit log entry (subject_handle, reason per postgres_schema.md).
func (db *DB) CreateAuthAuditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddress, userAgent, subjectHandle, reason *string) error {
	entry := &models.AuthAuditLog{
		ID:            uuid.New(),
		UserID:        userID,
		EventType:     eventType,
		Success:       success,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		SubjectHandle: subjectHandle,
		Reason:        reason,
		CreatedAt:     time.Now().UTC(),
	}
	return db.createRecord(ctx, entry, "create auth audit log")
}
