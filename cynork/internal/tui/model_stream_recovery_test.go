package tui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestStreamRecoveryBackoff_Capped(t *testing.T) {
	if d := streamRecoveryBackoff(1); d != 200*time.Millisecond {
		t.Errorf("attempt 1: %v", d)
	}
	if d := streamRecoveryBackoff(10); d != 5*time.Second {
		t.Errorf("attempt 10 should cap at 5s: %v", d)
	}
}

func TestIsRecoverableGatewayStreamError(t *testing.T) {
	if isRecoverableGatewayStreamError(nil) {
		t.Error("nil")
	}
	if isRecoverableGatewayStreamError(context.Canceled) {
		t.Error("canceled")
	}
	var ne net.DNSError
	if !isRecoverableGatewayStreamError(&ne) {
		t.Error("net.Error should be recoverable")
	}
	if !isRecoverableGatewayStreamError(fmt.Errorf("read tcp: i/o timeout")) {
		t.Error("timeout string")
	}
	if isRecoverableGatewayStreamError(fmt.Errorf("400 Bad Request")) {
		t.Error("generic HTTP should not be recoverable")
	}
}

func TestApplyStreamRecoveryTick_HealthOKRestores(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	m := NewModel(&chat.Session{Client: gateway.NewClient(srv.URL)})
	m.streamRecoveryGen = 7
	m.connectionRecoveryState = ConnectionStateReconnecting
	_, cmd := m.applyStreamRecoveryTick(streamRecoveryTickMsg{attempt: 1, gen: 7})
	if cmd != nil {
		t.Errorf("expected nil cmd on success, got %v", cmd)
	}
	if m.connectionRecoveryState != ConnectionStateUnknown {
		t.Errorf("state = %q", m.connectionRecoveryState)
	}
	found := false
	for _, line := range m.Scrollback {
		if strings.Contains(line, "Gateway connection restored") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected restored scrollback line")
	}
}

func TestMaybeScheduleStreamRecovery_CanceledNoCmd(t *testing.T) {
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	if m.maybeScheduleStreamRecovery(streamDoneMsg{err: context.Canceled}) != nil {
		t.Error("expected nil cmd for cancel")
	}
}

func TestMaybeScheduleStreamRecovery_RecoverableSchedulesTick(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	m := NewModel(&chat.Session{Client: gateway.NewClient(srv.URL)})
	m.streamRecoveryGen = 11
	cmd := m.maybeScheduleStreamRecovery(streamDoneMsg{err: fmt.Errorf("read tcp: i/o timeout")})
	if cmd == nil {
		t.Fatal("expected tick cmd")
	}
	if m.connectionRecoveryState != ConnectionStateReconnecting || m.streamRecoveryAttempt != 1 {
		t.Fatalf("state=%v attempt=%d", m.connectionRecoveryState, m.streamRecoveryAttempt)
	}
	start := time.Now()
	msg := cmd()
	if time.Since(start) < 50*time.Millisecond {
		t.Error("tick should wait for backoff")
	}
	if tm, ok := msg.(streamRecoveryTickMsg); !ok || tm.gen != 11 || tm.attempt != 1 {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestMaybeScheduleStreamRecovery_NoClientDisconnects(t *testing.T) {
	m := NewModel(&chat.Session{Client: nil})
	m.maybeScheduleStreamRecovery(streamDoneMsg{err: fmt.Errorf("connection reset by peer")})
	if m.connectionRecoveryState != ConnectionStateDisconnected {
		t.Errorf("state = %v", m.connectionRecoveryState)
	}
}

func TestApplyStreamRecoveryTick_StaleGenIgnored(t *testing.T) {
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://localhost")})
	m.streamRecoveryGen = 9
	_, cmd := m.applyStreamRecoveryTick(streamRecoveryTickMsg{attempt: 1, gen: 8})
	if cmd != nil {
		t.Error("stale gen should not schedule")
	}
}

func TestApplyStreamRecoveryTick_RetrySchedulesNextTick(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	m := NewModel(&chat.Session{Client: gateway.NewClient(srv.URL)})
	m.streamRecoveryGen = 2
	_, cmd := m.applyStreamRecoveryTick(streamRecoveryTickMsg{attempt: 1, gen: 2})
	if cmd == nil {
		t.Fatal("expected retry cmd")
	}
	if m.streamRecoveryAttempt != 2 {
		t.Errorf("attempt = %d", m.streamRecoveryAttempt)
	}
}

func TestApplyStreamRecoveryTick_MaxAttemptsGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	m := NewModel(&chat.Session{Client: gateway.NewClient(srv.URL)})
	m.streamRecoveryGen = 3
	_, cmd := m.applyStreamRecoveryTick(streamRecoveryTickMsg{attempt: streamRecoveryMaxAttempts, gen: 3})
	if cmd != nil {
		t.Error("expected nil after max attempts")
	}
	if m.connectionRecoveryState != ConnectionStateDisconnected {
		t.Errorf("state = %v", m.connectionRecoveryState)
	}
}

func TestApplyStreamRecoveryTick_NoSessionClientDisconnects(t *testing.T) {
	m := NewModel(&chat.Session{Client: nil})
	m.streamRecoveryGen = 1
	_, cmd := m.applyStreamRecoveryTick(streamRecoveryTickMsg{attempt: 1, gen: 1})
	if cmd != nil {
		t.Error("expected nil")
	}
	if m.connectionRecoveryState != ConnectionStateDisconnected {
		t.Errorf("state = %v", m.connectionRecoveryState)
	}
}

func TestThinkingPart_HiddenByDefaultExpandPreservesText(t *testing.T) {
	m := &Model{}
	m.ShowThinking = false
	m.appendTranscriptUser("hi")
	m.seedTranscriptAssistantInFlight()
	m.appendTranscriptThinking("secret-plan")
	last := &m.Transcript[len(m.Transcript)-1]
	if len(last.Parts) != 1 || last.Parts[0].Kind != PartKindThinking {
		t.Fatalf("parts: %+v", last.Parts)
	}
	if !last.Parts[0].HiddenByDefault || !last.Parts[0].Collapsed {
		t.Error("thinking should start hidden/collapsed")
	}
	m.ShowThinking = true
	if last.Parts[0].Text != "secret-plan" {
		t.Error("toggle show-thinking is view state; transcript text must remain")
	}
}

func TestApplyStreamDelta_AmendmentIsPerTurnVisibleReplace(t *testing.T) {
	m := &Model{}
	m.Scrollback = []string{assistantPrefix + "visible"}
	m.appendTranscriptUser("u")
	m.seedTranscriptAssistantInFlight()
	m.streamBuf.WriteString("visible")
	m.syncInFlightTranscriptVisible()
	m.applyStreamDelta(&streamDeltaMsg{amendment: "redacted-full-turn"})
	last := &m.Transcript[len(m.Transcript)-1]
	if last.Content != "redacted-full-turn" {
		t.Errorf("per-turn amendment should replace visible accumulator: %q", last.Content)
	}
}
