package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

func TestTranscript_SingleInFlightAssistantTurnUpdatesVisible(t *testing.T) {
	m := &Model{}
	m.appendTranscriptUser("hello")
	m.seedTranscriptAssistantInFlight()
	if len(m.Transcript) != 2 {
		t.Fatalf("Transcript len = %d, want 2", len(m.Transcript))
	}
	last := &m.Transcript[len(m.Transcript)-1]
	if last.Role != RoleAssistant || !last.InFlight {
		t.Fatalf("expected in-flight assistant turn, got %+v", last)
	}
	m.streamBuf.WriteString("part1")
	m.syncInFlightTranscriptVisible()
	if last.Content != "part1" {
		t.Errorf("Content = %q", last.Content)
	}
	m.appendTranscriptThinking("reason")
	if !strings.Contains(last.Content, "part1") {
		t.Error("thinking append should not wipe visible content on turn")
	}
	found := false
	for _, p := range last.Parts {
		if p.Kind == PartKindThinking && strings.Contains(p.Text, "reason") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected thinking part")
	}
}

func TestApplyStreamDone_FinalizesInterruptedTurnOnError(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.streamBuf.WriteString("partial")
	m.applyStreamDone(streamDoneMsg{err: fmt.Errorf("canceled")})
	last := &m.Transcript[len(m.Transcript)-1]
	if last.InFlight {
		t.Error("InFlight should be false after stream done")
	}
	if !last.Interrupted {
		t.Error("Interrupted should be true when stream ends with error")
	}
	if last.Content != "partial" {
		t.Errorf("Content = %q, want partial", last.Content)
	}
	if !strings.Contains(strings.Join(m.Scrollback, "\n"), "(stream interrupted)") {
		t.Error("scrollback should note stream interrupted")
	}
}

func TestApplyStreamDone_CanceledEmptyAppendsInterrupted(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.applyStreamDone(streamDoneMsg{err: context.Canceled})
	if m.Err != "" {
		t.Errorf("Err = %q, want empty for cancel", m.Err)
	}
	if !strings.Contains(strings.Join(m.Scrollback, "\n"), "(stream interrupted)") {
		t.Fatal("expected (stream interrupted) for cancel with no tokens")
	}
}

func TestApplyStreamDone_SuccessClearsInterrupted(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.streamBuf.WriteString("done")
	m.applyStreamDone(streamDoneMsg{})
	last := &m.Transcript[len(m.Transcript)-1]
	if last.Interrupted {
		t.Error("Interrupted should be false on success")
	}
	if last.Content != "done" {
		t.Errorf("Content = %q", last.Content)
	}
}

func TestAppendTranscriptToolCall_IsNonProsePart(t *testing.T) {
	m := &Model{}
	m.seedTranscriptAssistantInFlight()
	m.appendTranscriptToolCall("grep", `{"pattern":"x"}`)
	last := &m.Transcript[len(m.Transcript)-1]
	if len(last.Parts) != 1 || last.Parts[0].Kind != PartKindToolCall {
		t.Fatalf("Parts = %+v", last.Parts)
	}
	if last.Parts[0].Meta["name"] != "grep" {
		t.Errorf("Meta = %+v", last.Parts[0].Meta)
	}
}

func TestApplyStreamDelta_AmendmentReplacesBuffer(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix + "old"}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.streamBuf.WriteString("old")
	m.applyStreamDelta(&streamDeltaMsg{amendment: "replaced"})
	if m.streamBuf.String() != "replaced" {
		t.Errorf("streamBuf = %q", m.streamBuf.String())
	}
}

func TestApplyStreamDelta_IterationAmendmentReplacesOneSegment(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.applyStreamDelta(&streamDeltaMsg{iterationStart: true, iteration: 1})
	m.applyStreamDelta(&streamDeltaMsg{delta: "Hello world"})
	m.applyStreamDelta(&streamDeltaMsg{iterationStart: true, iteration: 2})
	m.applyStreamDelta(&streamDeltaMsg{delta: "Next part"})
	if m.streamBuf.String() != "Hello worldNext part" {
		t.Fatalf("buf = %q", m.streamBuf.String())
	}
	m.applyStreamDelta(&streamDeltaMsg{
		amendment:                "Corrected text",
		amendmentScope:           "iteration",
		amendmentTargetIteration: 1,
	})
	if m.streamIterSegs[1] != "Corrected text" || m.streamIterSegs[2] != "Next part" {
		t.Fatalf("segs = %#v", m.streamIterSegs)
	}
	if m.streamBuf.String() != "Corrected textNext part" {
		t.Errorf("buf = %q", m.streamBuf.String())
	}
}

func TestApplyStreamDelta_DefaultDeltaUpdatesScrollbackAndTranscript(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.applyStreamDelta(&streamDeltaMsg{delta: "hello"})
	last := &m.Transcript[len(m.Transcript)-1]
	if last.Content != "hello" {
		t.Errorf("transcript content = %q", last.Content)
	}
	if m.Scrollback[len(m.Scrollback)-1] != assistantPrefix+"hello" {
		t.Errorf("scrollback = %q", m.Scrollback[len(m.Scrollback)-1])
	}
}

