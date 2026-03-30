package tui

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// TestModel_Update_StreamDeltaMsg verifies that a streamDeltaMsg updates the scrollback
// in-place and schedules the next delta read (returns a non-nil cmd).
func TestModel_Update_StreamDeltaMsg(t *testing.T) {
	transport := &mockTransport{visible: "chunk1"}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	// Simulate a stream in progress: placeholder line already in scrollback.
	m.Scrollback = []string{"You: hello", assistantPrefix}
	m.Loading = true

	// Simulate a streamCh by doing a real streamCmd call.
	cmd := m.streamCmd(testSampleWordHello)
	if cmd == nil {
		t.Fatal("streamCmd returned nil")
	}
	// First message is streamStartMsg; process it to set m.streamCh.
	startMsg := cmd()
	start, ok := startMsg.(streamStartMsg)
	if !ok {
		t.Fatalf("streamCmd() returned %T, want streamStartMsg", startMsg)
	}
	m2, nextCmd := m.Update(start)
	m = m2.(*Model)
	if nextCmd == nil {
		t.Fatal("Update(streamStartMsg) returned nil cmd")
	}
	// Next cmd delivers the first delta.
	deltaMsg := nextCmd()
	if delta, ok := deltaMsg.(streamDeltaMsg); ok {
		updated, nextCmd2 := m.Update(delta)
		m = updated.(*Model)
		// The placeholder line should be updated.
		last := m.Scrollback[len(m.Scrollback)-1]
		if last != "Assistant: chunk1" {
			t.Errorf("scrollback last = %q, want %q", last, "Assistant: chunk1")
		}
		_ = nextCmd2
	}
}

// TestModel_Update_StreamDoneMsg_Success verifies that on a successful Done event
// the model is no longer loading and the final content is in scrollback.
func TestModel_Update_StreamDoneMsg_Success(t *testing.T) {
	transport := &mockTransport{visible: "final answer"}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	m.Loading = true
	m.Scrollback = []string{"You: hello", assistantPrefix}
	m.streamBuf.WriteString("final answer")

	updated, cmd := m.Update(streamDoneMsg{})
	m = updated.(*Model)
	if m.Loading {
		t.Error("Loading still true after Done")
	}
	if cmd != nil {
		t.Errorf("Update(streamDoneMsg) returned non-nil cmd")
	}
	last := m.Scrollback[len(m.Scrollback)-1]
	if last != "Assistant: final answer" {
		t.Errorf("scrollback last = %q, want %q", last, "Assistant: final answer")
	}
}

// TestModel_Update_StreamDoneMsg_Error verifies that a Done event with error and no content
// replaces the placeholder with an error line.
func TestModel_Update_StreamDoneMsg_Error(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	m.Scrollback = []string{"You: hello", assistantPrefix}

	updated, _ := m.Update(streamDoneMsg{err: errors.New("network failed")})
	m = updated.(*Model)
	if m.Loading {
		t.Error("Loading still true after Done with error")
	}
	last := m.Scrollback[len(m.Scrollback)-1]
	if !strings.HasPrefix(last, "Error:") {
		t.Errorf("scrollback last = %q, want error line", last)
	}
}

// TestModel_Update_StreamDoneMsg_PartialWithError verifies that when partial content was
// received before stream error, the partial content is kept and an interruption notice added.
func TestModel_Update_StreamDoneMsg_PartialWithError(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	m.Scrollback = []string{"You: hello", "Assistant: partial"}
	m.streamBuf.WriteString("partial")

	updated, _ := m.Update(streamDoneMsg{err: errors.New("cancelled")})
	m = updated.(*Model)
	if m.Loading {
		t.Error("Loading still true")
	}
	// Partial content kept; interruption notice appended.
	if len(m.Scrollback) < 3 {
		t.Fatalf("expected at least 3 scrollback lines, got %v", m.Scrollback)
	}
	last := m.Scrollback[len(m.Scrollback)-1]
	if !strings.Contains(last, "interrupted") {
		t.Errorf("expected interruption notice, got %q", last)
	}
}

