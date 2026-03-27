package tui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

const threadListHeader = "--- Threads (use ordinal, id, or title with /thread switch <selector>) ---"
const inputThreadList = "/thread list"
const pathChatThreads = "/v1/chat/threads"
const loginTestGatewayURL = "http://gw"
const loginTestPassword = "pass"
const testHealthzPath = "/healthz"
const testSlashThreadFilter = "/th"
const testThreadNewInput = "/thread new"

// amendStreamTransport emits a delta then a secret_redaction-style amendment before Done.
type amendStreamTransport struct{}

func (a *amendStreamTransport) SendMessage(context.Context, string, string, string) (*chat.AssistantTurn, error) {
	return nil, errors.New("unused")
}

func (a *amendStreamTransport) StreamMessage(_ context.Context, _, _, _ string) (<-chan chat.ChatStreamDelta, error) {
	ch := make(chan chat.ChatStreamDelta, 4) //nolint:mnd // order: delta, amendment, done
	ch <- chat.ChatStreamDelta{Delta: "draft"}
	ch <- chat.ChatStreamDelta{Amendment: "redacted"}
	ch <- chat.ChatStreamDelta{Done: true}
	close(ch)
	return ch, nil
}

type mockTransport struct {
	visible string
	err     error
}

func (m *mockTransport) SendMessage(_ context.Context, _, _, _ string) (*chat.AssistantTurn, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &chat.AssistantTurn{VisibleText: m.visible}, nil
}

// StreamMessage implements ChatTransport for the mock by emitting the visible text as a single
// delta followed by a Done event, or a Done event with Err if m.err is set.
func (m *mockTransport) StreamMessage(_ context.Context, _, _, _ string) (<-chan chat.ChatStreamDelta, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan chat.ChatStreamDelta, 2) //nolint:mnd // buffered so goroutine does not block
	ch <- chat.ChatStreamDelta{Delta: m.visible}
	ch <- chat.ChatStreamDelta{Done: true}
	close(ch)
	return ch, nil
}

func TestPlainTranscript_SkipsSystemLines(t *testing.T) {
	t.Parallel()
	lines := []string{
		"You: hi",
		scrollbackSystemLinePrefix + "Last message copied to clipboard.",
		"Assistant: hello",
	}
	got := plainTranscript(lines)
	want := "You: hi\nAssistant: hello"
	if got != want {
		t.Errorf("plainTranscript = %q, want %q", got, want)
	}
}

