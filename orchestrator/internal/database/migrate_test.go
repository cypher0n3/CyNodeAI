package database

import (
	"context"
	"os"
	"log/slog"
	"testing"
)

// RunSchema (AutoMigrate + DDL bootstrap) is exercised in integration tests.
// Set POSTGRES_TEST_DSN and run: go test -v -run Integration ./internal/database

func TestRunSchemaSkippedWithoutDSN(t *testing.T) {
	if os.Getenv(integrationEnv) != "" {
		return
	}
	t.Skipf("RunSchema tests require real Postgres; set %s and run integration tests", integrationEnv)
}

func TestRunSchemaWithDSN(t *testing.T) {
	dsn := os.Getenv(integrationEnv)
	if dsn == "" {
		t.Skipf("set %s to run", integrationEnv)
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.RunSchema(context.Background(), slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
}
