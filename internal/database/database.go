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

        "github.com/cypher0n3/cynodeai/internal/models"
)

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

        _, err := db.ExecContext(ctx,
                `INSERT INTO users (id, handle, email, is_active, created_at, updated_at)
                 VALUES ($1, $2, $3, $4, $5, $6)`,
                user.ID, user.Handle, user.Email, user.IsActive, user.CreatedAt, user.UpdatedAt)
        if err != nil {
                return nil, fmt.Errorf("create user: %w", err)
        }
        return user, nil
}

// GetUserByHandle retrieves a user by handle.
func (db *DB) GetUserByHandle(ctx context.Context, handle string) (*models.User, error) {
        user := &models.User{}
        err := db.QueryRowContext(ctx,
                `SELECT id, handle, email, is_active, external_source, external_id, created_at, updated_at
                 FROM users WHERE handle = $1`, handle).Scan(
                &user.ID, &user.Handle, &user.Email, &user.IsActive,
                &user.ExternalSource, &user.ExternalID, &user.CreatedAt, &user.UpdatedAt)
        if errors.Is(err, sql.ErrNoRows) {
                return nil, ErrNotFound
        }
        if err != nil {
                return nil, fmt.Errorf("get user by handle: %w", err)
        }
        return user, nil
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
        user := &models.User{}
        err := db.QueryRowContext(ctx,
                `SELECT id, handle, email, is_active, external_source, external_id, created_at, updated_at
                 FROM users WHERE id = $1`, id).Scan(
                &user.ID, &user.Handle, &user.Email, &user.IsActive,
                &user.ExternalSource, &user.ExternalID, &user.CreatedAt, &user.UpdatedAt)
        if errors.Is(err, sql.ErrNoRows) {
                return nil, ErrNotFound
        }
        if err != nil {
                return nil, fmt.Errorf("get user by id: %w", err)
        }
        return user, nil
}

// --- Password Credential Operations ---

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

        _, err := db.ExecContext(ctx,
                `INSERT INTO password_credentials (id, user_id, password_hash, hash_alg, created_at, updated_at)
                 VALUES ($1, $2, $3, $4, $5, $6)`,
                cred.ID, cred.UserID, cred.PasswordHash, cred.HashAlg, cred.CreatedAt, cred.UpdatedAt)
        if err != nil {
                return nil, fmt.Errorf("create password credential: %w", err)
        }
        return cred, nil
}

// GetPasswordCredentialByUserID retrieves password credential for a user.
func (db *DB) GetPasswordCredentialByUserID(ctx context.Context, userID uuid.UUID) (*models.PasswordCredential, error) {
        cred := &models.PasswordCredential{}
        err := db.QueryRowContext(ctx,
                `SELECT id, user_id, password_hash, hash_alg, created_at, updated_at
                 FROM password_credentials WHERE user_id = $1`, userID).Scan(
                &cred.ID, &cred.UserID, &cred.PasswordHash, &cred.HashAlg, &cred.CreatedAt, &cred.UpdatedAt)
        if errors.Is(err, sql.ErrNoRows) {
                return nil, ErrNotFound
        }
        if err != nil {
                return nil, fmt.Errorf("get password credential: %w", err)
        }
        return cred, nil
}

// --- Refresh Session Operations ---

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

        _, err := db.ExecContext(ctx,
                `INSERT INTO refresh_sessions (id, user_id, refresh_token_hash, is_active, expires_at, created_at, updated_at)
                 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
                session.ID, session.UserID, session.RefreshTokenHash, session.IsActive,
                session.ExpiresAt, session.CreatedAt, session.UpdatedAt)
        if err != nil {
                return nil, fmt.Errorf("create refresh session: %w", err)
        }
        return session, nil
}

// GetActiveRefreshSession retrieves an active refresh session by token hash.
func (db *DB) GetActiveRefreshSession(ctx context.Context, tokenHash []byte) (*models.RefreshSession, error) {
        session := &models.RefreshSession{}
        err := db.QueryRowContext(ctx,
                `SELECT id, user_id, refresh_token_hash, is_active, expires_at, last_used_at, created_at, updated_at
                 FROM refresh_sessions
                 WHERE refresh_token_hash = $1 AND is_active = true AND expires_at > NOW()`, tokenHash).Scan(
                &session.ID, &session.UserID, &session.RefreshTokenHash, &session.IsActive,
                &session.ExpiresAt, &session.LastUsedAt, &session.CreatedAt, &session.UpdatedAt)
        if errors.Is(err, sql.ErrNoRows) {
                return nil, ErrNotFound
        }
        if err != nil {
                return nil, fmt.Errorf("get refresh session: %w", err)
        }
        return session, nil
}

// InvalidateRefreshSession invalidates a refresh session.
func (db *DB) InvalidateRefreshSession(ctx context.Context, sessionID uuid.UUID) error {
        _, err := db.ExecContext(ctx,
                `UPDATE refresh_sessions SET is_active = false, updated_at = $2 WHERE id = $1`,
                sessionID, time.Now().UTC())
        if err != nil {
                return fmt.Errorf("invalidate refresh session: %w", err)
        }
        return nil
}

// InvalidateAllUserSessions invalidates all sessions for a user.
func (db *DB) InvalidateAllUserSessions(ctx context.Context, userID uuid.UUID) error {
        _, err := db.ExecContext(ctx,
                `UPDATE refresh_sessions SET is_active = false, updated_at = $2 WHERE user_id = $1`,
                userID, time.Now().UTC())
        if err != nil {
                return fmt.Errorf("invalidate all user sessions: %w", err)
        }
        return nil
}

// --- Auth Audit Log Operations ---

// CreateAuthAuditLog creates an auth audit log entry.
func (db *DB) CreateAuthAuditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddress, userAgent, details *string) error {
        _, err := db.ExecContext(ctx,
                `INSERT INTO auth_audit_log (id, user_id, event_type, success, ip_address, user_agent, details, created_at)
                 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
                uuid.New(), userID, eventType, success, ipAddress, userAgent, details, time.Now().UTC())
        if err != nil {
                return fmt.Errorf("create auth audit log: %w", err)
        }
        return nil
}
