package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// composerPromptPrefix must match view_render.go (buildComposerDisplayLines).
const composerPromptPrefix = "> "

// composerWrapAt is the cell width used when wrapping composer text. It matches lipgloss:
// inner width = m.Width - 2 (left+right border), then wrapAt = inner - leftPadding - rightPadding.
func composerWrapAt(m *Model) int {
	w := m.Width
	if w < 5 {
		return 1
	}
	return w - 4
}

// contentRowByteRanges splits one logical line of user input into soft-wrapped segments by display
// width. The first row reserves space for composerPromptPrefix on the composer line; continuation
// rows use the full wrap width. This follows the same width budget as lipgloss/cellbuf on the
// rendered composer without relying on cellbuf’s whitespace folding (which can drop spaces and
// break byte-for-byte round-trips).
func contentRowByteRanges(line string, wrapAt int) [][2]int {
	promptW := ansi.StringWidth(composerPromptPrefix)
	firstLimit := wrapAt - promptW
	if firstLimit < 1 {
		firstLimit = 1
	}
	contLimit := wrapAt
	if contLimit < 1 {
		contLimit = 1
	}
	if line == "" {
		return [][2]int{{0, 0}}
	}
	var rows [][2]int
	start := 0
	limit := firstLimit
	for start < len(line) {
		ext := byteExtentForDisplayWidth(line[start:], limit)
		if ext == 0 {
			break
		}
		end := start + ext
		rows = append(rows, [2]int{start, end})
		if end >= len(line) {
			break
		}
		start = end
		limit = contLimit
	}
	return rows
}

func byteExtentForDisplayWidth(s string, maxW int) int {
	if maxW < 1 {
		maxW = 1
	}
	w := 0
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		rw := ansi.StringWidth(string(r))
		if rw > maxW && w == 0 {
			return sz
		}
		if w+rw > maxW {
			break
		}
		w += rw
		i += sz
	}
	return i
}

func visualRowCountForLine(line string, wrapAt int) int {
	return len(contentRowByteRanges(line, wrapAt))
}

func totalVisualRows(input string, wrapAt int) int {
	lines := strings.Split(input, "\n")
	n := 0
	for _, line := range lines {
		n += visualRowCountForLine(line, wrapAt)
	}
	return n
}

// rowAndColForCursorInLine returns the visual row index within this logical line (0-based) and the
// display column from the start of that wrapped row. c is the byte offset into line (cursor in line).
func rowAndColForCursorInLine(line string, wrapAt, c int) (row, col int, ok bool) {
	if c < 0 {
		c = 0
	}
	if c > len(line) {
		c = len(line)
	}
	ranges := contentRowByteRanges(line, wrapAt)
	for ri, rg := range ranges {
		start, end := rg[0], rg[1]
		last := ri == len(ranges)-1
		var inRow bool
		if last {
			inRow = c >= start && c <= end
		} else {
			inRow = c >= start && c < end
		}
		if !inRow {
			continue
		}
		if ri == 0 {
			return ri, ansi.StringWidth(composerPromptPrefix + line[start:c]), true
		}
		return ri, ansi.StringWidth(line[start:c]), true
	}
	return 0, 0, false
}

func offsetInSegmentForDisplayCol(segment string, wantCol int) int {
	if wantCol <= 0 {
		return 0
	}
	w := 0
	i := 0
	for i < len(segment) {
		r, sz := utf8.DecodeRuneInString(segment[i:])
		rw := ansi.StringWidth(string(r))
		if w+rw > wantCol {
			return i
		}
		w += rw
		i += sz
		if w == wantCol {
			return i
		}
	}
	return len(segment)
}

func (m *Model) globalComposerVisualRow() (gr, col int, ok bool) {
	wrapAt := composerWrapAt(m)
	lines := strings.Split(m.Input, "\n")
	if len(lines) == 0 {
		return 0, 0, false
	}
	li := m.cursorLineIndex()
	if li < 0 || li >= len(lines) {
		return 0, 0, false
	}
	line := lines[li]
	ls := m.lineStartByte(li)
	c := m.inputCursor - ls
	if c < 0 {
		c = 0
	}
	if c > len(line) {
		c = len(line)
	}
	gr = 0
	for i := 0; i < li; i++ {
		gr += visualRowCountForLine(lines[i], wrapAt)
	}
	vj, col, ok := rowAndColForCursorInLine(line, wrapAt, c)
	if !ok {
		return 0, 0, false
	}
	return gr + vj, col, true
}

// composerCursorByteInRowSegment maps a display column within one wrapped row to a byte offset in line.
func composerCursorByteInRowSegment(line string, vj, start, end int, last bool, wantCol int) int {
	var seg string
	if vj == 0 {
		seg = composerPromptPrefix + line[start:end]
	} else {
		seg = line[start:end]
	}
	off := offsetInSegmentForDisplayCol(seg, wantCol)
	var c int
	if vj == 0 {
		if off <= len(composerPromptPrefix) {
			c = start
		} else {
			c = start + off - len(composerPromptPrefix)
		}
	} else {
		c = start + off
	}
	if c < start {
		c = start
	}
	if !last && c >= end {
		c = end - 1
		if c < start {
			c = start
		}
	}
	if last && c > end {
		c = end
	}
	if c > len(line) {
		c = len(line)
	}
	return c
}

func (m *Model) setCursorFromGlobalComposerRow(targetGR, wantCol int) bool {
	wrapAt := composerWrapAt(m)
	lines := strings.Split(m.Input, "\n")
	r := 0
	for li, line := range lines {
		ranges := contentRowByteRanges(line, wrapAt)
		for vj, rg := range ranges {
			if r != targetGR {
				r++
				continue
			}
			start, end := rg[0], rg[1]
			last := vj == len(ranges)-1
			c := composerCursorByteInRowSegment(line, vj, start, end, last, wantCol)
			m.inputCursor = m.lineStartByte(li) + c
			m.clampInputCursor()
			return true
		}
	}
	return false
}