func TestApplyStreamDelta_HeartbeatElapsedAndStatus(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.applyStreamDelta(&streamDeltaMsg{isHeartbeat: true, hbElapsed: 4, hbStatus: ""})
	if !strings.Contains(m.streamHeartbeatNote, "heartbeat") {
		t.Errorf("note = %q", m.streamHeartbeatNote)
	}
	m.applyStreamDelta(&streamDeltaMsg{isHeartbeat: true, hbElapsed: 0, hbStatus: "upstream slow"})
	if m.streamHeartbeatNote != "upstream slow" {
		t.Errorf("note = %q", m.streamHeartbeatNote)
	}
}

func TestApplyStreamDelta_IterationStartSetsPhase(t *testing.T) {
	m := &Model{}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.applyStreamDelta(&streamDeltaMsg{iterationStart: true, iteration: 2})
	last := &m.Transcript[len(m.Transcript)-1]
	if last.StreamingState.Phase != StreamingPhaseWorking {
		t.Errorf("phase = %v", last.StreamingState.Phase)
	}
}

func TestApplyStreamDelta_WithStreamChSchedulesPoll(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	ch := make(chan chat.ChatStreamDelta, 1)
	m.streamCh = ch
	_, cmd := m.applyStreamDelta(&streamDeltaMsg{delta: "x"})
	if cmd == nil {
		t.Fatal("expected cmd when streamCh set")
	}
	_, _ = m.applyStreamDelta(&streamDeltaMsg{thinking: "t"})
	_, _ = m.applyStreamDelta(&streamDeltaMsg{toolName: "n", toolArgs: "{}"})
	_, _ = m.applyStreamDelta(&streamDeltaMsg{isHeartbeat: true, hbElapsed: 1})
	_, _ = m.applyStreamDelta(&streamDeltaMsg{iterationStart: true})
}

func TestReadNextDelta_MapsChatDeltas(t *testing.T) {
	t.Run("closed_channel", func(t *testing.T) {
		c := make(chan chat.ChatStreamDelta)
		close(c)
		msg := readNextDelta(c)
		if _, ok := msg.(streamDoneMsg); !ok {
			t.Fatalf("closed chan: got %T", msg)
		}
	})
	ch := make(chan chat.ChatStreamDelta, 16)
	t.Run("done_with_error", func(t *testing.T) {
		ch <- chat.ChatStreamDelta{Done: true, Err: fmt.Errorf("eof")}
		if sd, ok := readNextDelta(ch).(streamDoneMsg); !ok || sd.err == nil {
			t.Fatalf("done: %+v", sd)
		}
	})
	cases := []struct {
		name string
		in   chat.ChatStreamDelta
		ok   func(streamDeltaMsg) bool
	}{
		{"amendment", chat.ChatStreamDelta{Amendment: "amend"}, func(m streamDeltaMsg) bool { return m.amendment == "amend" }},
		{"thinking", chat.ChatStreamDelta{Thinking: "why"}, func(m streamDeltaMsg) bool { return m.thinking == "why" }},
		{"tool", chat.ChatStreamDelta{ToolName: "fn", ToolArgs: "{}"}, func(m streamDeltaMsg) bool { return m.toolName == "fn" }},
		{"heartbeat", chat.ChatStreamDelta{IsHeartbeat: true, HeartbeatElapsed: 2, HeartbeatStatus: "wait"}, func(m streamDeltaMsg) bool { return m.isHeartbeat }},
		{"iteration", chat.ChatStreamDelta{IterationStart: true, Iteration: 3}, func(m streamDeltaMsg) bool { return m.iterationStart && m.iteration == 3 }},
		{"delta", chat.ChatStreamDelta{Delta: "text"}, func(m streamDeltaMsg) bool { return m.delta == "text" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch <- tc.in
			got, ok := readNextDelta(ch).(streamDeltaMsg)
			if !ok || !tc.ok(got) {
				t.Fatalf("got %+v ok=%v", got, ok)
			}
		})
	}
}

func TestViewStatusBar_IncludesHeartbeatTail(t *testing.T) {
	m := NewModel(&chat.Session{ProjectID: "proj", Model: "mdl", CurrentThreadID: "abcd-1234-efgh-ijkl"})
	m.Width = 120
	m.Height = 28
	m.streamHeartbeatNote = "heartbeat 2s"
	out := m.viewStatusBar()
	if !strings.Contains(out, "heartbeat 2s") {
		t.Fatalf("expected heartbeat in status: %q", out)
	}
}

func TestViewErrLine_WhenErrSet(t *testing.T) {
	m := NewModel(nil)
	m.Err = "gateway refused"
	if m.viewErrLine() == "" {
		t.Fatal("expected err line")
	}
}

func TestMergeThinkingPart_AppendsToExistingPart(t *testing.T) {
	turn := &TranscriptTurn{Parts: []TranscriptPart{{Kind: PartKindThinking, Text: "a"}}}
	mergeThinkingPart(turn, "b")
	if turn.Parts[0].Text != "ab" {
		t.Errorf("got %q", turn.Parts[0].Text)
	}
}

func TestMergeThinkingPart_NewPartIsIterationScopedBuffer(t *testing.T) {
	turn := &TranscriptTurn{}
	mergeThinkingPart(turn, "iter-a")
	mergeThinkingPart(turn, "iter-b")
	if len(turn.Parts) != 1 || turn.Parts[0].Text != "iter-aiter-b" {
		t.Errorf("parts %+v", turn.Parts)
	}
}
