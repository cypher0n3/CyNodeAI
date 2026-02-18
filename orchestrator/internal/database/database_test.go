package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// DB behavior is tested via integration tests. Set POSTGRES_TEST_DSN and run:
//
//	go test -v -run Integration ./internal/database
func TestOpen_InvalidDSN(t *testing.T) {
	ctx := context.Background()
	_, err := Open(ctx, "postgres://invalid:invalid@127.0.0.1:1/nonexistent?connect_timeout=1")
	if err == nil {
		t.Fatal("Open with invalid DSN should fail")
	}
}

func TestErrNotFound(t *testing.T) {
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Error("ErrNotFound should be errors.Is(self)")
	}
	err := wrapErr(errors.New("other"), "op")
	if errors.Is(err, ErrNotFound) {
		t.Error("wrapErr of non-ErrRecordNotFound should not be ErrNotFound")
	}
}

func TestWrapErr_Nil(t *testing.T) {
	if wrapErr(nil, "op") != nil {
		t.Error("wrapErr(nil) should return nil")
	}
}

func TestWrapErr_ErrRecordNotFound(t *testing.T) {
	err := wrapErr(gorm.ErrRecordNotFound, "get user")
	if err == nil {
		t.Fatal("wrapErr(ErrRecordNotFound) should return non-nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("wrapErr(ErrRecordNotFound) should wrap ErrNotFound")
	}
}

func TestOpen_GetSQLDBFails(t *testing.T) {
	dsn := os.Getenv(integrationEnv)
	if dsn == "" {
		t.Skipf("set %s to run", integrationEnv)
	}
	old := getSQLDB
	defer func() { getSQLDB = old }()
	getSQLDB = func(*gorm.DB) (*sql.DB, error) {
		return nil, errors.New("injected getSQLDB error")
	}
	_, err := Open(context.Background(), dsn)
	if err == nil {
		t.Fatal("Open should fail when getSQLDB fails")
	}
	if !strings.Contains(err.Error(), "get underlying sql.DB") {
		t.Errorf("Open error should mention get underlying sql.DB: %v", err)
	}
}

func TestClose_GetSQLDBFromDBFails(t *testing.T) {
	dsn := os.Getenv(integrationEnv)
	if dsn == "" {
		t.Skipf("set %s to run", integrationEnv)
	}
	db, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	old := getSQLDBFromDB
	defer func() { getSQLDBFromDB = old }()
	getSQLDBFromDB = func(*DB) (*sql.DB, error) {
		return nil, errors.New("injected getSQLDBFromDB error")
	}
	err = db.Close()
	if err == nil {
		t.Fatal("Close should fail when getSQLDBFromDB fails")
	}
}
