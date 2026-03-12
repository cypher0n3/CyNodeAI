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

func TestModel_HandleKey_Quit(t *testing.T) {
	m := NewModel(&chat.Session{})
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if mod != m {
		t.Error("handleKey(ctrl+c) changed model")
	}
	if cmd == nil {
		t.Error("handleKey(ctrl+c) returned nil cmd")
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
	if len(m.Scrollback) != 1 || m.Scrollback[0] != "You: hello" {
		t.Errorf("Scrollback = %v", m.Scrollback)
	}
	if !m.Loading {
		t.Error("Loading not set")
	}
	msg := cmd()
	if _, ok := msg.(sendResult); !ok {
		t.Errorf("cmd() = %T", msg)
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
	if !strings.Contains(v, "Enter send") || !strings.Contains(v, "Shift+Enter newline") {
		t.Errorf("View() should show composer hint: %s", v)
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

func TestModel_SendCmd(t *testing.T) {
	transport := &mockTransport{visible: "reply"}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	cmd := m.sendCmd("hello")
	if cmd == nil {
		t.Fatal("sendCmd returned nil")
	}
	msg := cmd()
	res, ok := msg.(sendResult)
	if !ok {
		t.Fatalf("cmd() returned %T", msg)
	}
	if res.err != nil {
		t.Errorf("sendResult err = %v", res.err)
	}
	if res.visible != "reply" {
		t.Errorf("sendResult visible = %q", res.visible)
	}
}

func TestModel_SendCmd_TransportError(t *testing.T) {
	transport := &mockTransport{err: errors.New("network error")}
	session := &chat.Session{Transport: transport}
	m := NewModel(session)
	cmd := m.sendCmd("hello")
	msg := cmd()
	res, ok := msg.(sendResult)
	if !ok {
		t.Fatalf("cmd() returned %T", msg)
	}
	if res.err == nil {
		t.Error("sendResult err = nil")
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
