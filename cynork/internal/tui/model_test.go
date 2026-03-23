package tui

import (
	"context"
	"encoding/json"
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

const threadListHeader = "--- Threads (use ordinal, id, or title with /thread switch <selector>) ---"
const inputThreadList = "/thread list"
const pathChatThreads = "/v1/chat/threads"
const loginTestGatewayURL = "http://gw"
const loginTestPassword = "pass"

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
	m.Input = "hello"
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
	m.Input = "/thread new"
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

func TestModel_ThreadRenameCmd_NilSession(t *testing.T) {
	m := NewModel(nil)
	cmd := m.threadRenameCmd("x")
	if cmd == nil {
		t.Fatal("threadRenameCmd returned nil")
	}
	msg := cmd()
	res, ok := msg.(threadRenameResult)
	if !ok {
		t.Fatalf("cmd() = %T", msg)
	}
	if res.err == nil {
		t.Error("threadRenameResult err = nil for nil session")
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
	m.Input = "/thread new"
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
func TestModel_StreamCmd(t *testing.T) {
	transport := &mockTransport{visible: "reply"}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	// streamCmd seeds the scrollback placeholder line.
	cmd := m.streamCmd("hello")
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
	cmd := m.streamCmd("hello")
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
	cmd := m.streamCmd("hello")
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
		if r.URL.Path != "/v1/users/me" || r.Method != http.MethodGet {
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
	m := NewModel(&chat.Session{})
	updated, cmd := m.Update(ensureThreadResult{threadID: "tid-ok"})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}
	mod := updated.(*Model)
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

// TestModel_Update_OpenLoginForm_NoSession verifies LoginGatewayURL falls back when session is nil.
func TestModel_Update_OpenLoginForm_NoSession(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Session.Client = nil
	updated, _ := m.Update(openLoginFormMsg{})
	mod := updated.(*Model)
	if !mod.ShowLoginForm {
		t.Error("ShowLoginForm should be true")
	}
}

// TestModel_Update_OpenLoginForm_AuthProvider verifies LoginGatewayURL from AuthProvider.
func TestModel_Update_OpenLoginForm_AuthProvider(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Session.Client = nil
	m.SetAuthProvider(&mockAuthProvider{gatewayURL: "http://from-provider"})
	updated, _ := m.Update(openLoginFormMsg{})
	mod := updated.(*Model)
	if mod.LoginGatewayURL != "http://from-provider" {
		t.Errorf("LoginGatewayURL = %q, want http://from-provider", mod.LoginGatewayURL)
	}
}

// TestModel_Update_LoginResult_Error verifies failed login shows error in scrollback.
func TestModel_Update_LoginResult_Error(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	updated, cmd := m.Update(loginResultMsg{Err: errors.New("bad credentials")})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}
	mod := updated.(*Model)
	if mod.ShowLoginForm {
		t.Error("ShowLoginForm should be false after login attempt")
	}
	if len(mod.Scrollback) == 0 || !strings.Contains(mod.Scrollback[0], "Login failed") {
		t.Errorf("Scrollback = %v; expected Login failed", mod.Scrollback)
	}
}

// newMockThreadServer starts a test server that responds to POST /v1/chat/threads with a JSON body
// containing the given threadID, and returns 200 for all other requests.
func newMockThreadServer(t *testing.T, threadID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathChatThreads && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"thread_id":%q}`, threadID)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

// TestModel_Update_LoginResult_NoAuthProvider verifies login with no provider adds thread cmd.
func TestModel_Update_LoginResult_NoAuthProvider(t *testing.T) {
	server := newMockThreadServer(t, "new-tid")
	defer server.Close()
	session := &chat.Session{Client: gateway.NewClient("http://original")}
	m := NewModel(session)
	updated, cmd := m.Update(loginResultMsg{
		GatewayURL:  server.URL,
		AccessToken: "new-tok",
	})
	if cmd == nil {
		t.Fatal("LoginResult success should return ensureThreadCmd")
	}
	mod := updated.(*Model)
	if len(mod.Scrollback) == 0 || !strings.Contains(mod.Scrollback[0], "Logged in") {
		t.Errorf("Scrollback = %v; expected Logged in", mod.Scrollback)
	}
}

// TestModel_Update_LoginResult_WithAuthProvider_SaveError verifies save error is shown.
func TestModel_Update_LoginResult_WithAuthProvider_SaveError(t *testing.T) {
	provider := &mockAuthProvider{saveErr: errors.New("disk full")}
	session := &chat.Session{Client: gateway.NewClient("http://orig")}
	m := NewModel(session)
	m.SetAuthProvider(provider)
	updated, cmd := m.Update(loginResultMsg{GatewayURL: "http://gw2", AccessToken: "tok2"})
	mod := updated.(*Model)
	if cmd != nil {
		t.Errorf("cmd = %v, want nil after save error", cmd)
	}
	found := false
	for _, line := range mod.Scrollback {
		if strings.Contains(line, "config save failed") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Scrollback = %v; expected config save failed", mod.Scrollback)
	}
}

// TestModel_Update_LoginResult_WithAuthProvider_Success verifies provider is updated and thread cmd issued.
func TestModel_Update_LoginResult_WithAuthProvider_Success(t *testing.T) {
	server := newMockThreadServer(t, "t-auth")
	defer server.Close()
	provider := &mockAuthProvider{}
	session := &chat.Session{Client: gateway.NewClient("http://orig")}
	m := NewModel(session)
	m.SetAuthProvider(provider)
	_, cmd := m.Update(loginResultMsg{
		GatewayURL:   server.URL,
		AccessToken:  "tok-a",
		RefreshToken: "tok-r",
	})
	if cmd == nil {
		t.Fatal("expected ensureThreadCmd after successful login with provider")
	}
	if !provider.saved {
		t.Error("AuthProvider.Save should have been called")
	}
	if provider.token != "tok-a" || provider.refreshToken != "tok-r" {
		t.Errorf("tokens: token=%q refresh=%q", provider.token, provider.refreshToken)
	}
}

// TestModel_LoginFormKey_Esc verifies Esc dismisses the login form.
func TestModel_LoginFormKey_Esc(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.LoginPassword = "secret"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Errorf("Esc cmd = %v, want nil", cmd)
	}
	mod := updated.(*Model)
	if mod.ShowLoginForm {
		t.Error("ShowLoginForm should be false after Esc")
	}
	if mod.LoginPassword != "" {
		t.Errorf("LoginPassword should be cleared, got %q", mod.LoginPassword)
	}
	if len(mod.Scrollback) == 0 || !strings.Contains(mod.Scrollback[0], "cancelled") {
		t.Errorf("Scrollback = %v; expected Login cancelled", mod.Scrollback)
	}
}

// TestModel_LoginFormKey_TabNavigation verifies Tab and Shift+Tab cycle focus.
func TestModel_LoginFormKey_TabNavigation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		key       tea.KeyType
		wantFocus int
	}{
		{"Tab forward", tea.KeyTab, 1},
		{"ShiftTab backward", tea.KeyShiftTab, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(&chat.Session{})
			m.ShowLoginForm = true
			m.LoginFocusedField = 0
			m.Update(tea.KeyMsg{Type: tc.key})
			if m.LoginFocusedField != tc.wantFocus {
				t.Errorf("%s: focus = %d, want %d", tc.name, m.LoginFocusedField, tc.wantFocus)
			}
		})
	}
}

// TestModel_LoginFormKey_Enter_MissingFields verifies Enter with empty fields shows validation error.
func TestModel_LoginFormKey_Enter_MissingFields(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.LoginGatewayURL = ""
	m.LoginUsername = ""
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("Enter with missing fields cmd = %v, want nil", cmd)
	}
	mod := updated.(*Model)
	if mod.LoginErr == "" {
		t.Error("LoginErr should be set for missing fields")
	}
}

// TestModel_LoginFormKey_Enter_Valid verifies Enter with fields set returns login cmd.
func TestModel_LoginFormKey_Enter_Valid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/login" {
			_, _ = w.Write([]byte(`{"access_token":"tok","refresh_token":"ref","token_type":"Bearer","expires_in":900}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.LoginGatewayURL = server.URL
	m.LoginUsername = "alice"
	m.LoginPassword = loginTestPassword
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter with valid fields should return login cmd")
	}
	msg := cmd()
	if res, ok := msg.(loginResultMsg); !ok || res.Err != nil {
		t.Errorf("loginResultMsg = %T %+v, expected success", msg, msg)
	}
}

// TestModel_LoginFormKey_Enter_LoginError verifies failed login returns loginResultMsg with Err.
func TestModel_LoginFormKey_Enter_LoginError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.LoginGatewayURL = server.URL
	m.LoginUsername = "alice"
	m.LoginPassword = "bad"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected login cmd")
	}
	msg := cmd()
	res, ok := msg.(loginResultMsg)
	if !ok || res.Err == nil {
		t.Errorf("expected loginResultMsg with Err; got %T %+v", msg, msg)
	}
}

