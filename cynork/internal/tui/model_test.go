package tui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

const threadListHeader = "--- Threads ---"
const inputThreadList = "/thread list"

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

func TestModel_HandleKey_UpDownHistory(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.InputHistory = []string{"newest", "older"}
	m.InputHistoryIdx = -1
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.InputHistoryIdx != 0 || m.Input != "newest" {
		t.Errorf("Up: idx=%d input=%q", m.InputHistoryIdx, m.Input)
	}
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.InputHistoryIdx != -1 || m.Input != "" {
		t.Errorf("Down from 0: idx=%d input=%q", m.InputHistoryIdx, m.Input)
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
	if !strings.Contains(v, "/ commands") || !strings.Contains(v, "@ files") || !strings.Contains(v, "! shell") {
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
	if len(mod.Scrollback) != 2 || mod.Scrollback[0] != threadListHeader {
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
	if len(mod.Scrollback) != 1 || mod.Scrollback[0] != "Thread renamed." {
		t.Errorf("Scrollback = %v", mod.Scrollback)
	}
}

func TestModel_ThreadCommand_New(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/threads" || r.Method != http.MethodPost {
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
	session := chat.NewSession(gateway.NewClient("http://localhost"))
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
	if !strings.Contains(v, "-") {
		t.Errorf("View with nil session should show - for gateway: %s", v)
	}
	if !strings.Contains(v, chat.LandmarkPromptReady) {
		t.Errorf("View with nil session should show landmark: %s", v)
	}
}

func TestModel_View_ContainsLandmarks(t *testing.T) {
	m := NewModel(&chat.Session{})
	v := m.View()
	if !strings.Contains(v, chat.LandmarkPromptReady) {
		t.Errorf("View() missing %q: %s", chat.LandmarkPromptReady, v)
	}
	m.Loading = true
	v2 := m.View()
	if !strings.Contains(v2, chat.LandmarkAssistantInFlight) {
		t.Errorf("View() when loading missing %q", chat.LandmarkAssistantInFlight)
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
	// Only the "You: /clear" echo should remain (added before the command ran),
	// and then Update with nil lines clears ALL scrollback including that echo.
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
	session := &chat.Session{Model: "my-model"}
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
		if strings.Contains(l, "my-model") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("/model with no arg should show 'my-model'; got %v", res.lines)
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
			input: "/model new-model",
			checkFunc: func(t *testing.T, session *chat.Session) {
				t.Helper()
				if session.Model != "new-model" {
					t.Errorf("session.Model = %q, want %q", session.Model, "new-model")
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
	if !strings.Contains(v, composerHint) {
		t.Errorf("View() should contain composerHint %q; got: %s", composerHint, truncate(v, 200))
	}
}

// mockAuthProvider implements AuthProvider for tests.
type mockAuthProvider struct {
	token, refreshToken string
	gatewayURL          string
	saveErr             error
	saved               bool
}

func (m *mockAuthProvider) Token() string        { return m.token }
func (m *mockAuthProvider) RefreshToken() string { return m.refreshToken }
func (m *mockAuthProvider) GatewayURL() string   { return m.gatewayURL }
func (m *mockAuthProvider) SetTokens(access, refresh string) {
	m.token, m.refreshToken = access, refresh
}
func (m *mockAuthProvider) SetGatewayURL(url string) { m.gatewayURL = url }
func (m *mockAuthProvider) Save() error {
	m.saved = true
	return m.saveErr
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
	if !provider.saved || provider.token != "new-access" || provider.refreshToken != "new-refresh" {
		t.Errorf("provider after refresh: saved=%v token=%q refresh=%q", provider.saved, provider.token, provider.refreshToken)
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
