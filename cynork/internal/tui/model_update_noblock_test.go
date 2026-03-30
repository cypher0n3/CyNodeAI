package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// TestUpdateNoBlock_StreamRecoveryTick verifies applyStreamRecoveryTick does not call Health synchronously (Task 4).
func TestUpdateNoBlock_StreamRecoveryTick(t *testing.T) {
	delay := 400 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	m := NewModel(&chat.Session{Client: gateway.NewClient(srv.URL)})
	m.streamRecoveryGen = 1
	start := time.Now()
	_, cmd := m.applyStreamRecoveryTick(streamRecoveryTickMsg{attempt: 1, gen: 1})
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("applyStreamRecoveryTick took %v, want < 50ms", elapsed)
	}
	if cmd == nil {
		t.Fatal("expected health cmd")
	}
	done := make(chan struct{})
	go func() {
		_ = cmd()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("health cmd should block until server responds")
	case <-time.After(20 * time.Millisecond):
	}
	<-done
}

// TestUpdateNoBlock_ThreadNew verifies handleKey returns before POST /v1/chat/threads completes (Task 4).
func TestUpdateNoBlock_ThreadNew(t *testing.T) {
	delay := 400 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		if r.URL.Path == "/v1/chat/threads" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"thread_id":"slow-tid"}`))
		}
	}))
	defer srv.Close()
	client := gateway.NewClient(srv.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	m := NewModel(session)
	m.Input = "/thread new"
	start := time.Now()
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("handleKey took %v, want < 50ms", elapsed)
	}
	if cmd == nil {
		t.Fatal("expected async cmd")
	}
	done := make(chan struct{})
	go func() {
		msg := cmd()
		_, _ = mod.Update(msg)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("thread new cmd should block until server responds")
	case <-time.After(20 * time.Millisecond):
	}
	<-done
}
