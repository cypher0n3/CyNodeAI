package database

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRunMigrationsNilFS(t *testing.T) {
	db, _ := newMockDB(t)
	defer func() { _ = db.Close() }()

	// Save and restore MigrationsFS
	oldFS := MigrationsFS
	MigrationsFS = nil
	defer func() { MigrationsFS = oldFS }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := db.RunMigrations(context.Background(), logger)
	if err == nil {
		t.Fatal("expected error for nil filesystem")
	}
	if err.Error() != "migrations filesystem not set" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrationsCreateTableError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	// Save and restore MigrationsFS
	oldFS := MigrationsFS
	MigrationsFS = fstest.MapFS{}
	defer func() { MigrationsFS = oldFS }()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
		WillReturnError(errors.New("create table error"))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := db.RunMigrations(context.Background(), logger)
	if err == nil {
		t.Fatal("expected error for create table failure")
	}
}

func TestRunMigrationsQueryError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	// Save and restore MigrationsFS
	oldFS := MigrationsFS
	MigrationsFS = fstest.MapFS{}
	defer func() { MigrationsFS = oldFS }()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(`SELECT version FROM schema_migrations`).
		WillReturnError(errors.New("query error"))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := db.RunMigrations(context.Background(), logger)
	if err == nil {
		t.Fatal("expected error for query failure")
	}
}

func TestRunMigrationsSuccess(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	// Create a mock filesystem with a migration
	mockFS := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE test (id INT);"),
		},
	}

	// Save and restore MigrationsFS
	oldFS := MigrationsFS
	MigrationsFS = mockFS
	defer func() { MigrationsFS = oldFS }()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rows := sqlmock.NewRows([]string{"version"})
	mock.ExpectQuery(`SELECT version FROM schema_migrations`).
		WillReturnRows(rows)

	mock.ExpectExec(`CREATE TABLE test`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(`INSERT INTO schema_migrations`).
		WithArgs("001_init").
		WillReturnResult(sqlmock.NewResult(1, 1))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := db.RunMigrations(context.Background(), logger)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRunMigrationsSkipApplied(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	// Create a mock filesystem with a migration
	mockFS := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE test (id INT);"),
		},
	}

	// Save and restore MigrationsFS
	oldFS := MigrationsFS
	MigrationsFS = mockFS
	defer func() { MigrationsFS = oldFS }()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("001_init") // Already applied
	mock.ExpectQuery(`SELECT version FROM schema_migrations`).
		WillReturnRows(rows)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := db.RunMigrations(context.Background(), logger)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
}

func TestRunMigrationsApplyError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	// Create a mock filesystem with a migration
	mockFS := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE test (id INT);"),
		},
	}

	// Save and restore MigrationsFS
	oldFS := MigrationsFS
	MigrationsFS = mockFS
	defer func() { MigrationsFS = oldFS }()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rows := sqlmock.NewRows([]string{"version"})
	mock.ExpectQuery(`SELECT version FROM schema_migrations`).
		WillReturnRows(rows)

	mock.ExpectExec(`CREATE TABLE test`).
		WillReturnError(errors.New("migration error"))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := db.RunMigrations(context.Background(), logger)
	if err == nil {
		t.Fatal("expected error for migration failure")
	}
}

func TestGetMigrationFilesReadDirError(t *testing.T) {
	db, _ := newMockDB(t)
	defer func() { _ = db.Close() }()

	// Save and restore MigrationsFS
	oldFS := MigrationsFS
	MigrationsFS = errorFS{}
	defer func() { MigrationsFS = oldFS }()

	_, err := db.getMigrationFiles()
	if err == nil {
		t.Fatal("expected error for read dir failure")
	}
}

// errorFS is a test filesystem that always returns errors
type errorFS struct{}

func (errorFS) Open(name string) (fs.File, error) {
	return nil, errors.New("open error")
}

func (errorFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return nil, errors.New("read dir error")
}
