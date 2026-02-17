package database

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// setupDB creates a mock DB and registers cleanup to close it.
func setupDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := newMockDB(t)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

// assertErrNotFound fails the test if err is nil or not ErrNotFound.
func assertErrNotFound(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// assertError fails the test if err is nil (expects a non-nil error).
func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