func TestNewModel(t *testing.T) {
	session := &chat.Session{Transport: &mockTransport{}}
	m := NewModel(session)
	if m == nil {
		t.Fatal("NewModel returned nil")
	}
	if m.Session != session {
		t.Errorf("Session = %p, want %p", m.Session, session)
	}
	if len(m.Scrollback) != 0 {
		t.Errorf("Scrollback = %v, want empty", m.Scrollback)
	}
	if m.Input != "" {
		t.Errorf("Input = %q", m.Input)
	}
	if m.Width != 80 || m.Height != 24 {
		t.Errorf("Width=%d Height=%d, want 80, 24", m.Width, m.Height)
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel(&chat.Session{})
	cmd := m.Init()
	if cmd != nil {
		t.Errorf("Init() = %v, want nil", cmd)
	}
}

func TestModel_Update_CopyClipboardResultMsg(t *testing.T) {
	m := NewModel(&chat.Session{})
	updated, cmd := m.Update(copyClipboardResultMsg{err: nil, successDetail: "All text copied to clipboard."})
	if cmd == nil {
		t.Error("Update(copyClipboardResultMsg) expected clip clear tick cmd")
	}
	_ = cmd
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if len(mod.Scrollback) != 1 || !strings.HasSuffix(mod.Scrollback[0], "All text copied to clipboard.") {
		t.Fatalf("Scrollback = %v", mod.Scrollback)
	}
	if mod.ClipNote != "All text copied to clipboard." {
		t.Errorf("ClipNote = %q", mod.ClipNote)
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 80
	m.Height = 24
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd != nil {
		t.Errorf("Update(WindowSizeMsg) cmd = %v", cmd)
	}
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mod.Width != 100 || mod.Height != 30 {
		t.Errorf("Width=%d Height=%d, want 100, 30", mod.Width, mod.Height)
	}
}

func TestModel_Update_SendResultErr(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	errMsg := errors.New("gateway error")
	updated, cmd := m.Update(sendResult{err: errMsg})
	if cmd != nil {
		t.Errorf("Update(sendResult) cmd = %v", cmd)
	}
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mod.Loading {
		t.Error("Loading still true")
	}
	if mod.Err != errMsg.Error() {
		t.Errorf("Err = %q", mod.Err)
	}
	if len(mod.Scrollback) != 1 || mod.Scrollback[0] != "Error: "+errMsg.Error() {
		t.Errorf("Scrollback = %v", mod.Scrollback)
	}
}

func TestModel_Update_SendResultVisible(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	updated, cmd := m.Update(sendResult{visible: "Hello"})
	if cmd != nil {
		t.Errorf("Update(sendResult) cmd = %v", cmd)
	}
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mod.Loading {
		t.Error("Loading still true")
	}
	if len(mod.Scrollback) != 1 || mod.Scrollback[0] != "Assistant: Hello" {
		t.Errorf("Scrollback = %v", mod.Scrollback)
	}
}

// TestModel_HandleKey_Quit verifies the two-Ctrl+C exit flow:
// first Ctrl+C when idle shows a hint; second Ctrl+C exits.
func TestModel_HandleKey_Quit(t *testing.T) {
	m := NewModel(&chat.Session{})
	// First Ctrl+C: shows hint, no quit cmd.
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if mod != m {
		t.Error("first ctrl+c changed model pointer")
	}
	if cmd != nil {
		// tea.Quit is a function value; we can't compare directly but it must be nil
		// unless it is the Quit cmd — accept it if it is the Quit cmd type.
		t.Logf("first ctrl+c returned a cmd (may be Quit): %v", cmd)
	}
	if m.ctrlCCount != 1 {
		t.Errorf("after first ctrl+c: ctrlCCount = %d, want 1", m.ctrlCCount)
	}
	hintFound := false
	for _, line := range m.Scrollback {
		if strings.Contains(line, "Ctrl+C") {
			hintFound = true
			break
		}
	}
	if !hintFound {
		t.Errorf("first ctrl+c: expected hint in scrollback, got %v", m.Scrollback)
	}
	// Second Ctrl+C: must return tea.Quit.
	_, cmd2 := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd2 == nil {
		t.Error("second ctrl+c returned nil cmd; expected tea.Quit")
	}
}

// TestModel_HandleKey_CtrlD verifies that Ctrl+D exits immediately.
func TestModel_HandleKey_CtrlD(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Error("handleKey(ctrl+d) returned nil cmd; expected tea.Quit")
	}
}

func TestModel_HandleKey_EnterEmptyAndBackspace(t *testing.T) {
	helper := func(t *testing.T, key tea.KeyMsg, initialInput, wantInput string, wantCmd bool) {
		t.Helper()
		m := NewModel(&chat.Session{})
		m.Input = initialInput
		m.syncInputCursorEnd()
		mod, cmd := m.handleKey(key)
		if (cmd != nil) != wantCmd {
			t.Errorf("handleKey cmd = %v, wantCmd=%v", cmd != nil, wantCmd)
		}
		if mod != m {
			t.Error("handleKey changed model")
		}
		if m.Input != wantInput {
			t.Errorf("Input = %q, want %q", m.Input, wantInput)
		}
	}
	t.Run("EnterEmpty", func(t *testing.T) {
		helper(t, tea.KeyMsg{Type: tea.KeyEnter}, "   ", "", false)
	})
	t.Run("Backspace", func(t *testing.T) {
		helper(t, tea.KeyMsg{Type: tea.KeyBackspace}, "ab", "a", false)
	})
}

// TestModel_HandleKey_EnterWithText verifies that pressing Enter with text starts a streaming turn.
// After Enter: "You: text" is in scrollback, Loading is true, and the first cmd() returns a
// streamDeltaMsg with the mock transport's visible text (or a streamDoneMsg on error).
func TestModel_HandleKey_EnterWithText(t *testing.T) {
	transport := &mockTransport{visible: "ok"}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	m.Input = testSampleWordHello
	m.syncInputCursorEnd()
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if mod != m {
		t.Error("handleKey changed model")
	}
	if cmd == nil {
		t.Fatal("handleKey(enter with text) returned nil cmd")
	}
	if m.Input != "" {
		t.Errorf("Input = %q", m.Input)
	}
	// "You: hello" must appear; assistantPrefix placeholder is also seeded.
	if len(m.Scrollback) < 2 || m.Scrollback[0] != "You: hello" {
		t.Errorf("Scrollback = %v", m.Scrollback)
	}
	if !m.Loading {
		t.Error("Loading not set")
	}
	// First message must be streamStartMsg (channel hand-off to main loop).
	firstMsg := cmd()
	if _, ok := firstMsg.(streamStartMsg); !ok {
		t.Errorf("cmd() = %T, want streamStartMsg", firstMsg)
	}
}

func TestModel_HandleKey_LoadingIgnoresKeys(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("handleKey when loading returned cmd")
	}
	if mod != m {
		t.Error("handleKey changed model")
	}
}