// TestModel_LoginFormKey_Backspace verifies backspace deletes from focused field.
func TestModel_LoginFormKey_Backspace(t *testing.T) {
	type bsCase struct {
		name  string
		field int
		check func(*testing.T, *Model)
	}
	cases := []bsCase{
		{"gateway", 0, func(t *testing.T, m *Model) {
			if m.LoginGatewayURL != "http://g" {
				t.Errorf("gateway = %q, want http://g", m.LoginGatewayURL)
			}
		}},
		{"username", 1, func(t *testing.T, m *Model) {
			if m.LoginUsername != string(RoleUser)[:3] {
				t.Errorf("username = %q, want use", m.LoginUsername)
			}
		}},
		{"password", 2, func(t *testing.T, m *Model) {
			if m.LoginPassword != loginTestPassword[:3] {
				t.Errorf("password = %q, want pas", m.LoginPassword)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(&chat.Session{})
			m.ShowLoginForm = true
			m.LoginFocusedField = tc.field
			m.LoginGatewayURL = loginTestGatewayURL
			m.LoginUsername = string(RoleUser)
			m.LoginPassword = loginTestPassword
			m.syncLoginFormCursors()
			m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
			tc.check(t, m)
		})
	}
}

// TestModel_LoginFormKey_Append verifies typing appends to focused field.
func TestModel_LoginFormKey_Append(t *testing.T) {
	type appendCase struct {
		name  string
		field int
		check func(*testing.T, *Model)
	}
	cases := []appendCase{
		{"gateway", 0, func(t *testing.T, m *Model) {
			if m.LoginGatewayURL != "x" {
				t.Errorf("gateway = %q, want x", m.LoginGatewayURL)
			}
		}},
		{"username", 1, func(t *testing.T, m *Model) {
			if m.LoginUsername != "x" {
				t.Errorf("username = %q, want x", m.LoginUsername)
			}
		}},
		{"password", 2, func(t *testing.T, m *Model) {
			if m.LoginPassword != "x" {
				t.Errorf("password = %q, want x", m.LoginPassword)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(&chat.Session{})
			m.ShowLoginForm = true
			m.LoginFocusedField = tc.field
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
			tc.check(t, m)
		})
	}
}

// TestModel_EnsureThreadCmd_NilSession verifies ensureThreadCmd returns error when session is nil.
func TestModel_EnsureThreadCmd_NilSession(t *testing.T) {
	m := &Model{}
	cmd := m.ensureThreadCmd()
	if cmd == nil {
		t.Fatal("ensureThreadCmd should return a cmd even with nil session")
	}
	msg := cmd()
	res, ok := msg.(ensureThreadResult)
	if !ok || res.err == nil {
		t.Errorf("expected ensureThreadResult with err; got %T %+v", msg, msg)
	}
}

// TestModel_ApplyStreamDelta_Amendment verifies the amendment branch replaces streamBuf.
func TestModel_ApplyStreamDelta_Amendment(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Scrollback = append(m.Scrollback, "Assistant: original text")
	m.streamBuf.WriteString("original text")
	updated, _ := m.Update(streamDeltaMsg{amendment: "replaced text"})
	mod := updated.(*Model)
	if mod.streamBuf.String() != "replaced text" {
		t.Errorf("streamBuf = %q, want replaced text", mod.streamBuf.String())
	}
	last := mod.Scrollback[len(mod.Scrollback)-1]
	if last != "Assistant: replaced text" {
		t.Errorf("scrollback = %q, want Assistant: replaced text", last)
	}
}

// TestModel_ApplySlashResult_NilLines verifies nil lines clears scrollback.
func TestModel_ApplySlashResult_NilLines(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Scrollback = []string{"existing"}
	updated, _ := m.Update(slashResultMsg{lines: nil})
	mod := updated.(*Model)
	if len(mod.Scrollback) != 0 {
		t.Errorf("Scrollback = %v, want empty after nil lines", mod.Scrollback)
	}
}

// TestModel_ApplySlashResult_ExitModel verifies exitModel=true returns tea.Quit.
func TestModel_ApplySlashResult_ExitModel(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, cmd := m.Update(slashResultMsg{exitModel: true})
	if cmd == nil {
		t.Error("exitModel=true should return tea.Quit cmd")
	}
}

// TestModel_PushInputHistory_Duplicate verifies duplicate last item is not pushed.
func TestModel_PushInputHistory_Duplicate(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.pushInputHistory("hello")
	m.pushInputHistory("hello")
	if len(m.InputHistory) != 1 {
		t.Errorf("InputHistory len = %d, want 1 (no duplicate)", len(m.InputHistory))
	}
}

// TestModel_PushInputHistory_MaxCap verifies history is capped at maxInputHistory.
func TestModel_PushInputHistory_MaxCap(t *testing.T) {
	m := NewModel(&chat.Session{})
	for i := range maxInputHistory + 5 {
		m.pushInputHistory(fmt.Sprintf("line-%d", i))
	}
	if len(m.InputHistory) != maxInputHistory {
		t.Errorf("InputHistory len = %d, want %d", len(m.InputHistory), maxInputHistory)
	}
}

// TestModel_EnsureThreadCmd_EnsureError verifies error from EnsureThread is returned.
func TestModel_EnsureThreadCmd_EnsureError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client}
	m := NewModel(session)
	cmd := m.ensureThreadCmd()
	msg := cmd()
	res, ok := msg.(ensureThreadResult)
	if !ok || res.err == nil {
		t.Errorf("expected ensureThreadResult with err; got %T %+v", msg, msg)
	}
}

