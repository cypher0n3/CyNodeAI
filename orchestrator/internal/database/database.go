// Package database provides PostgreSQL database operations.
// See docs/tech_specs/postgres_schema.md for schema details.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// wrapQueryErr maps query/scan errors to ErrNotFound when sql.ErrNoRows, or wraps with context.
func wrapQueryErr(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

// wrapExecErr wraps an exec error with an operation name.
func wrapExecErr(err error, op string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
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

	// Auth audit log operations
	CreateAuthAuditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddress, userAgent, details *string) error

	// Task operations
	CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string) (*models.Task, error)
	GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error)
	CreateJob(ctx context.Context, taskID uuid.UUID, payload string) (*models.Job, error)

	// Node operations
	CreateNode(ctx context.Context, nodeSlug string) (*models.Node, error)
	GetNodeBySlug(ctx context.Context, slug string) (*models.Node, error)
	UpdateNodeStatus(ctx context.Context, nodeID uuid.UUID, status string) error
	UpdateNodeLastSeen(ctx context.Context, nodeID uuid.UUID) error
	SaveNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID, capJSON string) error
	UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capHash string) error
}

// DB wraps database operations.
type DB struct {
	*sql.DB
}

// Ensure DB implements Store interface
var _ Store = (*DB)(nil)

// rowScanner is satisfied by *sql.Row and *sql.Rows (both have Scan(dest ...any) error).
type rowScanner interface {
	Scan(dest ...any) error
}

// scanOne allocates *T, runs s.Scan(ptrs(t)...), and returns t or error. Shared by all single-row and row-set scanners.
func scanOne[T any](s rowScanner, ptrs func(*T) []any) (*T, error) {
	t := new(T)
	if err := s.Scan(ptrs(t)...); err != nil {
		return nil, err
	}
	return t, nil
}

// queryRowInto runs a single-row query, scans into T via scan, and returns (T, error). Use for Get* methods.
func queryRowInto[T any](db *DB, ctx context.Context, op, query string, args []any, scan func(*sql.Row) (T, error)) (T, error) {
	var zero T
	row := db.QueryRowContext(ctx, query, args...)
	t, err := scan(row)
	if err != nil {
		return zero, wrapQueryErr(err, op)
	}
	return t, nil
}

// execAndReturn runs exec and returns result on success. Use for Create* methods that return the created entity.
func execAndReturn[T any](exec func() error, result T) (T, error) {
	var zero T
	if err := exec(); err != nil {
		return zero, err
	}
	return result, nil
}

// queryRows runs a multi-row query and collects results via scanAllRows. Use for List* / Get* slice methods.
func queryRows[T any](db *DB, ctx context.Context, op, query string, args []any, scanOne func(*sql.Rows) (T, error)) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer func() { _ = rows.Close() }()
	return scanAllRows(rows, op, scanOne)
}

// execContext runs an exec statement and wraps errors with op.
func (db *DB) execContext(ctx context.Context, op, query string, args ...any) error {
	_, err := db.ExecContext(ctx, query, args...)
	return wrapExecErr(err, op)
}