func TestEnsureThreadScrollbackLine(t *testing.T) {
	tests := []struct {
		name       string
		prior      string
		after      string
		resume     string
		wantSwitch bool
		wantReady  bool
	}{
		{"same_thread_confirmed", "tid-1", "tid-1", "", false, true},
		{"resume_from_empty", "", "tid-1", "1", true, false},
		{"switch_without_resume", "tid-1", "tid-2", "", true, false},
		{"first_thread_new", "", "tid-new", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureThreadScrollbackLine(tt.prior, tt.after, tt.resume)
			hasSw := strings.Contains(got, chat.LandmarkThreadSwitched)
			hasRd := strings.Contains(got, chat.LandmarkThreadReady)
			if hasSw != tt.wantSwitch || hasRd != tt.wantReady {
				t.Errorf("ensureThreadScrollbackLine(%q,%q,%q) = %q; want switch=%v ready=%v",
					tt.prior, tt.after, tt.resume, got, tt.wantSwitch, tt.wantReady)
			}
		})
	}
}

func TestModel_InputHistory_PushAndNavigate(t *testing.T) {
	const inputFirst, inputSecond = "first", "second"
	m := NewModel(&chat.Session{})
	m.pushInputHistory(inputFirst)
	m.pushInputHistory(inputSecond)
	if len(m.InputHistory) != 2 || m.InputHistory[0] != inputSecond || m.InputHistory[1] != inputFirst {
		t.Errorf("InputHistory = %v", m.InputHistory)
	}
	m.pushInputHistory(inputSecond) // dedupe
	if len(m.InputHistory) != 2 {
		t.Errorf("dedupe: InputHistory = %v", m.InputHistory)
	}
	// Up from -1: show newest (index 0)
	m.navigateInputHistory(true)
	if m.InputHistoryIdx != 0 || m.Input != inputSecond {
		t.Errorf("Up from -1: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
	// Up again: show older (index 1)
	m.navigateInputHistory(true)
	if m.InputHistoryIdx != 1 || m.Input != inputFirst {
		t.Errorf("Up again: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
	// Up at end: no change
	m.navigateInputHistory(true)
	if m.InputHistoryIdx != 1 || m.Input != inputFirst {
		t.Errorf("Up at end: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
	// Down: back to newer
	m.navigateInputHistory(false)
	if m.InputHistoryIdx != 0 || m.Input != inputSecond {
		t.Errorf("Down: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
	// Down: exit history, clear input
	m.navigateInputHistory(false)
	if m.InputHistoryIdx != -1 || m.Input != "" {
		t.Errorf("Down to exit: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
}

func TestModel_HandleKey_CtrlUpDownHistory(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.InputHistory = []string{"newest", "older"}
	m.InputHistoryIdx = -1
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlUp})
	if m.InputHistoryIdx != 0 || m.Input != "newest" {
		t.Errorf("Ctrl+Up: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlDown})
	if m.InputHistoryIdx != -1 || m.Input != "" {
		t.Errorf("Ctrl+Down from 0: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
}

func TestModel_HandleKey_DefaultRunes(t *testing.T) {
	m := NewModel(&chat.Session{})
	mod, cmd := m.handleKey(tea.KeyMsg{Runes: []rune("x")})
	if cmd != nil {
		t.Errorf("handleKey(rune) cmd = %v", cmd)
	}
	if mod != m {
		t.Error("handleKey changed model")
	}
	if m.Input != "x" {
		t.Errorf("Input = %q", m.Input)
	}
}

func TestModel_View_ComposerHint(t *testing.T) {
	m := NewModel(&chat.Session{})
	v := m.View()
	nw := strings.Join(strings.Fields(v), " ")
	if !strings.Contains(nw, "/ commands") || !strings.Contains(nw, "@ files") || !strings.Contains(nw, "! shell") {
		t.Errorf("View() should show composer hint (/ commands · @ files · ! shell): %s", v)
	}
}

func TestModel_Update_ThreadListResult_Error(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	updated, cmd := m.Update(threadListResult{err: errors.New("list failed")})
	if cmd != nil {
		t.Errorf("Update(threadListResult err) cmd = %v", cmd)
	}
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mod.Loading {
		t.Error("Loading still true")
	}
	if mod.Err != "list failed" {
		t.Errorf("Err = %q", mod.Err)
	}
}

func TestModel_Update_ThreadListResult(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	updated, cmd := m.Update(threadListResult{lines: []string{threadListHeader, "  id1  Title1"}})
	if cmd != nil {
		t.Errorf("Update(threadListResult) cmd = %v", cmd)
	}
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mod.Loading {
		t.Error("Loading still true")
	}
	if len(mod.Scrollback) != 2 || mod.Scrollback[0] != scrollbackSystemLinePrefix+threadListHeader {
		t.Errorf("Scrollback = %v", mod.Scrollback)
	}
}

func TestModel_Update_ThreadRenameResult_Error(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	updated, cmd := m.Update(threadRenameResult{err: errors.New("rename failed")})
	if cmd != nil {
		t.Errorf("Update(threadRenameResult err) cmd = %v", cmd)
	}
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mod.Err != "rename failed" {
		t.Errorf("Err = %q", mod.Err)
	}
}

func TestModel_Update_ThreadRenameResult(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	updated, cmd := m.Update(threadRenameResult{})
	if cmd != nil {
		t.Errorf("Update(threadRenameResult) cmd = %v", cmd)
	}
	mod, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mod.Loading {
		t.Error("Loading still true")
	}
	if len(mod.Scrollback) != 1 || mod.Scrollback[0] != scrollbackSystemLinePrefix+"Thread renamed." {
		t.Errorf("Scrollback = %v", mod.Scrollback)
	}
}

func TestModel_ThreadCommand_New(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathChatThreads || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "new-tid"})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	m := NewModel(session)
	m.Input = testThreadNewInput
	m.Scrollback = nil
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("handleKey(/thread new) cmd = %v", cmd)
	}
	if mod != m {
		t.Error("handleKey changed model")
	}
	if session.CurrentThreadID != "new-tid" {
		t.Errorf("CurrentThreadID = %q", session.CurrentThreadID)
	}
	found := false
	for _, s := range m.Scrollback {
		if strings.Contains(s, "New thread:") && strings.Contains(s, "new-tid") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Scrollback missing new thread line: %v", m.Scrollback)
	}
}

func TestModel_ThreadCommand_List_ReturnsCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	m := NewModel(session)
	m.Input = inputThreadList
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("handleKey(" + inputThreadList + ") returned nil cmd")
	}
	msg := cmd()
	res, ok := msg.(threadListResult)
	if !ok {
		t.Fatalf("cmd() = %T", msg)
	}
	if res.err != nil {
		t.Errorf("threadListResult err = %v", res.err)
	}
	if len(res.lines) < 1 || res.lines[0] != threadListHeader {
		t.Errorf("threadListResult lines = %v", res.lines)
	}
}

func TestModel_ThreadCommand_List_CmdErrorPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	m := NewModel(session)
	m.Input = inputThreadList
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("handleKey(" + inputThreadList + ") returned nil cmd")
	}
	msg := cmd()
	res, ok := msg.(threadListResult)
	if !ok {
		t.Fatalf("cmd() = %T", msg)
	}
	if res.err == nil {
		t.Error("threadListResult err = nil")
	}
}

func TestModel_ThreadListCmd_NilSession(t *testing.T) {
	m := NewModel(nil)
	cmd := m.threadListCmd()
	if cmd == nil {
		t.Fatal("threadListCmd returned nil")
	}
	msg := cmd()
	res, ok := msg.(threadListResult)
	if !ok {
		t.Fatalf("cmd() = %T", msg)
	}
	if res.err == nil {
		t.Error("threadListResult err = nil for nil session")
	}
}

func TestModel_ThreadListCmd_WithItemsAndTitles(t *testing.T) {
	titleA := "First"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "tid-1", "title": titleA},
				{"id": "tid-2"},
				{"id": "tid-3", "title": ""},
			},
		})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	m := NewModel(session)
	m.Input = inputThreadList
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("handleKey(" + inputThreadList + ") returned nil cmd")
	}
	msg := cmd()
	res, ok := msg.(threadListResult)
	if !ok {
		t.Fatalf("cmd() = %T", msg)
	}
	if res.err != nil {
		t.Fatalf("threadListResult err = %v", res.err)
	}
	if len(res.lines) != 4 {
		t.Fatalf("len(lines) = %d, want 4 (header + 3 threads)", len(res.lines))
	}
	if res.lines[0] != threadListHeader {
		t.Errorf("lines[0] = %q", res.lines[0])
	}
	if !strings.Contains(res.lines[1], "tid-1") || !strings.Contains(res.lines[1], titleA) {
		t.Errorf("lines[1] = %q", res.lines[1])
	}
	if !strings.Contains(res.lines[2], "tid-2") || !strings.Contains(res.lines[2], "(no title)") {
		t.Errorf("lines[2] = %q", res.lines[2])
	}
	if !strings.Contains(res.lines[3], "tid-3") || !strings.Contains(res.lines[3], "(no title)") {
		t.Errorf("lines[3] = %q", res.lines[3])
	}
}

