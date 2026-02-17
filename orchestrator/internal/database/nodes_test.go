package database

import (
	"os"
	"testing"
)

// Node Store behavior is covered by integration tests.
// Set POSTGRES_TEST_DSN and run: go test -v -run Integration ./internal/database

func TestNodesSkippedWithoutDSN(t *testing.T) {
	if os.Getenv(integrationEnv) != "" {
		return
	}
	t.Skipf("node tests require real Postgres; set %s and run integration tests", integrationEnv)
}