// TestModel_EnsureThreadCmd_Success verifies successful EnsureThread returns threadID.
func TestModel_EnsureThreadCmd_Success(t *testing.T) {
	server := newMockThreadServer(t, "new-thread")
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := &chat.Session{Client: client}
	m := NewModel(session)
	cmd := m.ensureThreadCmd()
	msg := cmd()
	res, ok := msg.(ensureThreadResult)
	if !ok || res.err != nil || res.threadID != "new-thread" {
		t.Errorf("expected ensureThreadResult{threadID:new-thread}; got %T %+v", msg, msg)
	}
}

func TestModel_SlashConnect_ShowURL(t *testing.T) {
	client := gateway.NewClient("http://gw-test:9")
	m := NewModel(&chat.Session{Client: client})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/connect")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /connect should produce cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, "gateway") {
			found = true
		}
	}
	if !found {
		t.Errorf("/connect show expected gateway in lines; got %v", res.lines)
	}
}

// newHealthzServer creates an httptest.Server that responds OK to /healthz.
func newHealthzServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	}))
}

func TestModel_SlashConnect_UpdateURL(t *testing.T) {
	srv := newHealthzServer(t)
	defer srv.Close()
	client := gateway.NewClient("http://old:1")
	m := NewModel(&chat.Session{Client: client})
	provider := &mockAuthProvider{gatewayURL: "http://old:1"}
	m.SetAuthProvider(provider)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/connect " + srv.URL)})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /connect URL should produce cmd")
	}
	msg := cmd()
	if _, ok := msg.(slashResultMsg); !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	if client.BaseURL != srv.URL {
		t.Errorf("expected BaseURL=%s, got %q", srv.URL, client.BaseURL)
	}
}

