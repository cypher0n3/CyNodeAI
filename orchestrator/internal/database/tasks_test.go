package database

import (
	"os"
	"testing"
)

// Task/Job Store behavior is covered by integration tests.
// Set POSTGRES_TEST_DSN and run: go test -v -run Integration ./internal/database

func TestTasksSkippedWithoutDSN(t *testing.T) {
	if os.Getenv(integrationEnv) != "" {
		return
	}
	t.Skipf("task/job tests require real Postgres; set %s and run integration tests", integrationEnv)
}

func TestNormalizeTaskName_Empty(t *testing.T) {
	if got := normalizeTaskName(""); got != "" {
		t.Errorf("normalizeTaskName(%q) = %q, want \"\"", "", got)
	}
	if got := normalizeTaskName("   "); got != "" {
		t.Errorf("normalizeTaskName(%q) = %q, want \"\"", "   ", got)
	}
}

func TestNormalizeTaskName_RepeatedDashes(t *testing.T) {
	if got := normalizeTaskName("foo---bar"); got != "foo-bar" {
		t.Errorf("normalizeTaskName(\"foo---bar\") = %q, want \"foo-bar\"", got)
	}
	if got := normalizeTaskName("  My  Task  "); got != "my-task" {
		t.Errorf("normalizeTaskName(\"  My  Task  \") = %q, want \"my-task\"", got)
	}
}

func TestNormalizeTaskName_LowercaseAndTrim(t *testing.T) {
	if got := normalizeTaskName("AlreadyClean"); got != "alreadyclean" {
		t.Errorf("normalizeTaskName(\"AlreadyClean\") = %q, want \"alreadyclean\"", got)
	}
	if got := normalizeTaskName("-leading-trailing-"); got != "leading-trailing" {
		t.Errorf("normalizeTaskName(\"-leading-trailing-\") = %q, want \"leading-trailing\"", got)
	}
}