// scanAllRows iterates rows and collects results via scanOne; wraps scan errors with op.
func scanAllRows[T any](rows *sql.Rows, op string, scanOne func(*sql.Rows) (T, error)) ([]T, error) {
	var out []T
	for rows.Next() {
		t, err := scanOne(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		out = append(out, t)
	}
	return out, nil
}

// Open opens a database connection.
func Open(dataSourceName string) (*DB, error) {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{db}, nil
}

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("not found")

// --- User Operations ---

const selectUserCols = `SELECT id, handle, email, is_active, external_source, external_id, created_at, updated_at FROM users`

func scanUserRow(row *sql.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(&u.ID, &u.Handle, &u.Email, &u.IsActive, &u.ExternalSource, &u.ExternalID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

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
	if err := db.execContext(ctx, "create user",
		`INSERT INTO users (id, handle, email, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID, user.Handle, user.Email, user.IsActive, user.CreatedAt, user.UpdatedAt); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByHandle retrieves a user by handle.
func (db *DB) GetUserByHandle(ctx context.Context, handle string) (*models.User, error) {
	return queryRowInto(db, ctx, "get user by handle", selectUserCols+` WHERE handle = $1`, []any{handle}, scanUserRow)
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return queryRowInto(db, ctx, "get user by id", selectUserCols+` WHERE id = $1`, []any{id}, scanUserRow)
}

// --- Password Credential Operations ---

const selectCredCols = `SELECT id, user_id, password_hash, hash_alg, created_at, updated_at FROM password_credentials`

func scanPasswordCredentialRow(row *sql.Row) (*models.PasswordCredential, error) {
	c := &models.PasswordCredential{}
	if err := row.Scan(&c.ID, &c.UserID, &c.PasswordHash, &c.HashAlg, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

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
	return execAndReturn(func() error {
		return db.execContext(ctx, "create password credential",
			`INSERT INTO password_credentials (id, user_id, password_hash, hash_alg, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			cred.ID, cred.UserID, cred.PasswordHash, cred.HashAlg, cred.CreatedAt, cred.UpdatedAt)
	}, cred)
}

// GetPasswordCredentialByUserID retrieves password credential for a user.
func (db *DB) GetPasswordCredentialByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordCredential, error) {
	return queryRowInto(db, ctx, "get password credential", selectCredCols+` WHERE user_id = $1`, []any{userID}, scanPasswordCredentialRow)
}

// --- Refresh Session Operations ---

const selectSessionCols = `SELECT id, user_id, refresh_token_hash, is_active, expires_at, last_used_at, created_at, updated_at FROM refresh_sessions`

func scanRefreshSessionRow(row *sql.Row) (*models.RefreshSession, error) {
	s := &models.RefreshSession{}
	if err := row.Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.IsActive, &s.ExpiresAt, &s.LastUsedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return s, nil
}

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
	err := db.execContext(ctx, "create refresh session",
		`INSERT INTO refresh_sessions (id, user_id, refresh_token_hash, is_active, expires_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		session.ID, session.UserID, session.RefreshTokenHash, session.IsActive, session.ExpiresAt, session.CreatedAt, session.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetActiveRefreshSession retrieves an active refresh session by token hash.
func (db *DB) GetActiveRefreshSession(ctx context.Context, tokenHash []byte) (*models.RefreshSession, error) {
	return queryRowInto(db, ctx, "get refresh session",
		selectSessionCols+` WHERE refresh_token_hash = $1 AND is_active = true AND expires_at > NOW()`, []any{tokenHash}, scanRefreshSessionRow)
}

// InvalidateRefreshSession invalidates a refresh session.
func (db *DB) InvalidateRefreshSession(ctx context.Context, sessionID uuid.UUID) error {
	return db.execContext(ctx, "invalidate refresh session",
		`UPDATE refresh_sessions SET is_active = false, updated_at = $2 WHERE id = $1`,
		sessionID, time.Now().UTC())
}

// InvalidateAllUserSessions invalidates all sessions for a user.
func (db *DB) InvalidateAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	return db.execContext(ctx, "invalidate all user sessions",
		`UPDATE refresh_sessions SET is_active = false, updated_at = $2 WHERE user_id = $1`,
		userID, time.Now().UTC())
}

// --- Auth Audit Log Operations ---

// CreateAuthAuditLog creates an auth audit log entry.
func (db *DB) CreateAuthAuditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddress, userAgent, details *string) error {
	return db.execContext(ctx, "create auth audit log",
		`INSERT INTO auth_audit_log (id, user_id, event_type, success, ip_address, user_agent, details, created_at)
                 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		uuid.New(), userID, eventType, success, ipAddress, userAgent, details, time.Now().UTC())
}