// TestModel_Update_StreamDoneMsg_EmptyContent verifies that empty content is replaced with
// a "(no response)" placeholder.
func TestModel_Update_StreamDoneMsg_EmptyContent(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	m.Scrollback = []string{"You: hello", assistantPrefix}

	updated, _ := m.Update(streamDoneMsg{})
	m = updated.(*Model)
	last := m.Scrollback[len(m.Scrollback)-1]
	if last != "Assistant: (no response)" {
		t.Errorf("expected no response placeholder, got %q", last)
	}
}

// TestModel_HandleCtrlC_CancelsInFlightStream verifies that Ctrl+C while loading cancels
// the stream (calls cancel) and clears streamCancel.
func TestModel_HandleCtrlC_CancelsInFlightStream(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	cancelled := false
	m.streamCancel = func() { cancelled = true }

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Errorf("handleCtrlC while loading returned cmd %v", cmd)
	}
	if !cancelled {
		t.Error("streamCancel not called on Ctrl+C while loading")
	}
	if m.streamCancel != nil {
		t.Error("streamCancel not cleared after cancellation")
	}
}

// TestModel_ReadNextDelta_ClosedChannel verifies that a closed channel returns streamDoneMsg.
func TestModel_ReadNextDelta_ClosedChannel(t *testing.T) {
	ch := make(chan chat.ChatStreamDelta)
	close(ch)
	msg := readNextDelta(ch)
	_, ok := msg.(streamDoneMsg)
	if !ok {
		t.Errorf("readNextDelta on closed channel = %T, want streamDoneMsg", msg)
	}
}

// TestReadNextDelta_StreamPollTimeout covers the time.After branch when no delta is ready.
func TestReadNextDelta_StreamPollTimeout(t *testing.T) {
	ch := make(chan chat.ChatStreamDelta)
	msg := readNextDelta(ch)
	if _, ok := msg.(streamPollMsg); !ok {
		t.Fatalf("got %T want streamPollMsg", msg)
	}
}

func TestModel_Update_ExpectNilCmd_Variants(t *testing.T) {
	tests := []struct {
		name string
		prep func(*Model)
		msg  tea.Msg
	}{
		{
			name: "proactiveTokenRefreshWhenLoading",
			prep: func(m *Model) { m.Loading = true },
			msg:  proactiveTokenRefreshMsg{},
		},
		{
			name: "streamPollNilChannel",
			prep: func(m *Model) { m.streamCh = nil },
			msg:  streamPollMsg{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&chat.Session{})
			tt.prep(m)
			upd, cmd := m.Update(tt.msg)
			if cmd != nil {
				t.Errorf("cmd = %v", cmd)
			}
			_ = upd.(*Model)
		})
	}
}

func TestPushInputHistory_CapsAtMax(t *testing.T) {
	m := NewModel(&chat.Session{})
	for i := 0; i < maxInputHistory+10; i++ {
		m.pushInputHistory(fmt.Sprintf("line-%d", i))
	}
	if len(m.InputHistory) != maxInputHistory {
		t.Errorf("len = %d want %d", len(m.InputHistory), maxInputHistory)
	}
}

// TestModel_PushInputHistory_Dedup verifies that pushing the same line twice does not duplicate.
func TestModel_PushInputHistory_Dedup(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.pushInputHistory("a")
	m.pushInputHistory("a")
	if len(m.InputHistory) != 1 {
		t.Errorf("InputHistory = %v, want 1 entry", m.InputHistory)
	}
}

// ---- Slash command and shell escape tests ----

