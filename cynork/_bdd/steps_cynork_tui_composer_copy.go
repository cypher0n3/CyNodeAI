package bdd

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/tui"
)

func registerCynorkTuiComposerCopy(sc *godog.ScenarioContext) {
	sc.Step(`^plain transcript for copy all excludes system-prefixed scrollback lines$`, func(_ context.Context) error {
		lines := []string{
			"You: hi",
			tui.ScrollbackSystemLinePrefix + "ignored system",
			"Assistant: hello",
		}
		got := tui.PlainTranscript(lines)
		want := "You: hi\nAssistant: hello"
		if got != want {
			return fmt.Errorf("PlainTranscript = %q, want %q", got, want)
		}
		return nil
	})

	sc.Step(`^last assistant plain text for scrollback is the latest Assistant line without prefix$`, func(_ context.Context) error {
		lines := []string{"You: x", "Assistant: last text", "You: y"}
		if got := tui.LastAssistantPlain(lines); got != "last text" {
			return fmt.Errorf("LastAssistantPlain = %q, want %q", got, "last text")
		}
		return nil
	})

	sc.Step(`^navigating input history with Ctrl\+Up then Ctrl\+Down restores the newer sent line$`, func(_ context.Context) error {
		m := tui.NewModel(&chat.Session{})
		m.InputHistory = []string{"newer", "older"}
		m.InputHistoryIdx = -1
		m.NavigateInputHistory(true)
		if m.InputHistoryIdx != 0 || m.Input != "newer" {
			return fmt.Errorf("after Ctrl+Up: idx=%d input=%q", m.InputHistoryIdx, m.Input)
		}
		m.NavigateInputHistory(true)
		if m.InputHistoryIdx != 1 || m.Input != "older" {
			return fmt.Errorf("after second Ctrl+Up: idx=%d input=%q", m.InputHistoryIdx, m.Input)
		}
		m.NavigateInputHistory(false)
		if m.InputHistoryIdx != 0 || m.Input != "newer" {
			return fmt.Errorf("after Ctrl+Down: idx=%d input=%q", m.InputHistoryIdx, m.Input)
		}
		return nil
	})

	sc.Step(`^the composer newline keys include Alt\+Enter and Ctrl\+J per spec \(not Shift\+Enter as mandatory newline\)$`, func(_ context.Context) error {
		footnote := tui.CopySelectFootnote
		if strings.Contains(footnote, "Shift+Enter MUST") {
			return fmt.Errorf("footnote should not assert Shift+Enter MUST as newline: %q", footnote)
		}
		if !strings.Contains(footnote, "Alt+Enter") || !strings.Contains(footnote, "Ctrl+J") {
			return fmt.Errorf("footnote should mention Alt+Enter and Ctrl+J: %q", footnote)
		}
		return nil
	})
}