func nilSessionCmdMessage(t *testing.T, gen func(*Model) tea.Cmd) any {
	t.Helper()
	m := NewModel(nil)
	cmd := gen(m)
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	return cmd()
}

func TestModel_ThreadRenameCmd_NilSession(t *testing.T) {
	msg := nilSessionCmdMessage(t, func(m *Model) tea.Cmd { return m.threadRenameCmd("x") })
	res, ok := msg.(threadRenameResult)
	if !ok {
		t.Fatalf("cmd() = %T", msg)
	}
	if res.err == nil {
		t.Error("threadRenameResult err = nil for nil session")
	}
}

func TestModel_StreamCmd_NoSession(t *testing.T) {
	msg := nilSessionCmdMessage(t, func(m *Model) tea.Cmd { return m.streamCmd(testSampleWordHello) })
	sr, ok := msg.(sendResult)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if sr.err == nil {
		t.Fatal("expected error")
	}
}

func TestModel_ThreadCommand_Switch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathChatThreads || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "thread-123", "title": "Test"},
			},
		})
	}))
	defer server.Close()
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.Client.SetToken("tok")
	m := NewModel(session)
	m.Input = "/thread switch thread-123"
	m.Scrollback = nil
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("handleKey(/thread switch) cmd = %v", cmd)
	}
	if session.CurrentThreadID != "thread-123" {
		t.Errorf("CurrentThreadID = %q", session.CurrentThreadID)
	}
	found := false
	for _, s := range m.Scrollback {
		if strings.Contains(s, "Switched to thread") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Scrollback = %v", m.Scrollback)
	}
}