// TestModel_SlashHelp verifies /help appends command catalog to scrollback.
func TestModel_SlashHelp(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/help")})
	if cmd != nil {
		t.Fatal("typing /help should not produce cmd yet")
	}
	_, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("pressing Enter on /help should return a cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	if res.exitModel {
		t.Error("slashResultMsg.exitModel should be false for /help")
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, "/clear") || strings.Contains(l, "/exit") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("/help lines should contain /clear or /exit; got %v", res.lines)
	}
}

// TestModel_SlashClear verifies /clear produces slashResultMsg with nil lines (clears scrollback).
func TestModel_SlashClear(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Scrollback = []string{"old line"}
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/clear")})
	if cmd != nil {
		t.Fatal("typing chars should not produce cmd")
	}
	_, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /clear should return a cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	// Apply via Update.
	updated, _ := m.Update(res)
	m = updated.(*Model)
	// Slash input is not echoed; Update with nil lines clears scrollback.
	if len(m.Scrollback) != 0 {
		t.Errorf("After /clear scrollback should be empty, got %v", m.Scrollback)
	}
}

// TestModel_SlashVersion verifies /version shows "cynork" in scrollback.
func TestModel_SlashVersion(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/version")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /version should return a cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(strings.ToLower(l), "cynork") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("/version lines should contain 'cynork'; got %v", res.lines)
	}
}

// TestModel_SlashExit verifies /exit sets exitModel=true.
func TestModel_SlashExit(t *testing.T) {
	for _, slash := range []string{"/exit", "/quit"} {
		t.Run(slash, func(t *testing.T) {
			m := NewModel(&chat.Session{})
			_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(slash)})
			_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("Enter on " + slash + " should return a cmd")
			}
			msg := cmd()
			res, ok := msg.(slashResultMsg)
			if !ok {
				t.Fatalf("cmd() = %T, want slashResultMsg", msg)
			}
			if !res.exitModel {
				t.Errorf("%s should set exitModel=true", slash)
			}
		})
	}
}

// TestModel_SlashUnknown verifies unknown /command shows hint mentioning /help.
func TestModel_SlashUnknown(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/notacommand")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on unknown slash should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, "/help") || strings.Contains(strings.ToLower(l), "unknown") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unknown slash should mention /help; got %v", res.lines)
	}
	if res.exitModel {
		t.Error("unknown slash should not exit model")
	}
}

// TestModel_SlashModel_NoArg verifies /model with no arg shows current model.
func TestModel_SlashModel_NoArg(t *testing.T) {
	session := &chat.Session{Model: gateway.ModelProjectManager}
	m := NewModel(session)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/model")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /model should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, gateway.ModelProjectManager) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("/model with no arg should show routing model; got %v", res.lines)
	}
}

// TestModel_SlashSessionFields verifies /model and /project set update the session fields.
func TestModel_SlashSessionFields(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		checkFunc func(t *testing.T, session *chat.Session)
	}{
		{
			name:  "model set",
			input: "/model " + gateway.ModelProjectManager,
			checkFunc: func(t *testing.T, session *chat.Session) {
				t.Helper()
				if session.Model != gateway.ModelProjectManager {
					t.Errorf("session.Model = %q, want %q", session.Model, gateway.ModelProjectManager)
				}
			},
		},
		{
			name:  "project set",
			input: "/project set proj-xyz",
			checkFunc: func(t *testing.T, session *chat.Session) {
				t.Helper()
				if session.ProjectID != "proj-xyz" {
					t.Errorf("session.ProjectID = %q, want %q", session.ProjectID, "proj-xyz")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := &chat.Session{}
			m := NewModel(session)
			for _, r := range tc.input {
				_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
			_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatalf("Enter on %q should return cmd", tc.input)
			}
			msg := cmd()
			_, ok := msg.(slashResultMsg)
			if !ok {
				t.Fatalf("cmd() = %T, want slashResultMsg", msg)
			}
			tc.checkFunc(t, session)
		})
	}
}

