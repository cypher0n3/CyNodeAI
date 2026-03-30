package tui

import (
	"sync"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// TestEnsureThread_OutcomeDoesNotMutateSessionBeforeApply verifies buildEnsureThreadOutcome does not
// write Session (tea.Cmd may run off the main goroutine; REQ-TUI thread safety).
func TestEnsureThread_OutcomeDoesNotMutateSessionBeforeApply(t *testing.T) {
	server := newMockThreadServer(t, "fresh-thread-id")
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client, ProjectID: "p1"}
	m := NewModel(session)
	before := session.CurrentThreadID
	msg := m.buildEnsureThreadOutcome()
	if msg.err != nil {
		t.Fatalf("buildEnsureThreadOutcome: %v", msg.err)
	}
	if msg.threadID == "" {
		t.Fatal("expected thread id")
	}
	if session.CurrentThreadID != before {
		t.Fatalf("session mutated before Update: CurrentThreadID was %q now %q", before, session.CurrentThreadID)
	}
	updated, _ := m.Update(msg)
	mod := updated.(*Model)
	if mod.Session.CurrentThreadID != msg.threadID {
		t.Fatalf("after apply CurrentThreadID = %q want %q", mod.Session.CurrentThreadID, msg.threadID)
	}
}

// TestEnsureThread_ConcurrentReadCurrentThreadIDDuringOutcomeBuild exercises concurrent reads of
// Session.CurrentThreadID while buildEnsureThreadOutcome runs (writes to Session must not occur
// in the cmd path). Run with -race.
func TestEnsureThread_ConcurrentReadCurrentThreadIDDuringOutcomeBuild(t *testing.T) {
	server := newMockThreadServer(t, "race-thread")
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client, ProjectID: "p"}
	m := NewModel(session)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = session.CurrentThreadID
			}
		}()
	}
	_ = m.buildEnsureThreadOutcome()
	wg.Wait()
}
