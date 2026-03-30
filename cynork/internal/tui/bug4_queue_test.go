package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

func TestEnterBlockedWhileLoadingSemantics(t *testing.T) {
	if !EnterBlockedWhileLoading(true, false, "hello") {
		t.Fatal("plain chat blocked when loading and not streaming")
	}
	if EnterBlockedWhileLoading(true, true, "hello") {
		t.Fatal("plain chat queues when streaming")
	}
	if EnterBlockedWhileLoading(true, false, "/version") {
		t.Fatal("slash not blocked")
	}
}

func TestHandleKey_CtrlSAndCtrlQStringPaths(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{}})
	m.streamCancel = func() {}
	m.Input = "now"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		t.Fatalf("ctrl+s interrupt should return nil cmd, got %v", cmd)
	}
	if m.pendingInterruptSend != "now" {
		t.Fatalf("pendingInterruptSend = %q", m.pendingInterruptSend)
	}

	m2 := NewModel(&chat.Session{})
	m2.streamCancel = func() {}
	m2.Input = "qtext"
	_, cmd2 := m2.handleKey(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if cmd2 != nil {
		t.Fatalf("ctrl+q should return nil cmd")
	}
	if len(m2.queuedExplicit) != 1 || m2.queuedExplicit[0] != "qtext" {
		t.Fatalf("explicit queue = %v", m2.queuedExplicit)
	}
}

func TestEnterQueuesDuringStream(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{visible: "ok"}})
	m.streamCancel = func() {}
	m.Loading = true
	m.Input = "queue-me"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Enter while streaming should not start a new cmd, got %v", cmd)
	}
	if len(m.queuedAutoSend) != 1 || m.queuedAutoSend[0] != "queue-me" {
		t.Fatalf("queuedAutoSend = %v", m.queuedAutoSend)
	}
	if m.Input != "" {
		t.Fatalf("composer should clear: %q", m.Input)
	}
}

func TestCtrlEnterSends(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{visible: "x"}})
	cancelled := false
	m.streamCancel = func() { cancelled = true }
	m.Loading = true
	m.Input = "interrupt-with-this"
	_, cmd := m.handleCtrlEnterKey()
	if cmd != nil {
		t.Fatalf("interrupt path should defer send until stream done, got cmd")
	}
	if !cancelled {
		t.Fatal("expected stream cancel")
	}
	if m.pendingInterruptSend != "interrupt-with-this" {
		t.Fatalf("pendingInterruptSend = %q", m.pendingInterruptSend)
	}
}

func TestEnterNotStreaming(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{visible: "hi"}})
	m.Input = testSampleWordHello
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected stream cmd")
	}
	wantYou := fmt.Sprintf("You: %s", testSampleWordHello)
	if !strings.Contains(m.Scrollback[0], wantYou) {
		t.Fatalf("scrollback = %v", m.Scrollback)
	}
}

func TestCtrlQQueues(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{}})
	m.streamCancel = func() {}
	m.Loading = true
	m.Input = "explicit"
	_, cmd := m.handleCtrlQKey()
	if cmd != nil {
		t.Fatalf("ctrl+q should not return cmd, got %v", cmd)
	}
	if len(m.queuedExplicit) != 1 || m.queuedExplicit[0] != "explicit" {
		t.Fatalf("queuedExplicit = %v", m.queuedExplicit)
	}
	if len(m.queuedAutoSend) != 0 {
		t.Fatalf("auto queue should be empty: %v", m.queuedAutoSend)
	}
}

func TestQueueFIFO(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{visible: "x"}})
	m.queuedAutoSend = []string{"first", "second"}
	m.streamCancel = nil
	m.Loading = false
	cmd := m.maybeStartNextQueuedUserTurn(true)
	if cmd == nil {
		t.Fatal("expected cmd to send first queued draft")
	}
	msg := cmd()
	if _, ok := msg.(streamStartMsg); !ok {
		t.Fatalf("cmd() = %T want streamStartMsg", msg)
	}
	if len(m.queuedAutoSend) != 1 || m.queuedAutoSend[0] != "second" {
		t.Fatalf("after starting first queued, remainder = %v", m.queuedAutoSend)
	}
}

func TestSlashDuringStream(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.streamCancel = func() {}
	m.Loading = true
	m.Input = "/version"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("slash should dispatch cmd while streaming")
	}
}

func TestCtrlEnterEmptyDrainsExplicitWhenIdle(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{visible: "z"}})
	m.queuedExplicit = []string{"only-explicit"}
	_, cmd := m.handleCtrlEnterKey()
	if cmd == nil {
		t.Fatal("expected stream cmd")
	}
	_ = cmd()
	if len(m.queuedExplicit) != 0 {
		t.Fatalf("queue should drain: %v", m.queuedExplicit)
	}
}

func TestCtrlEnterEmptyWhileStreamingPopsQueue(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{visible: "z"}})
	m.streamCancel = func() {}
	m.queuedAutoSend = []string{"next-line"}
	_, cmd := m.handleCtrlEnterKey()
	if cmd != nil {
		t.Fatalf("expected cancel deferral, cmd=%v", cmd)
	}
	if m.pendingInterruptSend != "next-line" {
		t.Fatalf("pendingInterruptSend=%q", m.pendingInterruptSend)
	}
	if len(m.queuedAutoSend) != 0 {
		t.Fatalf("should pop from queue: %v", m.queuedAutoSend)
	}
}

func TestStreamDoneDrainsAutoQueue(t *testing.T) {
	m := NewModel(&chat.Session{Transport: &mockTransport{visible: "done"}})
	m.queuedAutoSend = []string{"after-done"}
	m.streamCancel = nil
	m.Loading = false
	upd, cmd := m.Update(streamDoneMsg{})
	mod := upd.(*Model)
	if cmd == nil {
		t.Fatal("expected follow-up cmd to send queued turn")
	}
	msg := cmd()
	if _, ok := msg.(streamStartMsg); !ok {
		t.Fatalf("got %T want streamStartMsg", msg)
	}
	if len(mod.queuedAutoSend) != 0 {
		t.Fatalf("queue should be consumed when starting next turn: %v", mod.queuedAutoSend)
	}
}
