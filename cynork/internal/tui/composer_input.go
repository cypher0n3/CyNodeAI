package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func (m *Model) clampInputCursor() {
	if m.inputCursor < 0 {
		m.inputCursor = 0
	}
	if m.inputCursor > len(m.Input) {
		m.inputCursor = len(m.Input)
	}
}

func (m *Model) syncInputCursorEnd() {
	m.inputCursor = len(m.Input)
}

// insertAtCursor inserts s at inputCursor (must be valid UTF-8).
func (m *Model) insertAtCursor(s string) {
	m.clampInputCursor()
	m.Input = m.Input[:m.inputCursor] + s + m.Input[m.inputCursor:]
	m.inputCursor += len(s)
}

// deleteRuneBeforeCursor removes one Unicode code point before the cursor.
func (m *Model) deleteRuneBeforeCursor() {
	m.clampInputCursor()
	if m.inputCursor == 0 {
		return
	}
	_, size := utf8.DecodeLastRuneInString(m.Input[:m.inputCursor])
	m.Input = m.Input[:m.inputCursor-size] + m.Input[m.inputCursor:]
	m.inputCursor -= size
}

// moveInputCursorRune moves the cursor by one rune (dir -1 = left, +1 = right).
func (m *Model) moveInputCursorRune(dir int) {
	m.clampInputCursor()
	if dir < 0 {
		if m.inputCursor == 0 {
			return
		}
		_, size := utf8.DecodeLastRuneInString(m.Input[:m.inputCursor])
		m.inputCursor -= size
		return
	}
	if m.inputCursor >= len(m.Input) {
		return
	}
	_, size := utf8.DecodeRuneInString(m.Input[m.inputCursor:])
	m.inputCursor += size
}

func runeBeforeCursor(s string, cursorByte int) (r rune, size int) {
	if cursorByte == 0 {
		return 0, 0
	}
	return utf8.DecodeLastRuneInString(s[:cursorByte])
}

func runeAtCursor(s string, cursorByte int) (r rune, size int) {
	if cursorByte >= len(s) {
		return 0, 0
	}
	return utf8.DecodeRuneInString(s[cursorByte:])
}

// moveInputCursorWordLeft moves to the start of the previous space-separated segment.
func (m *Model) moveInputCursorWordLeft() {
	m.clampInputCursor()
	pos := m.inputCursor
	// Skip spaces left
	for pos > 0 {
		r, sz := runeBeforeCursor(m.Input, pos)
		if sz == 0 || !unicode.IsSpace(r) {
			break
		}
		pos -= sz
	}
	// Skip non-space (word) left
	for pos > 0 {
		r, sz := runeBeforeCursor(m.Input, pos)
		if sz == 0 || unicode.IsSpace(r) {
			break
		}
		pos -= sz
	}
	m.inputCursor = pos
}

// moveInputCursorWordRight moves to the start of the next space-separated word.
func (m *Model) moveInputCursorWordRight() {
	m.clampInputCursor()
	pos := m.inputCursor
	// If inside a word, move to end of this word (after last char of run of non-space)
	for pos < len(m.Input) {
		r, sz := runeAtCursor(m.Input, pos)
		if sz == 0 || unicode.IsSpace(r) {
			break
		}
		pos += sz
	}
	// Skip spaces
	for pos < len(m.Input) {
		r, sz := runeAtCursor(m.Input, pos)
		if sz == 0 || !unicode.IsSpace(r) {
			break
		}
		pos += sz
	}
	m.inputCursor = pos
}

// cursorLineIndex returns the 0-based line index in m.Input where inputCursor lies.
func (m *Model) cursorLineIndex() int {
	return strings.Count(m.Input[:m.inputCursor], "\n")
}

// cursorColumnBytes returns the byte offset within the current line (no newline) for the cursor.
func (m *Model) cursorColumnBytes(lineIdx int) int {
	lines := strings.Split(m.Input, "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return 0
	}
	lineStart := 0
	for i := 0; i < lineIdx; i++ {
		lineStart += len(lines[i]) + 1
	}
	return m.inputCursor - lineStart
}

// visibleComposerLineRange returns [startLine, endLine) line indices into strings.Split(Input, "\n")
// to show at most maxLines rows while keeping the line containing the cursor visible.
func (m *Model) visibleComposerLineRange(maxLines int) (start, end int) {
	lines := strings.Split(m.Input, "\n")
	n := len(lines)
	if n == 0 {
		return 0, 0
	}
	if n <= maxLines {
		return 0, n
	}
	cl := m.cursorLineIndex()
	start = n - maxLines
	if cl < start {
		start = cl
	}
	if cl >= start+maxLines {
		start = cl - maxLines + 1
	}
	if start < 0 {
		start = 0
	}
	end = start + maxLines
	if end > n {
		end = n
	}
	return start, end
}
