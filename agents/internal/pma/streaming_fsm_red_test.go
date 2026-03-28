package pma

import (
	"strings"
	"testing"
)

func TestStreamingTokenFSM_ThinkTagsNotEmittedAsDelta(t *testing.T) {
	fsm := newStreamingTokenFSM()
	var out []streamEmitted
	// Split <think>internal reasoning</think> visible across chunks.
	for _, chunk := range []string{
		"<think>", "internal reasoning", "</think>", " visible",
	} {
		out = append(out, fsm.Feed(chunk)...)
	}
	for _, e := range out {
		if e.Kind != streamEmitDelta {
			continue
		}
		if strings.Contains(e.Text, "internal reasoning") ||
			strings.Contains(e.Text, "<think>") || strings.Contains(e.Text, "</think>") {
			t.Fatalf("delta must not carry think markup or reasoning: kind=%s text=%q (all=%v)", e.Kind, e.Text, out)
		}
	}
	var sawThinking bool
	for _, e := range out {
		if e.Kind == streamEmitThinking && strings.Contains(e.Text, "internal") {
			sawThinking = true
		}
	}
	if !sawThinking {
		t.Fatalf("expected thinking channel to carry inner reasoning, got %v", out)
	}
}

func TestStreamingTokenFSM_PartialOpenTagBufferedNotLeaked(t *testing.T) {
	fsm := newStreamingTokenFSM()
	var out []streamEmitted
	for _, chunk := range []string{"\u003c", "thin"} {
		out = append(out, fsm.Feed(chunk)...)
	}
	for _, e := range out {
		if e.Kind == streamEmitDelta && (strings.Contains(e.Text, "<") || strings.Contains(e.Text, "think")) {
			t.Fatalf("partial think open leaked to delta: %v", out)
		}
	}
}

func TestIterationOverwriteReplace_SegmentOnly(t *testing.T) {
	full := "prefixBADsuffix"
	got := iterationOverwriteReplace(full, 6, 9, "OK")
	want := "prefixOKsuffix"
	if got != want {
		t.Fatalf("iterationOverwriteReplace(%q) = %q, want %q", full, got, want)
	}
}

func TestTurnOverwriteReplace_PrefersCorrection(t *testing.T) {
	got := turnOverwriteReplace("wrong visible", "corrected")
	want := "corrected"
	if got != want {
		t.Fatalf("turnOverwriteReplace = %q, want %q", got, want)
	}
}

func TestAppendStreamBufferSecure_Accumulates(t *testing.T) {
	var dst []byte
	appendStreamBufferSecure(&dst, []byte("ab"))
	appendStreamBufferSecure(&dst, []byte("cd"))
	if string(dst) != "abcd" {
		t.Fatalf("got %q", string(dst))
	}
}

func TestIterationOverwriteReplace_InvalidBounds(t *testing.T) {
	full := "abc"
	if iterationOverwriteReplace(full, -1, 1, "x") != full {
		t.Fatal("negative start")
	}
	if iterationOverwriteReplace(full, 1, 10, "x") != full {
		t.Fatal("end past len")
	}
	if iterationOverwriteReplace(full, 2, 1, "x") != full {
		t.Fatal("end before start")
	}
}

func TestTurnOverwriteReplace_WhitespaceOnlyUsesVisible(t *testing.T) {
	got := turnOverwriteReplace("keep", "   \t\n")
	if got != "keep" {
		t.Fatalf("got %q", got)
	}
}

func TestStreamingTokenFSM_ToolBlockAndStrayCloses(t *testing.T) {
	fsm := newStreamingTokenFSM()
	open := "\u003ctool_call\u003e"
	toolCloseTag := "\u003c/tool_call\u003e"
	var out []streamEmitted
	for _, chunk := range []string{open, `{"x":1}`, toolCloseTag, " after"} {
		out = append(out, fsm.Feed(chunk)...)
	}
	out = append(out, fsm.Flush()...)
	var toolText, deltas string
	for _, e := range out {
		switch e.Kind {
		case streamEmitToolCall:
			toolText += e.Text
		case streamEmitDelta:
			deltas += e.Text
		}
	}
	if toolText != `{"x":1}` {
		t.Fatalf("tool text = %q", toolText)
	}
	if deltas != " after" {
		t.Fatalf("deltas = %q", deltas)
	}

	fsm2 := newStreamingTokenFSM()
	out2 := append([]streamEmitted(nil), fsm2.Feed("\u003c/tool_call\u003evisible")...)
	var d2 string
	for _, e := range out2 {
		if e.Kind == streamEmitDelta {
			d2 += e.Text
		}
	}
	if d2 != "visible" {
		t.Fatalf("stray close stripped, want visible, got %q", d2)
	}
}

func TestStreamingTokenFSM_ThinkEOFWithoutClose(t *testing.T) {
	fsm := newStreamingTokenFSM()
	_ = fsm.Feed("\u003cthink\u003e" + "tail")
	out := fsm.Flush()
	var thinking string
	for _, e := range out {
		if e.Kind == streamEmitThinking {
			thinking += e.Text
		}
	}
	if thinking != "tail" {
		t.Fatalf("thinking = %q", thinking)
	}
}

func TestStreamingTokenFSM_ToolEOFWithoutClose(t *testing.T) {
	fsm := newStreamingTokenFSM()
	_ = fsm.Feed("\u003ctool_call\u003ejson")
	out := fsm.Flush()
	var tool string
	for _, e := range out {
		if e.Kind == streamEmitToolCall {
			tool += e.Text
		}
	}
	if tool != "json" {
		t.Fatalf("tool = %q", tool)
	}
}
