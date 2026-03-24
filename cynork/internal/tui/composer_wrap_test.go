package tui

import (
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

func TestComposerWrapAt_MinWidth(t *testing.T) {
	t.Parallel()
	m := NewModel(&chat.Session{})
	m.Width = 3
	if got := composerWrapAt(m); got != 1 {
		t.Errorf("composerWrapAt = %d want 1", got)
	}
}

func TestContentRowByteRanges_multiRowNarrowWrap(t *testing.T) {
	t.Parallel()
	line := strings.Repeat("m", 80)
	rows := contentRowByteRanges(line, 12)
	if len(rows) < 2 {
		t.Fatalf("expected multiple visual rows, got %#v", rows)
	}
}

func TestContentRowByteRanges_EmptyLine(t *testing.T) {
	t.Parallel()
	rows := contentRowByteRanges("", 10)
	if len(rows) != 1 || rows[0][0] != 0 || rows[0][1] != 0 {
		t.Errorf("empty line rows = %#v", rows)
	}
}

func TestByteExtentForDisplayWidth_WideRuneFirstCol(t *testing.T) {
	t.Parallel()
	// Wide rune exceeds maxW on first position: must consume one rune (per byteExtentForDisplayWidth).
	ext := byteExtentForDisplayWidth("\U0001F600x", 1)
	if ext < 1 {
		t.Errorf("ext = %d", ext)
	}
}

func TestRowAndColForCursorInLine_LastRowEnd(t *testing.T) {
	t.Parallel()
	line := strings.Repeat("a", 50)
	wrap := 20
	ranges := contentRowByteRanges(line, wrap)
	if len(ranges) < 2 {
		t.Fatalf("expected wrap, got %#v", ranges)
	}
	last := ranges[len(ranges)-1]
	end := last[1]
	row, col, ok := rowAndColForCursorInLine(line, wrap, end)
	if !ok || row != len(ranges)-1 {
		t.Errorf("cursor at end: row=%d ok=%v ranges=%d", row, ok, len(ranges))
	}
	_ = col
}

func TestOffsetInSegmentForDisplayCol_Edges(t *testing.T) {
	t.Parallel()
	if offsetInSegmentForDisplayCol("abc", 0) != 0 {
		t.Error("want 0 for col 0")
	}
	off := offsetInSegmentForDisplayCol("abcd", 4)
	if off != len("abcd") {
		t.Errorf("off = %d", off)
	}
}

func TestGlobalComposerVisualRow_MultiLine(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 40
	m.Input = "short\n" + strings.Repeat("x", 60)
	m.inputCursor = len(m.Input)
	gr, col, ok := m.globalComposerVisualRow()
	if !ok {
		t.Fatal("expected ok")
	}
	if gr < 1 {
		t.Errorf("gr = %d", gr)
	}
	_ = col
}

func TestSetCursorFromGlobalComposerRow_RoundTrip(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 30
	m.Input = strings.Repeat("y", 80)
	m.syncInputCursorEnd()
	gr, col, ok := m.globalComposerVisualRow()
	if !ok {
		t.Fatal("ok")
	}
	if !m.setCursorFromGlobalComposerRow(gr, col) {
		t.Fatal("setCursorFromGlobalComposerRow failed")
	}
}

func TestSetCursorFromGlobalComposerRow_WrappedContinuationRow(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 25
	line := strings.Repeat("z", 120)
	m.Input = line
	m.inputCursor = 0
	if !m.setCursorFromGlobalComposerRow(1, 2) {
		t.Fatal("expected move to second visual row")
	}
	if m.inputCursor <= 0 {
		t.Errorf("cursor %d", m.inputCursor)
	}
}

func TestVisualRowCountForLine_AndTotalVisualRows(t *testing.T) {
	t.Parallel()
	if visualRowCountForLine("", 10) != 1 {
		t.Fatal("empty line one row")
	}
	if n := totalVisualRows("a\nb\n", 20); n < 2 {
		t.Fatalf("totalVisualRows = %d", n)
	}
}

func TestComposerCursorByteInRowSegment_continuationRow(t *testing.T) {
	t.Parallel()
	line := "abcdefghij"
	c := composerCursorByteInRowSegment(line, 1, 2, 8, false, 3)
	if c < 2 || c > len(line) {
		t.Fatalf("c=%d", c)
	}
}

func TestComposerCursorByteInRowSegment_firstRowPastPrompt(t *testing.T) {
	t.Parallel()
	line := "ab"
	// Past the two-cell "> " prefix, wantCol 4 targets the end of the visible segment (covers off > len(prefix)).
	c := composerCursorByteInRowSegment(line, 0, 0, len(line), true, 4)
	if c != len(line) {
		t.Fatalf("c=%d want %d (end of line)", c, len(line))
	}
}

func TestComposerCursorByteInRowSegment_notLastClampsToEndMinusOne(t *testing.T) {
	t.Parallel()
	line := strings.Repeat("x", 50)
	wrap := 10
	ranges := contentRowByteRanges(line, wrap)
	if len(ranges) < 2 {
		t.Fatalf("need wrap, got %#v", ranges)
	}
	start, end := ranges[0][0], ranges[0][1]
	c := composerCursorByteInRowSegment(line, 0, start, end, false, 9999)
	if c >= end {
		t.Fatalf("c=%d should be < end %d when not last row", c, end)
	}
}