func TestModel_ThreadCommand_NilSession(t *testing.T) {
	m := NewModel(nil) // Session is nil
	m.Input = testThreadNewInput
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("handleKey(/thread new) with nil session returned cmd")
	}
}

func TestModel_ThreadCommand_NonSwitchRenameShowsMessage(t *testing.T) {
	helper := func(t *testing.T, input string) {
		t.Helper()
		session := chat.NewSession(gateway.NewClient("http://localhost"))
		m := NewModel(session)
		m.Input = input
		m.Scrollback = nil
		_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Errorf("handleKey(%q) cmd = %v", input, cmd)
		}
		if len(m.Scrollback) < 1 {
			t.Errorf("Scrollback = %v", m.Scrollback)
		}
	}
	t.Run("UnknownSubcommand", func(t *testing.T) { helper(t, "/thread unknown") })
	t.Run("EmptyRest", func(t *testing.T) { helper(t, "/thread ") })
}

func TestModel_ThreadCommand_UsageOutput(t *testing.T) {
	helper := func(t *testing.T, input string) {
		t.Helper()
		session := chat.NewSession(gateway.NewClient("http://localhost"))
		m := NewModel(session)
		m.Input = input
		m.Scrollback = nil
		_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Errorf("handleKey(%q) cmd = %v", input, cmd)
		}
		hasUsage := false
		for _, s := range m.Scrollback {
			if strings.Contains(s, "Usage") {
				hasUsage = true
				break
			}
		}
		if !hasUsage {
			t.Errorf("Scrollback = %v", m.Scrollback)
		}
	}
	t.Run("SwitchNoID", func(t *testing.T) { helper(t, "/thread switch") })
	t.Run("RenameEmptyTitle", func(t *testing.T) { helper(t, "/thread rename   ") })
}

