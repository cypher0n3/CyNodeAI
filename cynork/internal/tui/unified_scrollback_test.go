package tui

import (
	"strings"
	"testing"
)

// TestUnifiedScrollbackView verifies on-screen scrollback is driven by m.Scrollback (single buffer for View()).
func TestUnifiedScrollbackView(t *testing.T) {
	t.Parallel()
	m := Model{
		Width:      80,
		Height:     24,
		Scrollback: []string{"You: hello", assistantPrefix + "world"},
		Transcript: nil,
	}
	out := m.renderScrollbackContent()
	if out == "" {
		t.Fatal("expected non-empty renderScrollbackContent")
	}
	// Content must reflect Scrollback lines (ANSI styles may wrap text).
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Fatalf("renderScrollbackContent should include scrollback text: %q", out)
	}
}
