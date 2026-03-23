package tui

import "testing"

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
	m.Input = "/th"
	got := m.filteredSlashCommands()
	if len(got) < 1 {
		t.Fatalf("want matches for /th, got %d", len(got))
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
