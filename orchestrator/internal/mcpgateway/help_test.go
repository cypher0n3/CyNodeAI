package mcpgateway

import (
	"strings"
	"testing"
)

func TestTruncateHelp_longInput(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("x", helpMaxBytes+50)
	got := truncateHelp(s)
	if len(got) != helpMaxBytes {
		t.Fatalf("len=%d want %d", len(got), helpMaxBytes)
	}
}

func TestHelpGetMarkdown_unknownTopicUsesPathHint(t *testing.T) {
	t.Parallel()
	got := helpGetMarkdown("not-a-known-topic", "/docs/x")
	if !strings.Contains(got, "informational") {
		t.Fatalf("expected path hint, got len %d", len(got))
	}
}
