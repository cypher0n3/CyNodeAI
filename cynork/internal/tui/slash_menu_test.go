package tui

import (
	"strings"
	"testing"
)

func TestSlashMenuEligible(t *testing.T) {
	t.Parallel()
	if !slashMenuEligible("/") {
		t.Error("expected /")
	}
	if !slashMenuEligible("  /help") {
		t.Error("expected leading spaces")
	}
	if slashMenuEligible("x /help") {
		t.Error("non-whitespace before slash")
	}
}

func TestActiveComposerLine(t *testing.T) {
	t.Parallel()
	if activeComposerLine("a\n/b") != "/b" {
		t.Errorf("got %q", activeComposerLine("a\n/b"))
	}
}

func TestFilteredSlashCommands(t *testing.T) {
	t.Parallel()
	m := NewModel(nil)
	m.Input = testSlashThreadFilter
	got := m.filteredSlashCommands()
	if len(got) < 1 {
		t.Fatalf("want matches for %q, got %d", testSlashThreadFilter, len(got))
	}
	if got[0].name != "/thread" {
		t.Errorf("first match = %q", got[0].name)
	}
}

func TestFilteredSlashCommands_AuthPrefixHidesSubcommandsUntilSpace(t *testing.T) {
	t.Parallel()
	m := NewModel(nil)
	m.Input = "/auth"
	got := m.filteredSlashCommands()
	if len(got) != 1 || got[0].name != "/auth" {
		t.Fatalf("want only /auth before trailing space, got %#v", got)
	}
}

func TestFilteredSlashCommands_AuthSpaceShowsSubcommands(t *testing.T) {
	t.Parallel()
	m := NewModel(nil)
	m.Input = "/auth "
	got := m.filteredSlashCommands()
	if len(got) != 4 {
		t.Fatalf("want 4 /auth subcommands, got %d: %#v", len(got), got)
	}
	names := []string{got[0].name, got[1].name, got[2].name, got[3].name}
	for _, want := range []string{"/auth login", "/auth logout", "/auth refresh", "/auth whoami"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %q in %#v", want, names)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	t.Parallel()
	if truncateRunes("abc", -1) != "" {
		t.Errorf("maxCells < 1 should return empty")
	}
	if got := truncateRunes(testSampleWordHello, 100); got != testSampleWordHello {
		t.Errorf("short string: got %q", got)
	}
	// Force truncation: 1 cell budget clips after first rune.
	if got := truncateRunes(testSampleWordHello, 1); got != "h" {
		t.Errorf("got %q want h", got)
	}
}

func TestRenderSlashMenuBlock_NarrowWidth_TruncatesLongNames(t *testing.T) {
	t.Parallel()
	m := NewModel(nil)
	m.Width = 28
	m.Input = "/show-tool-output"
	m.slashMenuSel = 0
	out := m.renderSlashMenuBlock()
	if out == "" {
		t.Fatal("expected menu block")
	}
	// Narrow layout should still list the entry (name may be truncated with …).
	if !strings.Contains(out, "show-tool") && !strings.Contains(out, "show") {
		t.Errorf("unexpected menu: %q", truncateView(out, 500))
	}
}

func TestRenderSlashMenuBlock_WidthFloorAndNoMatch(t *testing.T) {
	t.Parallel()
	m := NewModel(nil)
	m.Width = 8
	m.Input = "/zzzznotacommand"
	block := m.renderSlashMenuBlock()
	if !strings.Contains(block, "No matching slash commands") {
		t.Fatalf("expected no-match line: %q", block)
	}
}

func TestEnsureSlashMenuScrollVisible_ScrollsForManyMatches(t *testing.T) {
	m := NewModel(nil)
	m.Input = "/"
	m.slashMenuSel = len(slashHelpCatalog) - 1
	m.ensureSlashMenuScrollVisible()
	if m.slashMenuScroll < 0 {
		t.Errorf("slashMenuScroll = %d", m.slashMenuScroll)
	}
}

func TestClampSlashMenuSelection_EmptyFilter(t *testing.T) {
	m := NewModel(nil)
	m.Input = "no slash"
	m.slashMenuSel = 5
	m.clampSlashMenuSelection()
	if m.slashMenuSel != 0 || m.slashMenuScroll != 0 {
		t.Errorf("sel=%d scroll=%d", m.slashMenuSel, m.slashMenuScroll)
	}
}

func TestReplaceActiveComposerLine_Multiline(t *testing.T) {
	m := NewModel(nil)
	m.Input = "line1\n/xx"
	m.replaceActiveComposerLine("/help ")
	if !strings.HasSuffix(m.Input, "/help ") {
		t.Errorf("Input = %q", m.Input)
	}
}

func truncateView(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
