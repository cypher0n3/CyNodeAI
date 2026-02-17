package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func newMockDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	return &DB{db}, mock
}

// --- User Tests ---

func TestCreateUser(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	email := "test@example.com"
	mock.ExpectExec(`INSERT INTO users`).
		WithArgs(sqlmock.AnyArg(), "testuser", &email, true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	user, err := db.CreateUser(context.Background(), "testuser", &email)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.Handle != "testuser" {
		t.Errorf("expected handle testuser, got %s", user.Handle)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreateUserError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`INSERT INTO users`).
		WillReturnError(errors.New("db error"))

	_, err := db.CreateUser(context.Background(), "testuser", nil)
	assertError(t, err)
}

func TestGetUserByHandle(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	userID := uuid.New()
	now := time.Now().UTC()
	email := "test@example.com"

	rows := sqlmock.NewRows([]string{"id", "handle", "email", "is_active", "external_source", "external_id", "created_at", "updated_at"}).
		AddRow(userID, "testuser", &email, true, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM users WHERE handle`).
		WithArgs("testuser").
		WillReturnRows(rows)

	user, err := db.GetUserByHandle(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("GetUserByHandle failed: %v", err)
	}
	if user.ID != userID {
		t.Errorf("expected user ID %v, got %v", userID, user.ID)
	}
}

func TestGetUserByID(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	userID := uuid.New()
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "handle", "email", "is_active", "external_source", "external_id", "created_at", "updated_at"}).
		AddRow(userID, "testuser", nil, true, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM users WHERE id`).
		WithArgs(userID).
		WillReturnRows(rows)

	user, err := db.GetUserByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if user.Handle != "testuser" {
		t.Errorf("expected handle testuser, got %s", user.Handle)
	}
}

// --- Password Credential Tests ---

func TestCreatePasswordCredential(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	userID := uuid.New()
	hash := []byte("hashed_password")

	mock.ExpectExec(`INSERT INTO password_credentials`).
		WithArgs(sqlmock.AnyArg(), userID, hash, "argon2id", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	cred, err := db.CreatePasswordCredential(context.Background(), userID, hash, "argon2id")
	if err != nil {
		t.Fatalf("CreatePasswordCredential failed: %v", err)
	}
	if cred.UserID != userID {
		t.Errorf("expected userID %v, got %v", userID, cred.UserID)
	}
}

func TestGetPasswordCredentialByUserID(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	credID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()
	hash := []byte("hashed_password")

	rows := sqlmock.NewRows([]string{"id", "user_id", "password_hash", "hash_alg", "created_at", "updated_at"}).
		AddRow(credID, userID, hash, "argon2id", now, now)

	mock.ExpectQuery(`SELECT .* FROM password_credentials WHERE user_id`).
		WithArgs(userID).
		WillReturnRows(rows)

	cred, err := db.GetPasswordCredentialByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetPasswordCredentialByUserID failed: %v", err)
	}
	if cred.ID != credID {
		t.Errorf("expected credID %v, got %v", credID, cred.ID)
	}
}

// --- Refresh Session Tests ---

func TestCreateRefreshSession(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	userID := uuid.New()
	tokenHash := []byte("token_hash")
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectExec(`INSERT INTO refresh_sessions`).
		WithArgs(sqlmock.AnyArg(), userID, tokenHash, true, expiresAt, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	session, err := db.CreateRefreshSession(context.Background(), userID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("CreateRefreshSession failed: %v", err)
	}
	if session.UserID != userID {
		t.Errorf("expected userID %v, got %v", userID, session.UserID)
	}
}

func TestGetActiveRefreshSession(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	sessionID := uuid.New()
	userID := uuid.New()
	tokenHash := []byte("token_hash")
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{"id", "user_id", "refresh_token_hash", "is_active", "expires_at", "last_used_at", "created_at", "updated_at"}).
		AddRow(sessionID, userID, tokenHash, true, expiresAt, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM refresh_sessions`).
		WithArgs(tokenHash).
		WillReturnRows(rows)

	session, err := db.GetActiveRefreshSession(context.Background(), tokenHash)
	if err != nil {
		t.Fatalf("GetActiveRefreshSession failed: %v", err)
	}
	if session.ID != sessionID {
		t.Errorf("expected sessionID %v, got %v", sessionID, session.ID)
	}
}

func TestGetActiveRefreshSessionNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	tokenHash := []byte("token_hash")
	mock.ExpectQuery(`SELECT .* FROM refresh_sessions`).
		WithArgs(tokenHash).
		WillReturnError(sql.ErrNoRows)

	_, err := db.GetActiveRefreshSession(context.Background(), tokenHash)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Auth Audit Log Tests ---

func TestCreateAuthAuditLog(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	userID := uuid.New()
	ipAddr := "127.0.0.1"
	userAgent := "test-agent"
	details := "login success"

	mock.ExpectExec(`INSERT INTO auth_audit_log`).
		WithArgs(sqlmock.AnyArg(), &userID, "login", true, &ipAddr, &userAgent, &details, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := db.CreateAuthAuditLog(context.Background(), &userID, "login", true, &ipAddr, &userAgent, &details)
	if err != nil {
		t.Fatalf("CreateAuthAuditLog failed: %v", err)
	}
}

func TestCreateAuthAuditLogNilUser(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`INSERT INTO auth_audit_log`).
		WithArgs(sqlmock.AnyArg(), nil, "login_failed", false, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := db.CreateAuthAuditLog(context.Background(), nil, "login_failed", false, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateAuthAuditLog failed: %v", err)
	}
}

// --- Error Cases Tests ---

func TestCreatePasswordCredentialError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	userID := uuid.New()
	mock.ExpectExec(`INSERT INTO password_credentials`).
		WillReturnError(errors.New("db error"))

	_, err := db.CreatePasswordCredential(context.Background(), userID, []byte("hash"), "argon2id")
	assertError(t, err)
}

func TestCreateRefreshSessionError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	userID := uuid.New()
	mock.ExpectExec(`INSERT INTO refresh_sessions`).
		WillReturnError(errors.New("db error"))

	_, err := db.CreateRefreshSession(context.Background(), userID, []byte("hash"), time.Now())
	assertError(t, err)
}

func TestGetActiveRefreshSessionError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT .* FROM refresh_sessions`).
		WillReturnError(errors.New("db error"))

	_, err := db.GetActiveRefreshSession(context.Background(), []byte("hash"))
	assertError(t, err)
}

func TestCreateAuthAuditLogError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`INSERT INTO auth_audit_log`).
		WillReturnError(errors.New("db error"))

	err := db.CreateAuthAuditLog(context.Background(), nil, "login", true, nil, nil, nil)
	assertError(t, err)
}
