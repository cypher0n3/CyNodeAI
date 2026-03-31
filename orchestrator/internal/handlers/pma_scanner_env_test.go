package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestParseEnvDuration(t *testing.T) {
	const key = "PMA_PARSE_TEST_DUR"
	defaultDur := 5 * time.Second
	tests := []struct {
		name string
		env  string
		want time.Duration
		unit time.Duration
	}{
		{name: "empty_uses_default", env: "", want: defaultDur, unit: time.Second},
		{name: "invalid_uses_default", env: "not-an-int", want: defaultDur, unit: time.Second},
		{name: "non_positive_uses_default", env: "0", want: defaultDur, unit: time.Second},
		{name: "positive", env: "3", want: 3 * time.Second, unit: time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(key, tt.env)
			if got := parseEnvDuration(key, defaultDur, tt.unit); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestPmaScannerIntervalAndIdleReadEnv(t *testing.T) {
	t.Setenv("PMA_BINDING_SCAN_INTERVAL_SEC", "2")
	t.Setenv("PMA_BINDING_IDLE_TIMEOUT_MIN", "15")
	if got := pmaScannerInterval(); got != 2*time.Second {
		t.Fatalf("pmaScannerInterval: %v", got)
	}
	if got := pmaIdleTimeout(); got != 15*time.Minute {
		t.Fatalf("pmaIdleTimeout: %v", got)
	}
}

func TestRunPMABindingScanner_ExitsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testutil.NewMockDB()
	RunPMABindingScanner(ctx, db, nil)
}
