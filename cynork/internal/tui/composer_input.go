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
	m.Input, m.inputCursor = insertStringAtCursor(m.Input, m.inputCursor, s)
}

// deleteRuneBeforeCursor removes one Unicode code point before the cursor.
func (m *Model) deleteRuneBeforeCursor() {
	m.clampInputCursor()
	m.Input, m.inputCursor = deleteRuneBeforeCursorString(m.Input, m.inputCursor)
}

// moveInputCursorRune moves the cursor by one rune (dir -1 = left, +1 = right).
func (m *Model) moveInputCursorRune(dir int) {
	m.clampInputCursor()
	m.inputCursor = moveStringCursorRune(m.Input, m.inputCursor, dir)
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

// clampStringCursor clamps a byte offset to a valid UTF-8 boundary in s.
func clampStringCursor(s string, c int) int {
	if c < 0 {
		return 0
	}
	if c > len(s) {
		return len(s)
	}
	return c
}

// moveStringCursorRune moves a UTF-8 byte offset by one code point (dir -1 = left, +1 = right).
func moveStringCursorRune(s string, c int, dir int) int {
	c = clampStringCursor(s, c)
	if dir < 0 {
		if c == 0 {
			return 0
		}
		_, size := utf8.DecodeLastRuneInString(s[:c])
		return c - size
	}
	if c >= len(s) {
		return c
	}
	_, size := utf8.DecodeRuneInString(s[c:])
	return c + size
}

// deleteRuneBeforeCursorString removes one Unicode code point before c; returns new string and cursor.
func deleteRuneBeforeCursorString(s string, c int) (string, int) {
	c = clampStringCursor(s, c)
	if c == 0 {
		return s, 0
	}
	_, size := utf8.DecodeLastRuneInString(s[:c])
	return s[:c-size] + s[c:], c - size
}

// insertStringAtCursor inserts ins at byte offset c (must be UTF-8 boundary).
func insertStringAtCursor(s string, c int, ins string) (string, int) {
	c = clampStringCursor(s, c)
	return s[:c] + ins + s[c:], c + len(ins)
}

// moveStringCursorWordLeft moves to the start of the previous space-separated segment.
func moveStringCursorWordLeft(s string, pos int) int {
	pos = clampStringCursor(s, pos)
	for pos > 0 {
		r, sz := runeBeforeCursor(s, pos)
		if sz == 0 || !unicode.IsSpace(r) {
			break
		}
		pos -= sz
	}
	for pos > 0 {
		r, sz := runeBeforeCursor(s, pos)
		if sz == 0 || unicode.IsSpace(r) {
			break
		}
		pos -= sz
	}
	return pos
}

// moveStringCursorWordRight moves to the start of the next space-separated word.
func moveStringCursorWordRight(s string, pos int) int {
	pos = clampStringCursor(s, pos)
	for pos < len(s) {
		r, sz := runeAtCursor(s, pos)
		if sz == 0 || unicode.IsSpace(r) {
			break
		}
		pos += sz
	}
	for pos < len(s) {
		r, sz := runeAtCursor(s, pos)
		if sz == 0 || !unicode.IsSpace(r) {
			break
		}
		pos += sz
	}
	return pos
}

// moveInputCursorWordLeft moves to the start of the previous space-separated segment.
func (m *Model) moveInputCursorWordLeft() {
	m.clampInputCursor()
	m.inputCursor = moveStringCursorWordLeft(m.Input, m.inputCursor)
}

// moveInputCursorWordRight moves to the start of the next space-separated word.
func (m *Model) moveInputCursorWordRight() {
	m.clampInputCursor()
	m.inputCursor = moveStringCursorWordRight(m.Input, m.inputCursor)
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
	lineStart := m.lineStartByte(lineIdx)
	return m.inputCursor - lineStart
}

// lineStartByte returns the byte offset in m.Input where line lineIdx begins (0-based).
func (m *Model) lineStartByte(lineIdx int) int {
	lines := strings.Split(m.Input, "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return 0
	}
	start := 0
	for i := 0; i < lineIdx; i++ {
		start += len(lines[i]) + 1
	}
	return start
}

// cursorColumnRunes returns the rune offset from the start of line lineIdx to the cursor (clamped to that line).
func (m *Model) cursorColumnRunes(lineIdx int) int {
	lines := strings.Split(m.Input, "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return 0
	}
	line := lines[lineIdx]
	ls := m.lineStartByte(lineIdx)
	off := m.inputCursor - ls
	if off < 0 {
		return 0
	}
	if off > len(line) {
		off = len(line)
	}
	return utf8.RuneCountInString(line[:off])
}

// moveInputCursorVertical moves the cursor up (delta -1) or down (+1) one **visual** row in the
// composer (matching lipgloss word-wrap at composer width), including wrapped segments of a single
// logical line. Preserves display column when possible.
func (m *Model) moveInputCursorVertical(delta int) {
	m.clampInputCursor()
	if delta == 0 {
		return
	}
	wrapAt := composerWrapAt(m)
	total := totalVisualRows(m.Input, wrapAt)
	if total <= 1 {
		return
	}
	gr, col, ok := m.globalComposerVisualRow()
	if !ok {
		return
	}
	target := gr + delta
	if target < 0 || target >= total {
		return
	}
	m.setCursorFromGlobalComposerRow(target, col)
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
