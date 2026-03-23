package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

func TestComposerCursor_UTF8Backspace(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.insertAtCursor("é") // 2 bytes in UTF-8
	if m.Input != "é" || m.inputCursor != len(m.Input) {
		t.Fatalf("after insert: Input=%q cursor=%d", m.Input, m.inputCursor)
	}
	m.deleteRuneBeforeCursor()
	if m.Input != "" || m.inputCursor != 0 {
		t.Errorf("after backspace: Input=%q cursor=%d", m.Input, m.inputCursor)
	}
}

func TestComposerCursor_WordMotion(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Input = "foo bar baz"
	m.syncInputCursorEnd()
	m.moveInputCursorWordLeft()
	if m.inputCursor != 8 { // before 'b' of "baz"
		t.Errorf("word left from end: cursor=%d want 8", m.inputCursor)
	}
	m.moveInputCursorWordLeft()
	if m.inputCursor != 4 { // before 'b' of "bar"
		t.Errorf("word left: cursor=%d want 4", m.inputCursor)
	}
	m.moveInputCursorWordRight()
	if m.inputCursor != 8 {
		t.Errorf("word right: cursor=%d want 8", m.inputCursor)
	}
	m.moveInputCursorWordRight()
	if m.inputCursor != len(m.Input) {
		t.Errorf("word right to end: cursor=%d want %d", m.inputCursor, len(m.Input))
	}
}

func TestComposerCursor_HandleKey_ArrowsAndCtrlWord(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Input = "ab cd"
	m.syncInputCursorEnd()
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if want := len("ab cd") - 1; m.inputCursor != want {
		t.Fatalf("left: cursor=%d want %d", m.inputCursor, want)
	}
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlLeft})
	if m.inputCursor != 3 { // start of "cd"
		t.Fatalf("ctrl+left: cursor=%d want 3", m.inputCursor)
	}
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlRight})
	if m.inputCursor != 5 {
		t.Fatalf("ctrl+right: cursor=%d want 5", m.inputCursor)
	}
}

func TestComposer_NewlineWithoutSend_AltEnterAndCtrlJ(t *testing.T) {
	if got := (tea.KeyMsg{Type: tea.KeyEnter, Alt: true}).String(); got != "alt+enter" {
		t.Fatalf("alt+enter string = %q", got)
	}
	if got := (tea.KeyMsg{Type: tea.KeyCtrlJ}).String(); got != "ctrl+j" {
		t.Fatalf("ctrl+j string = %q", got)
	}
	m := NewModel(&chat.Session{})
	m.Input = "hi"
	m.syncInputCursorEnd()
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	if cmd != nil {
		t.Fatalf("alt+enter cmd = %v", cmd)
	}
	if m.Input != "hi\n" {
		t.Fatalf("alt+enter: Input = %q want %q", m.Input, "hi\n")
	}
	m.Input = "x"
	m.syncInputCursorEnd()
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if m.Input != "x\n" {
		t.Fatalf("ctrl+j: Input = %q", m.Input)
	}
}

func TestVisibleComposerLineRange_KeepsCursorLine(t *testing.T) {
	m := NewModel(&chat.Session{})
	// 6 lines: cursor on line 0 should scroll window to include line 0
	var b strings.Builder
	for i := 0; i < 6; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteByte('a')
	}
	m.Input = b.String()
	m.inputCursor = 0 // first line
	start, end := m.visibleComposerLineRange(5)
	if start != 0 || end != 5 {
		t.Errorf("cursor on first line of 6: start=%d end=%d want 0,5", start, end)
	}
}