// TestModel_SlashProject_NoArg verifies /project shows current project context.
func TestModel_SlashProject_NoArg(t *testing.T) {
	session := &chat.Session{ProjectID: "proj-abc"}
	m := NewModel(session)
	for _, r := range "/project" {
		_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /project should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, "proj-abc") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("/project should show 'proj-abc'; got %v", res.lines)
	}
}

// TestModel_ShellEscape_Output verifies ! echo produces output in scrollback.
func TestModel_ShellEscape_Output(t *testing.T) {
	m := NewModel(&chat.Session{})
	for _, r := range "! echo hello_test" {
		_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on ! echo should return cmd")
	}
	msg := cmd()
	done, ok := msg.(shellExecDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want shellExecDoneMsg", msg)
	}
	if done.exitCode != 0 {
		t.Errorf("echo exit code = %d, want 0", done.exitCode)
	}
	if !strings.Contains(done.output, "hello_test") {
		t.Errorf("output = %q, want 'hello_test'", done.output)
	}
	// Apply via Update to verify scrollback.
	updated, _ := m.Update(done)
	m = updated.(*Model)
	found := false
	for _, l := range m.Scrollback {
		if strings.Contains(l, "hello_test") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("scrollback should contain 'hello_test'; got %v", m.Scrollback)
	}
}

// TestModel_ShellEscape_Empty verifies ! with no command shows usage hint.
func TestModel_ShellEscape_Empty(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on ! should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(strings.ToLower(l), "usage") || strings.Contains(l, "!") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("empty ! should show usage; got %v", res.lines)
	}
}

// TestModel_ShellEscape_NonzeroExit verifies non-zero exit code appears in scrollback.
func TestModel_ShellEscape_NonzeroExit(t *testing.T) {
	m := NewModel(&chat.Session{})
	for _, r := range "! sh -c 'exit 7'" {
		_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on ! sh -c 'exit 7' should return cmd")
	}
	msg := cmd()
	done, ok := msg.(shellExecDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want shellExecDoneMsg", msg)
	}
	if done.exitCode != 7 {
		t.Errorf("exit code = %d, want 7", done.exitCode)
	}
	updated, _ := m.Update(done)
	m = updated.(*Model)
	found := false
	for _, l := range m.Scrollback {
		if strings.Contains(l, "7") && strings.Contains(strings.ToLower(l), "exit") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("scrollback should show exit code 7; got %v", m.Scrollback)
	}
}

// TestModel_View_ContainsComposerHint verifies composerHint is in the status bar.
func TestModel_View_ContainsComposerHint(t *testing.T) {
	m := NewModel(&chat.Session{})
	v := m.View()
	nw := strings.Join(strings.Fields(v), " ")
	if !strings.Contains(nw, composerHint) {
		t.Errorf("View() should contain composerHint %q; got: %s", composerHint, truncate(v, 200))
	}
}

// mockAuthProvider implements AuthProvider for tests.
type mockAuthProvider struct {
	token, refreshToken     string
	gatewayURL              string
	saveErr                 error
	saved                   bool
	showThinkingByDefault   bool
	showToolOutputByDefault bool
}

func (m *mockAuthProvider) Token() string        { return m.token }
func (m *mockAuthProvider) RefreshToken() string { return m.refreshToken }
func (m *mockAuthProvider) GatewayURL() string   { return m.gatewayURL }
func (m *mockAuthProvider) SetTokens(access, refresh string) {
	m.token, m.refreshToken = access, refresh
}
func (m *mockAuthProvider) SetGatewayURL(url string, _ bool) { m.gatewayURL = url }
func (m *mockAuthProvider) Save() error {
	m.saved = true
	return m.saveErr
}
func (m *mockAuthProvider) ShowThinkingByDefault() bool { return m.showThinkingByDefault }
func (m *mockAuthProvider) SetShowThinkingByDefault(v bool) {
	m.showThinkingByDefault = v
}
func (m *mockAuthProvider) ShowToolOutputByDefault() bool { return m.showToolOutputByDefault }
func (m *mockAuthProvider) SetShowToolOutputByDefault(v bool) {
	m.showToolOutputByDefault = v
}