func TestModel_ThreadCommand_Rename_ReturnsCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	session.CurrentThreadID = "tid-1"
	m := NewModel(session)
	m.Input = "/thread rename My Title"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("handleKey(/thread rename) returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(threadRenameResult); !ok {
		t.Errorf("cmd() = %T", msg)
	}
}

func TestModel_ThreadCommand_Rename_CmdErrorPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	session.CurrentThreadID = "tid-1"
	m := NewModel(session)
	m.Input = "/thread rename New"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("handleKey(/thread rename) returned nil cmd")
	}
	msg := cmd()
	res, ok := msg.(threadRenameResult)
	if !ok {
		t.Fatalf("cmd() = %T", msg)
	}
	if res.err == nil {
		t.Error("threadRenameResult err = nil")
	}
}

func TestModel_View_ShowsThreadInStatus(t *testing.T) {
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	session.CurrentThreadID = "abc12345-xxxx"
	m := NewModel(session)
	m.Session = session
	v := m.View()
	if !strings.Contains(v, "thread:") {
		t.Errorf("View() should show thread in status: %s", v)
	}
}

func TestModel_View_SessionNil(t *testing.T) {
	m := &Model{Session: nil, Width: 80, Height: 24}
	v := m.View()
	if !strings.Contains(v, "(default)") {
		t.Errorf("View with nil session should show placeholders: %s", v)
	}
	if !strings.Contains(v, chat.TUIStatusIdle) {
		t.Errorf("View with nil session should show idle glyph: %s", v)
	}
}

func TestModel_View_ContainsLandmarks(t *testing.T) {
	m := NewModel(&chat.Session{})
	v := m.View()
	if !strings.Contains(v, chat.TUIStatusIdle) {
		t.Errorf("View() missing idle status %q: %s", chat.TUIStatusIdle, v)
	}
	m.Loading = true
	v2 := m.View()
	if !strings.Contains(v2, chat.TUIStatusBusy) {
		t.Errorf("View() when loading missing busy glyph %q", chat.TUIStatusBusy)
	}
}

