package database

import (
	"os"
	"testing"
)

// Getter/query behavior is covered by integration tests.
// Set POSTGRES_TEST_DSN and run: go test -v -run Integration ./internal/database

func TestGettersSkippedWithoutDSN(t *testing.T) {
	if os.Getenv(integrationEnv) != "" {
		return
	}
	t.Skipf("getter tests require real Postgres; set %s and run integration tests", integrationEnv)
}
