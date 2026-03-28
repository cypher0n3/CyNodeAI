package tui

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestStreamBDDApplyAndFinish(t *testing.T) {
	t.Parallel()
	cl := gateway.NewClient("http://localhost")
	sess := chat.NewSession(cl)
	m := NewModel(sess)
	m.StreamBDDSimulateUserMessage("hi")
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDApply(&StreamBDDDelta{Delta: "hello"})
	m.StreamBDDFinish(nil)
	last := m.Transcript[len(m.Transcript)-1]
	if last.Role != RoleAssistant || last.InFlight {
		t.Fatalf("assistant turn not finalized: %+v", last)
	}
	if last.Content != "hello" {
		t.Errorf("Content = %q", last.Content)
	}
}

func TestStreamBDDDrainChatStream_EmptyChannel(t *testing.T) {
	t.Parallel()
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	ch := make(chan chat.ChatStreamDelta)
	close(ch)
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDDrainChatStream(ch)
	if m.Loading {
		t.Error("expected Loading false after drain")
	}
}

func TestStreamBDDFinish_CanceledRetainsPartial(t *testing.T) {
	t.Parallel()
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDApply(&StreamBDDDelta{Delta: "partial"})
	m.StreamBDDFinish(context.Canceled)
	last := m.Transcript[len(m.Transcript)-1]
	if !last.Interrupted || last.Content != "partial" {
		t.Errorf("Interrupted=%v Content=%q", last.Interrupted, last.Content)
	}
}

func TestStreamBDDResetModel(t *testing.T) {
	t.Parallel()
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	m.Scrollback = []string{"x"}
	m.Transcript = []TranscriptTurn{{Role: RoleUser, Content: "u"}}
	m.Err = "e"
	m.StreamBDDResetModel()
	if len(m.Scrollback) != 0 || len(m.Transcript) != 0 || m.Err != "" {
		t.Fatalf("reset: scrollback=%d transcript=%d err=%q", len(m.Scrollback), len(m.Transcript), m.Err)
	}
}

func TestStreamBDDResetStreamingState(t *testing.T) {
	t.Parallel()
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	m.Loading = true
	m.streamBuf.WriteString("z")
	m.StreamBDDResetStreamingState()
	if m.Loading || m.streamBuf.Len() != 0 {
		t.Fatalf("loading=%v buf=%q", m.Loading, m.streamBuf.String())
	}
}

func TestStreamBDDApply_AllDeltaKinds(t *testing.T) {
	t.Parallel()
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	m.StreamBDDSimulateUserMessage("hi")
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDApply(&StreamBDDDelta{Thinking: "why"})
	m.StreamBDDApply(&StreamBDDDelta{ToolName: "fn", ToolArgs: "{}"})
	m.StreamBDDApply(&StreamBDDDelta{IsHeartbeat: true, HeartbeatElapsed: 3})
	if m.StreamBDDHeartbeatNote() == "" {
		t.Fatal("expected heartbeat note")
	}
	m.StreamBDDApply(&StreamBDDDelta{IsHeartbeat: true, HeartbeatStatus: "upstream slow"})
	m.StreamBDDApply(&StreamBDDDelta{IterationStart: true, Iteration: 2})
	m.StreamBDDApply(&StreamBDDDelta{Delta: "vis"})
	m.StreamBDDApply(&StreamBDDDelta{Amendment: "replaced"})
	m.StreamBDDFinish(nil)
	last := m.Transcript[len(m.Transcript)-1]
	if last.InFlight || last.Content != "replaced" {
		t.Fatalf("final: %+v", last)
	}
}

func TestStreamBDDFinish_RecoverableNetworkError(t *testing.T) {
	t.Parallel()
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDApply(&StreamBDDDelta{Delta: "keep"})
	err := &net.OpError{Op: "read", Net: "tcp", Err: fmt.Errorf("connection reset by peer")}
	m.StreamBDDFinish(err)
	if m.StreamBDDConnectionRecoveryState() != ConnectionStateReconnecting {
		t.Fatalf("state=%v attempt=%d", m.StreamBDDConnectionRecoveryState(), m.StreamBDDStreamRecoveryAttempt())
	}
	if m.StreamBDDStreamRecoveryAttempt() < 1 {
		t.Fatal("expected recovery attempt")
	}
}

func TestStreamBDDDrainChatStream_WithDeltas(t *testing.T) {
	t.Parallel()
	ch := make(chan chat.ChatStreamDelta, 4)
	ch <- chat.ChatStreamDelta{Delta: "a"}
	ch <- chat.ChatStreamDelta{Delta: "b"}
	ch <- chat.ChatStreamDelta{Done: true}
	close(ch)
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDDrainChatStream(ch)
	last := m.Transcript[len(m.Transcript)-1]
	if last.Content != "ab" || last.InFlight {
		t.Fatalf("got %+v", last)
	}
}
