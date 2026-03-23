package tui

import "testing"

func TestTrimMDEdges(t *testing.T) {
	got := trimMDEdges("\n\nhello\n\n")
	if got != "hello" {
		t.Errorf("trimMDEdges = %q, want %q", got, "hello")
	}
}

func TestIsRolePairLine(t *testing.T) {
	if !isRolePairLine("You: hi", "Assistant: there") {
		t.Error("expected role pair")
	}
	if isRolePairLine("You: hi", "meta") {
		t.Error("meta should not pair with You")
	}
}
