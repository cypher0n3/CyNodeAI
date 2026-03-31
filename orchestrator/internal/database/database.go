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
// It composes focused sub-interfaces; see store_interfaces.go.
type Store interface {
	UserStore
	TaskStore
	NodeStore
	ChatStore
	PreferenceStore
	SkillStore
	WorkflowStore
	SystemSettingsStore
	Transactional
}

// DB wraps GORM database operations.
type DB struct {
	db *gorm.DB
	// workerBearerKey is a 32-byte AES key derived from JWT secret; when set, worker_api_bearer_token is encrypted at rest (Task 6).
	workerBearerKey []byte
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

// WithTx runs fn inside a single SQL transaction.
func (db *DB) WithTx(ctx context.Context, fn func(ctx context.Context, tx Store) error) error {
	return db.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txDB := &DB{db: tx, workerBearerKey: db.workerBearerKey}
		return fn(ctx, txDB)
	})
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
