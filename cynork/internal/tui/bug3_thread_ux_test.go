package tui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/cynork/internal/tuicache"
)

// TestLogoutClearsThread verifies /auth logout clears Session.CurrentThreadID (Bug 3).
func TestLogoutClearsThread(t *testing.T) {
	provider := &mockAuthProvider{token: "t", refreshToken: "r"}
	session := &chat.Session{Client: gateway.NewClient("http://localhost")}
	session.SetToken("t")
	session.SetCurrentThreadID("stale-thread-id")
	m := NewModel(session)
	m.SetAuthProvider(provider)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/auth logout")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /auth logout should return cmd")
	}
	_ = cmd()
	if session.CurrentThreadID != "" {
		t.Fatalf("CurrentThreadID after logout = %q, want empty", session.CurrentThreadID)
	}
}

// TestLoginNewUser_NewThreadAfterStaleCleared verifies that after logout clears the thread id,
// ensure-thread selects a newly created thread instead of reusing a stale id (Bug 3).
func TestLoginNewUser(t *testing.T) {
	const wantNewID = "fresh-thread-from-post"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/chat/threads" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"thread_id":%q}`, wantNewID)
		case r.URL.Path == pathV1UsersMe && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"user-1","handle":"alice"}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client, ProjectID: "proj-1"}
	session.SetCurrentThreadID("stale-previous-user-thread")
	m := NewModel(session)
	m.SetAuthProvider(&mockAuthProvider{token: "tok", refreshToken: "r"})

	msg := m.authLogout()
	if len(msg.lines) == 0 || msg.lines[0] != "logged_out=true" {
		t.Fatalf("authLogout: %+v", msg.lines)
	}
	if session.CurrentThreadID != "" {
		t.Fatal("logout should clear CurrentThreadID before ensure")
	}

	out := m.buildEnsureThreadOutcome()
	if out.err != nil {
		t.Fatalf("buildEnsureThreadOutcome: %v", out.err)
	}
	if out.threadID != wantNewID {
		t.Fatalf("threadID = %q, want new thread %q (stale id must not be reused)", out.threadID, wantNewID)
	}
	if !out.createdNew {
		t.Fatal("expected createdNew=true when selecting a newly created thread after logout")
	}
}

// TestBuildEnsureThreadOutcome_resumesFromCache covers readResumeThreadFromCache + resolveEnsureThreadID cache branch.
func TestBuildEnsureThreadOutcome_resumesFromCache(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("CYNORK_CACHE_DIR", cacheDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathV1UsersMe && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"u1","handle":"alice","is_active":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	if err := tuicache.WriteLastThread(cacheDir, server.URL, "u1", "p1", "tid-cache"); err != nil {
		t.Fatal(err)
	}

	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client, ProjectID: "p1"}
	m := NewModel(session)
	out := m.buildEnsureThreadOutcome()
	if out.err != nil {
		t.Fatalf("buildEnsureThreadOutcome: %v", out.err)
	}
	if out.threadID != "tid-cache" {
		t.Fatalf("threadID = %q", out.threadID)
	}
	if !out.resumedFromCache {
		t.Fatal("expected resumedFromCache")
	}
}

func assertScrollbackHasLandmarkAndID(t *testing.T, m *Model, landmark, id string) {
	t.Helper()
	for _, line := range m.Scrollback {
		if strings.Contains(line, landmark) && strings.Contains(line, id) {
			return
		}
	}
	t.Fatalf("scrollback = %v; want %s containing %q", m.Scrollback, landmark, id)
}

// TestScrollbackLandmark_FromEnsureThreadResult verifies scrollback landmarks after ensure-thread apply (Bug 3).
func TestScrollbackLandmark_FromEnsureThreadResult(t *testing.T) {
	tests := []struct {
		name     string
		msg      ensureThreadResult
		landmark string
		threadID string
	}{
		{
			name: "NewThreadEnsure",
			msg: ensureThreadResult{
				threadID:         "tid-new",
				priorThreadID:    "",
				resumeSelector:   "",
				createdNew:       true,
				resumedFromCache: false,
			},
			landmark: chat.LandmarkThreadReady,
			threadID: "tid-new",
		},
		{
			name: "CacheResume",
			msg: ensureThreadResult{
				threadID:         "tid-cache",
				priorThreadID:    "tid-cache",
				resumeSelector:   "",
				createdNew:       false,
				resumedFromCache: true,
			},
			landmark: chat.LandmarkThreadSwitched,
			threadID: "tid-cache",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&chat.Session{})
			_, _ = m.Update(tt.msg)
			assertScrollbackHasLandmarkAndID(t, m, tt.landmark, tt.threadID)
		})
	}
}

// TestScrollbackLandmark_ThreadSwitch verifies THREAD_SWITCHED after /thread switch (Bug 3).
func TestScrollbackLandmark_ThreadSwitch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/threads" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			body := `{"data":[{"id":"sw-1","title":"Alpha","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}]}`
			_, _ = w.Write([]byte(body))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("t")
	session := &chat.Session{Client: client, ProjectID: "p1"}
	session.SetCurrentThreadID("old-tid")
	m := NewModel(session)
	_ = m.handleThreadCommand("/thread switch 1")
	assertScrollbackHasLandmarkAndID(t, m, chat.LandmarkThreadSwitched, "sw-1")
}