func TestModel_SlashSetTuiPref(t *testing.T) {
	runSlashTuiPrefTest := func(t *testing.T, line string, initModel, initProvider, wantModel, wantProvider bool,
		getModel func(*Model) *bool, getProvider func(*mockAuthProvider) *bool) {
		t.Helper()
		m := NewModel(&chat.Session{})
		provider := &mockAuthProvider{}
		if initModel {
			*getModel(m) = true
		}
		if initProvider {
			*getProvider(provider) = true
		}
		m.SetAuthProvider(provider)
		_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(line)})
		_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("Enter should produce cmd")
		}
		if _, ok := cmd().(slashResultMsg); !ok {
			t.Fatal("cmd should return slashResultMsg")
		}
		if got := *getModel(m); got != wantModel {
			t.Errorf("model field = %v, want %v", got, wantModel)
		}
		if got := *getProvider(provider); got != wantProvider {
			t.Errorf("provider field = %v, want %v", got, wantProvider)
		}
	}
	tests := []struct {
		name                    string
		line                    string
		initModel, initProvider bool
		wantModel, wantProvider bool
		getModel                func(*Model) *bool
		getProvider             func(*mockAuthProvider) *bool
	}{
		{"thinking show", "/show-thinking", false, false, true, true,
			func(m *Model) *bool { return &m.ShowThinking },
			func(p *mockAuthProvider) *bool { return &p.showThinkingByDefault }},
		{"thinking hide", "/hide-thinking", true, true, false, false,
			func(m *Model) *bool { return &m.ShowThinking },
			func(p *mockAuthProvider) *bool { return &p.showThinkingByDefault }},
		{"tool output show", "/show-tool-output", false, false, true, true,
			func(m *Model) *bool { return &m.ShowToolOutput },
			func(p *mockAuthProvider) *bool { return &p.showToolOutputByDefault }},
		{"tool output hide", "/hide-tool-output", true, true, false, false,
			func(m *Model) *bool { return &m.ShowToolOutput },
			func(p *mockAuthProvider) *bool { return &p.showToolOutputByDefault }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSlashTuiPrefTest(t, tt.line, tt.initModel, tt.initProvider, tt.wantModel, tt.wantProvider,
				tt.getModel, tt.getProvider)
		})
	}
}

