// Package tui provides the full-screen TUI for cynork. See docs/tech_specs/cynork_tui.md.
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) handleComposerAfterGlobalChords(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if next, cmd, ok := m.tryComposerChordPrimary(msg); ok {
		return next, cmd
	}
	if next, cmd, ok := m.tryComposerChordNavigation(msg); ok {
		return next, cmd
	}
	return m.handleComposerRuneInsert(msg)
}

// tryComposerChordPrimary handles newline injection, send-now, send, and backspace (low cyclomatic fan-out).
func (m *Model) tryComposerChordPrimary(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "shift+enter", "alt+enter", "ctrl+j":
		m.insertAtCursor("\n")
		m.clampSlashMenuSelection()
		return m, nil, true
	case "ctrl+enter", "ctrl+s":
		// ctrl+s: many terminals cannot emit ctrl+enter distinctly from Enter (bubbletea KeyEnter).
		next, cmd := m.handleCtrlEnterKey()
		return next, cmd, true
	case "ctrl+q":
		next, cmd := m.handleCtrlQKey()
		return next, cmd, true
	case "enter":
		next, cmd := m.handleEnterKey()
		return next, cmd, true
	case "backspace":
		m.deleteRuneBeforeCursor()
		m.clampSlashMenuSelection()
		return m, nil, true
	default:
		return nil, nil, false
	}
}

// tryComposerChordNavigation handles movement, history, tab completion, and escape when the slash menu applies.
func (m *Model) tryComposerChordNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "up":
		next, cmd := m.handleComposerUpKey()
		return next, cmd, true
	case "down":
		next, cmd := m.handleComposerDownKey()
		return next, cmd, true
	case "ctrl+up":
		next, cmd := m.handleComposerCtrlUpKey()
		return next, cmd, true
	case "ctrl+down":
		next, cmd := m.handleComposerCtrlDownKey()
		return next, cmd, true
	case "tab":
		next, cmd := m.handleComposerTabKey()
		return next, cmd, true
	case "esc":
		next, cmd := m.handleComposerEscKey()
		return next, cmd, true
	case "left":
		next, cmd := m.handleComposerMoveRuneKey(-1)
		return next, cmd, true
	case "right":
		next, cmd := m.handleComposerMoveRuneKey(1)
		return next, cmd, true
	case "ctrl+left":
		next, cmd := m.handleComposerWordKey(m.moveInputCursorWordLeft)
		return next, cmd, true
	case "ctrl+right":
		next, cmd := m.handleComposerWordKey(m.moveInputCursorWordRight)
		return next, cmd, true
	default:
		return nil, nil, false
	}
}