// TestModel_SlashAuth_NoArg verifies /auth with no args shows usage (login, logout, whoami, refresh).
func TestModel_SlashAuth_NoArg(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/auth")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /auth should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	wantSubs := []string{"login", "logout", "whoami", "refresh"}
	for _, sub := range wantSubs {
		found := false
		for _, l := range res.lines {
			if strings.Contains(l, "auth "+sub) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("/auth lines should contain 'auth %s'; got %v", sub, res.lines)
		}
	}
}

// TestModel_View_LoginFormShowsLandmark verifies the login overlay includes the auth-recovery landmark for PTY.
func TestModel_View_LoginFormShowsLandmark(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.LoginGatewayURL = "http://localhost:12080"
	m.Width = 80
	m.Height = 24
	v := m.View()
	if !strings.Contains(v, chat.LandmarkAuthRecoveryReady) {
		t.Errorf("View() with login form should contain %q for PTY; got (len=%d)",
			chat.LandmarkAuthRecoveryReady, len(v))
	}
	if !strings.Contains(v, "Gateway URL") || !strings.Contains(v, "Username") {
		t.Errorf("View() with login form should show Gateway URL and Username; got (len=%d)", len(v))
	}
}

// TestModel_SlashAuth_LoginOpensForm verifies /auth login opens the in-TUI login form (openLoginFormMsg).
func TestModel_SlashAuth_LoginOpensForm(t *testing.T) {
	session := &chat.Session{}
	m := NewModel(session)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/auth login")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /auth login should return cmd")
	}
	msg := cmd()
	_, ok := msg.(openLoginFormMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want openLoginFormMsg (login form)", msg)
	}
	// Apply the message so the form opens
	updated, _ := m.Update(msg)
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if !mod.ShowLoginForm {
		t.Error("ShowLoginForm should be true after openLoginFormMsg")
	}
}

// TestModel_SlashAuth_Whoami_NoClient verifies /auth whoami with nil client shows error.
func TestModel_SlashAuth_Whoami_NoClient(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/auth whoami")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /auth whoami should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	if len(res.lines) == 0 || !strings.Contains(res.lines[0], "not connected") {
		t.Errorf("expected 'not connected'; got %v", res.lines)
	}
}