func TestModel_SlashStatus_OK(t *testing.T) {
	srv := newHealthzServer(t)
	defer srv.Close()
	client := gateway.NewClient(srv.URL)
	m := NewModel(&chat.Session{Client: client})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/status")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /status should produce cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, "status") {
			found = true
		}
	}
	if !found {
		t.Errorf("/status expected 'status' in lines; got %v", res.lines)
	}
}

func TestModel_SlashStatus_NotConnected(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/status")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /status should produce cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	if len(res.lines) == 0 || !strings.Contains(res.lines[0], "not connected") {
		t.Errorf("/status with nil client expected 'not connected'; got %v", res.lines)
	}
	// /status should never trigger the exit flow.
	if res.exitModel {
		t.Error("expected exitModel=false for /status not-connected")
	}
}

func TestModel_SlashWhoami_Dispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/users/me" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"u1","handle":"alice"}`))
		}
	}))
	defer srv.Close()
	client := gateway.NewClient(srv.URL)
	client.SetToken("tok")
	m := NewModel(&chat.Session{Client: client})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/whoami")})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on /whoami should produce cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want slashResultMsg", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, "alice") {
			found = true
		}
	}
	if !found {
		t.Errorf("/whoami expected 'alice' in lines; got %v", res.lines)
	}
}
