package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

// handleThreadCommand handles /thread new, list, switch, rename. Returns a tea.Cmd for async ops, or nil.
func (m *Model) handleThreadCommand(line string) tea.Cmd {
	if m.Session == nil {
		return nil
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/thread") {
		return nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "/thread"))
	parts := strings.Fields(rest)
	sub := ""
	if len(parts) > 0 {
		sub = strings.ToLower(parts[0])
	}
	switch sub {
	case "new":
		return m.threadCommandNew()
	case "list":
		return m.threadListCmd()
	case "switch":
		return m.threadCommandSwitch(parts, rest)
	case "rename":
		return m.threadCommandRename(parts, rest)
	default:
		m.threadCommandUsage(rest)
		return nil
	}
}

func (m *Model) threadCommandNew() tea.Cmd {
	threadID, err := m.Session.NewThread(context.Background())
	if err != nil {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Error: "+err.Error())
		return nil
	}
	m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+chat.LandmarkThreadSwitched+" New thread: "+threadID)
	m.persistLastThreadToCache()
	return nil
}

func (m *Model) threadCommandSwitch(parts []string, rest string) tea.Cmd {
	if len(parts) < 2 {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Usage: /thread switch <selector> (use ordinal, id, or title from /thread list)")
		return nil
	}
	selector := strings.TrimSpace(strings.TrimPrefix(rest, "switch"))
	id, err := m.Session.ResolveThreadSelector(context.Background(), selector, 50)
	if err != nil {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Error: "+err.Error())
		return nil
	}
	m.Session.SetCurrentThreadID(id)
	m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+chat.LandmarkThreadSwitched+" Switched to thread: "+id)
	m.persistLastThreadToCache()
	return nil
}

func (m *Model) threadCommandRename(parts []string, rest string) tea.Cmd {
	if len(parts) < 2 {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Usage: /thread rename <title>")
		return nil
	}
	title := strings.TrimSpace(strings.TrimPrefix(rest, "rename"))
	title = strings.TrimSpace(title)
	if title == "" {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Usage: /thread rename <title>")
		return nil
	}
	return m.threadRenameCmd(title)
}

func (m *Model) threadCommandUsage(rest string) {
	if rest != "" {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Unknown: /thread "+rest+" (use new, list, switch, rename)")
	} else {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Thread: new, list, switch <id>, rename <title>")
	}
}

func (m *Model) threadListCmd() tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil {
			return threadListResult{err: fmt.Errorf("no session")}
		}
		items, err := m.Session.ListThreads(context.Background(), 20, 0)
		if err != nil {
			return threadListResult{err: err}
		}
		lines := []string{"--- Threads (use ordinal, id, or title with /thread switch <selector>) ---"}
		for i, t := range items {
			title := ""
			if t.Title != nil {
				title = *t.Title
			}
			if title == "" {
				title = "(no title)"
			}
			ordinal := fmt.Sprintf("%d", i+1)
			lines = append(lines, fmt.Sprintf("  %s  %s  %s", ordinal, t.ID, title))
		}
		return threadListResult{lines: lines}
	}
}

func (m *Model) threadRenameCmd(title string) tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil {
			return threadRenameResult{err: fmt.Errorf("no session")}
		}
		err := m.Session.PatchThreadTitle(context.Background(), "", title)
		return threadRenameResult{err: err}
	}
}

func (m *Model) pushInputHistory(line string) {
	if line == "" {
		return
	}
	// Prepend so newest is index 0; drop duplicates of last sent
	if len(m.InputHistory) > 0 && m.InputHistory[0] == line {
		return
	}
	m.InputHistory = append([]string{line}, m.InputHistory...)
	if len(m.InputHistory) > maxInputHistory {
		m.InputHistory = m.InputHistory[:maxInputHistory]
	}
}

// NavigateInputHistory moves through sent-message history (Ctrl+Up / Ctrl+Down).
func (m *Model) NavigateInputHistory(up bool) {
	if len(m.InputHistory) == 0 {
		return
	}
	switch {
	case up:
		switch {
		case m.InputHistoryIdx < 0:
			m.InputHistoryIdx = 0
		case m.InputHistoryIdx < len(m.InputHistory)-1:
			m.InputHistoryIdx++
		default:
			return
		}
		m.Input = m.InputHistory[m.InputHistoryIdx]
		m.syncInputCursorEnd()
	default:
		if m.InputHistoryIdx <= 0 {
			m.InputHistoryIdx = -1
			m.Input = ""
			m.inputCursor = 0
			return
		}
		m.InputHistoryIdx--
		m.Input = m.InputHistory[m.InputHistoryIdx]
		m.syncInputCursorEnd()
	}
}