// TestModel_SlashAuth_Whoami_Success verifies /auth whoami calls gateway and shows user.
//
//nolint:dupl // server handler pattern similar to TestModel_SlashAuth_Refresh_Success
func TestModel_SlashAuth_Whoami_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1UsersMe || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"u1","handle":"alice","is_active":true}`))
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client}
	m := NewModel(session)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/auth whoami")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /auth whoami should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	if len(res.lines) == 0 || !strings.Contains(res.lines[0], "user=alice") || !strings.Contains(res.lines[0], "id=u1") {
		t.Errorf("/auth whoami should show id and user; got %v", res.lines)
	}
}

// TestModel_SlashAuth_NoProvider verifies /auth logout and /auth refresh without AuthProvider show not available.
func TestModel_SlashAuth_NoProvider(t *testing.T) {
	for _, tc := range []struct {
		name, input string
	}{
		{"logout", "/auth logout"},
		{"refresh", "/auth refresh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(&chat.Session{})
			_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.input)})
			_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("Enter should return cmd")
			}
			msg := cmd()
			res, ok := msg.(slashResultMsg)
			if !ok {
				t.Fatalf("cmd() = %T, want slashResultMsg", msg)
			}
			if len(res.lines) == 0 || !strings.Contains(res.lines[0], "not available") {
				t.Errorf("expected 'not available'; got %v", res.lines)
			}
		})
	}
}

// TestModel_SlashAuth_Logout_Success verifies /auth logout clears tokens and shows logged_out.
func TestModel_SlashAuth_Logout_Success(t *testing.T) {
	provider := &mockAuthProvider{token: "t", refreshToken: "r"}
	session := &chat.Session{Client: gateway.NewClient("http://localhost")}
	session.SetToken("t")
	m := NewModel(session)
	m.SetAuthProvider(provider)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/auth logout")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /auth logout should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	if len(res.lines) == 0 || res.lines[0] != "logged_out=true" {
		t.Errorf("/auth logout should show logged_out=true; got %v", res.lines)
	}
	if !provider.saved {
		t.Error("AuthProvider.Save should have been called")
	}
	if provider.token != "" || provider.refreshToken != "" {
		t.Errorf("tokens should be cleared; got token=%q refresh=%q", provider.token, provider.refreshToken)
	}
}

// TestModel_SlashAuth_Refresh_Success verifies /auth refresh updates tokens and shows success.
//
//nolint:dupl // server handler pattern similar to TestModel_SlashAuth_Whoami_Success
func TestModel_SlashAuth_Refresh_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/refresh" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":900}`))
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("old-tok")
	provider := &mockAuthProvider{token: "old-tok", refreshToken: "old-refresh"}
	session := &chat.Session{Client: client}
	session.SetToken("old-tok")
	m := NewModel(session)
	m.SetAuthProvider(provider)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/auth refresh")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /auth refresh should return cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	if len(res.lines) == 0 || !strings.Contains(res.lines[0], "Token refreshed") {
		t.Errorf("/auth refresh should show success; got %v", res.lines)
	}
	if provider.saved {
		t.Error("AuthProvider.Save should not be called on refresh (tokens are not written to config)")
	}
	if provider.token != "new-access" || provider.refreshToken != "new-refresh" {
		t.Errorf("provider after refresh: token=%q refresh=%q", provider.token, provider.refreshToken)
	}
}

// TestParseSlashTUI verifies parseSlashTUI parses command and rest correctly.
func TestParseSlashTUI(t *testing.T) {
	cases := []struct{ line, wantCmd, wantRest string }{
		{"/help", "help", ""},
		{"/model test-model", "model", "test-model"},
		{"/project set abc", "project", "set abc"},
		{"/exit", "exit", ""},
		{"/QUIT", "quit", ""},
		{"/auth", "auth", ""},
		{"/auth whoami", "auth", "whoami"},
	}
	for _, tc := range cases {
		cmd, rest := parseSlashTUI(tc.line)
		if cmd != tc.wantCmd || rest != tc.wantRest {
			t.Errorf("parseSlashTUI(%q) = (%q, %q), want (%q, %q)", tc.line, cmd, rest, tc.wantCmd, tc.wantRest)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

// TestModel_SetResumeThreadSelector verifies the setter writes to the field.
func TestModel_SetResumeThreadSelector(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.SetResumeThreadSelector("selector-abc")
	if m.ResumeThreadSelector != "selector-abc" {
		t.Errorf("ResumeThreadSelector = %q, want selector-abc", m.ResumeThreadSelector)
	}
}

// TestModel_Init_OpenLoginFormOnInit verifies Init returns a cmd that sends openLoginFormMsg.
func TestModel_Init_OpenLoginFormOnInit(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.OpenLoginFormOnInit = true
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() with OpenLoginFormOnInit=true should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(openLoginFormMsg); !ok {
		t.Errorf("cmd() = %T, want openLoginFormMsg", msg)
	}
}

// TestModel_Init_EnsureThreadWhenLoggedIn verifies Init schedules EnsureThread when a token is present.
func TestModel_Init_EnsureThreadWhenLoggedIn(t *testing.T) {
	server := newMockThreadServer(t, "init-thread-id")
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client}
	m := NewModel(session)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() with token should return ensureThreadCmd")
	}
	msg := cmd()
	if session.CurrentThreadID != "" {
		t.Fatal("EnsureThread cmd must not set CurrentThreadID before Update")
	}
	res, ok := msg.(ensureThreadResult)
	if !ok || res.err != nil || res.threadID != "init-thread-id" {
		t.Errorf("expected ensureThreadResult{threadID:init-thread-id}; got %T %+v", msg, msg)
	}
}

// TestModel_Update_EnsureThreadResult_Error verifies applyEnsureThreadResult shows error in scrollback.
func TestModel_Update_EnsureThreadResult_Error(t *testing.T) {
	m := NewModel(&chat.Session{})
	updated, cmd := m.Update(ensureThreadResult{err: errors.New("thread error")})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}
	mod := updated.(*Model)
	if len(mod.Scrollback) == 0 || !strings.Contains(mod.Scrollback[0], "thread error") {
		t.Errorf("Scrollback = %v; expected error message", mod.Scrollback)
	}
}