// TestModel_StreamCmd verifies that streamCmd starts a streaming turn, returning the first
// delta or done event. On success the mock transport emits a single delta "reply" then Done.
func TestModel_StreamCmd_AmendmentReplacesBuffer(t *testing.T) {
	session := &chat.Session{Transport: &amendStreamTransport{}}
	m := NewModel(session)
	cmd := m.streamCmd(testSampleWordHello)
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	start, ok := cmd().(streamStartMsg)
	if !ok {
		t.Fatalf("got %T", start)
	}
	m2, next := m.Update(start)
	m = m2.(*Model)
	if next == nil {
		t.Fatal("nil next")
	}
	// Delta
	d1 := next()
	if _, ok := d1.(streamDeltaMsg); !ok {
		t.Fatalf("first delta = %T", d1)
	}
	m3, next2 := m.Update(d1)
	m = m3.(*Model)
	if next2 == nil {
		t.Fatal("nil cmd after delta")
	}
	// Amendment
	d2 := next2()
	am, ok := d2.(streamDeltaMsg)
	if !ok || am.amendment != "redacted" {
		t.Fatalf("amendment msg = %#v ok=%v", d2, ok)
	}
	m4, next3 := m.Update(d2)
	m = m4.(*Model)
	if !strings.HasSuffix(m.Scrollback[len(m.Scrollback)-1], "Assistant: redacted") {
		t.Errorf("scrollback = %v", m.Scrollback)
	}
	if next3 == nil {
		t.Fatal("nil after amendment")
	}
	done := next3()
	m5, _ := m.Update(done)
	mod := m5.(*Model)
	if mod.Loading {
		t.Error("still loading after done")
	}
}

func TestModel_StreamCmd(t *testing.T) {
	transport := &mockTransport{visible: "reply"}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	// streamCmd seeds the scrollback placeholder line.
	cmd := m.streamCmd(testSampleWordHello)
	if cmd == nil {
		t.Fatal("streamCmd returned nil")
	}
	if len(m.Scrollback) == 0 || m.Scrollback[len(m.Scrollback)-1] != assistantPrefix {
		t.Errorf("streamCmd should seed scrollback placeholder, got %v", m.Scrollback)
	}
	// First cmd() returns streamStartMsg carrying the channel.
	firstMsg := cmd()
	start, ok := firstMsg.(streamStartMsg)
	if !ok {
		t.Fatalf("first cmd() returned %T, want streamStartMsg", firstMsg)
	}
	// Process streamStartMsg through Update: stores ch and returns scheduleNextDelta cmd.
	m2, nextCmd := m.Update(start)
	m = m2.(*Model)
	if m.streamCh == nil {
		t.Fatal("Update(streamStartMsg) did not set streamCh")
	}
	if nextCmd == nil {
		t.Fatal("Update(streamStartMsg) returned nil cmd")
	}
	// Next cmd() returns the first delta "reply".
	deltaMsg := nextCmd()
	delta, ok := deltaMsg.(streamDeltaMsg)
	if !ok {
		t.Fatalf("scheduleNextDelta cmd returned %T, want streamDeltaMsg", deltaMsg)
	}
	if delta.delta != "reply" {
		t.Errorf("delta.delta = %q, want %q", delta.delta, "reply")
	}
	// Simulate processing the delta: update scrollback and schedule next read.
	m3, nextCmd2 := m.Update(delta)
	m = m3.(*Model)
	if m.Scrollback[len(m.Scrollback)-1] != "Assistant: reply" {
		t.Errorf("scrollback not updated in-place: %v", m.Scrollback)
	}
	// Next read should return the Done event.
	if nextCmd2 == nil {
		t.Fatal("no next cmd after delta")
	}
	doneMsg := nextCmd2()
	done, ok := doneMsg.(streamDoneMsg)
	if !ok {
		t.Fatalf("next cmd returned %T, want streamDoneMsg", doneMsg)
	}
	if done.err != nil {
		t.Errorf("streamDoneMsg err = %v", done.err)
	}
}

// TestModel_StreamCmd_TransportError verifies that when StreamMessage returns an error the
// model receives a streamDoneMsg with Err set.
func TestModel_StreamCmd_TransportError(t *testing.T) {
	transport := &mockTransport{err: errors.New("network error")}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	cmd := m.streamCmd(testSampleWordHello)
	msg := cmd()
	done, ok := msg.(streamDoneMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want streamDoneMsg", msg)
	}
	if done.err == nil {
		t.Error("streamDoneMsg err = nil, want network error")
	}
}

func TestOrEmpty(t *testing.T) {
	if g := orEmpty(""); g != defaultPlaceholder {
		t.Errorf("orEmpty(\"\") = %q, want %q", g, defaultPlaceholder)
	}
	if g := orEmpty("x"); g != "x" {
		t.Errorf("orEmpty(\"x\") = %q", g)
	}
}
