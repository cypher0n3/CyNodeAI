package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

const testSampleWordHello = "hello"

func TestTrimMDEdges(t *testing.T) {
	got := trimMDEdges("\n\n" + testSampleWordHello + "\n\n")
	if got != testSampleWordHello {
		t.Errorf("trimMDEdges = %q, want %q", got, testSampleWordHello)
	}
}

func TestIndentLines_EmptyLine(t *testing.T) {
	got := indentLines("a\n\nb", "  ")
	if !strings.Contains(got, "  a") || !strings.Contains(got, "  b") {
		t.Errorf("got %q", got)
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

func TestModel_IsViewportScrollKey(t *testing.T) {
	m := NewModel(nil)
	if !m.isViewportScrollKey(tea.KeyMsg{Type: tea.KeyPgUp}) || !m.isViewportScrollKey(tea.KeyMsg{Type: tea.KeyPgDown}) {
		t.Error("expected PgUp/PgDown")
	}
	if m.isViewportScrollKey(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Error("enter should not scroll viewport")
	}
}

func TestEnsureScrollViewport_MinHeightAndZeroWidth(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 0
	m.ensureScrollViewport(0)
	if m.ScrollVP.Width < 1 {
		t.Errorf("ScrollVP.Width = %d", m.ScrollVP.Width)
	}
	m.ensureScrollViewport(-5)
	if m.ScrollVP.Height < 1 {
		t.Errorf("ScrollVP.Height = %d", m.ScrollVP.Height)
	}
}

func TestModel_GlamRenderAndMarkdownCache(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 80
	if got := m.glamRender("# Title\n\nbody"); got == "" {
		t.Fatal("expected rendered markdown")
	}
	if got := m.glamRender(""); got != "" {
		t.Errorf("empty glam = %q", got)
	}
}

func TestBuildComposerDisplayLines_CursorVariants(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 40
	m.Input = "abc"
	m.inputCursor = 1
	lines := m.buildComposerDisplayLines(12)
	if len(lines) == 0 {
		t.Fatal("no lines")
	}
}

func TestRenderScrollbackEntry_MetaLine(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 80
	if got := m.renderScrollbackEntry("plain meta line"); !strings.Contains(got, "plain meta") {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestWrapSystemScrollbackLines(t *testing.T) {
	if wrapSystemScrollbackLines(nil) != nil {
		t.Fatal("nil in nil out")
	}
	got := wrapSystemScrollbackLines([]string{"a", "b"})
	if len(got) != 2 || !strings.HasPrefix(got[0], scrollbackSystemLinePrefix) {
		t.Fatalf("%v", got)
	}
}

func TestGlamRender_CacheHit(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 80
	_ = m.glamRender(testSampleWordHello)
	_ = m.glamRender("world")
}

func TestRenderStyledLineWithCursor_EmptyAfter(t *testing.T) {
	base := lipgloss.NewStyle()
	s := renderStyledLineWithCursor(&base, "", 0)
	if s == "" {
		t.Fatal("expected space cursor")
	}
}

func TestRenderComposerBox_MinWidth(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 2
	_ = m.renderComposerBox([]string{"> x"})
}