// TestModel_Update_EnsureThreadResult_Success verifies applyEnsureThreadResult adds thread ID line.
func TestModel_Update_EnsureThreadResult_Success(t *testing.T) {
	session := &chat.Session{}
	m := NewModel(session)
	updated, cmd := m.Update(ensureThreadResult{threadID: "tid-ok"})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}
	mod := updated.(*Model)
	if mod.Session.CurrentThreadID != "tid-ok" {
		t.Errorf("CurrentThreadID = %q, want tid-ok", mod.Session.CurrentThreadID)
	}
	found := false
	for _, line := range mod.Scrollback {
		if strings.Contains(line, "tid-ok") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Scrollback = %v; expected thread ID tid-ok", mod.Scrollback)
	}
}

// TestModel_Update_EnsureThreadResult_StartsProactiveRefresh verifies tea.Every is scheduled once when a refresh token exists.
func TestModel_Update_EnsureThreadResult_StartsProactiveRefresh(t *testing.T) {
	session := &chat.Session{Client: gateway.NewClient("http://localhost")}
	m := NewModel(session)
	m.SetAuthProvider(&mockAuthProvider{refreshToken: "rt"})
	updated, cmd := m.Update(ensureThreadResult{threadID: "tid-ok"})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for proactive token refresh")
	}
	mod := updated.(*Model)
	if !mod.proactiveTokenRefreshStarted {
		t.Error("proactiveTokenRefreshStarted should be true")
	}
	// Second ensureThreadResult must not stack another Every.
	updated2, cmd2 := mod.Update(ensureThreadResult{})
	mod2 := updated2.(*Model)
	if cmd2 != nil {
		t.Errorf("second Update(ensureThreadResult) cmd = %v, want nil", cmd2)
	}
	if !mod2.proactiveTokenRefreshStarted {
		t.Error("proactiveTokenRefreshStarted should stay true")
	}
}

// TestModel_Update_EnsureThreadResult_NoThreadID verifies empty threadID is a no-op.
func TestModel_Update_EnsureThreadResult_NoThreadID(t *testing.T) {
	m := NewModel(&chat.Session{})
	updated, _ := m.Update(ensureThreadResult{})
	mod := updated.(*Model)
	if len(mod.Scrollback) != 0 {
		t.Errorf("Scrollback should be empty for empty ensureThreadResult; got %v", mod.Scrollback)
	}
}

// TestModel_Update_OpenLoginForm verifies applyOpenLoginForm sets ShowLoginForm and clears fields.
func TestModel_Update_OpenLoginForm(t *testing.T) {
	session := &chat.Session{Client: gateway.NewClient(loginTestGatewayURL)}
	m := NewModel(session)
	m.Loading = true
	updated, cmd := m.Update(openLoginFormMsg{})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}
	mod := updated.(*Model)
	if !mod.ShowLoginForm {
		t.Error("ShowLoginForm should be true")
	}
	if mod.Loading {
		t.Error("Loading should be false")
	}
	if mod.LoginGatewayURL != loginTestGatewayURL {
		t.Errorf("LoginGatewayURL = %q, want http://gw", mod.LoginGatewayURL)
	}
}
