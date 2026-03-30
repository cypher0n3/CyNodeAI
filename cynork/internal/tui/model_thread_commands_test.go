package tui

import (
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

const (
	threadListHeader   = "--- Threads (use ordinal, id, or title with /thread switch <selector>) ---"
	inputThreadList    = "/thread list"
	pathChatThreads    = "/v1/chat/threads"
	testThreadNewInput = "/thread new"
)

func TestModel_InputHistory_PushAndNavigate(t *testing.T) {
	const inputFirst, inputSecond = "first", "second"
	m := NewModel(&chat.Session{})
	m.pushInputHistory("")
	if len(m.InputHistory) != 0 {
		t.Fatalf("pushInputHistory(\"\") should not append: %v", m.InputHistory)
	}
	m.pushInputHistory(inputFirst)
	m.pushInputHistory(inputSecond)
	if len(m.InputHistory) != 2 || m.InputHistory[0] != inputSecond || m.InputHistory[1] != inputFirst {
		t.Fatalf("InputHistory = %v", m.InputHistory)
	}
	m.pushInputHistory(inputSecond) // dedupe
	if len(m.InputHistory) != 2 {
		t.Fatalf("dedupe: InputHistory = %v", m.InputHistory)
	}
	steps := []struct {
		up        bool
		wantIdx   int
		wantInput string
	}{
		{true, 0, inputSecond},
		{true, 1, inputFirst},
		{true, 1, inputFirst},
		{false, 0, inputSecond},
		{false, -1, ""},
	}
	for i, s := range steps {
		m.NavigateInputHistory(s.up)
		if m.InputHistoryIdx != s.wantIdx || m.Input != s.wantInput {
			t.Fatalf("step %d NavigateInputHistory(%v): idx=%d input=%q want idx=%d %q",
				i, s.up, m.InputHistoryIdx, m.Input, s.wantIdx, s.wantInput)
		}
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
	if len(mod.Scrollback) != 2 || mod.Scrollback[0] != ScrollbackSystemLinePrefix+threadListHeader {
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
	if len(mod.Scrollback) != 1 || mod.Scrollback[0] != ScrollbackSystemLinePrefix+"Thread renamed." {
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
	if cmd == nil {
		t.Fatal("handleKey(/thread new) expected async cmd")
	}
	updated, cmd2 := mod.Update(cmd())
	if cmd2 != nil {
		t.Fatalf("unexpected cmd after thread new: %v", cmd2)
	}
	out := updated.(*Model)
	if session.CurrentThreadID != "new-tid" {
		t.Errorf("CurrentThreadID = %q", session.CurrentThreadID)
	}
	found := false
	for _, s := range out.Scrollback {
		if strings.Contains(s, "New thread:") && strings.Contains(s, "new-tid") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Scrollback missing new thread line: %v", out.Scrollback)
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
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("handleKey(/thread switch) expected async cmd")
	}
	updated, cmd2 := mod.Update(cmd())
	if cmd2 != nil {
		t.Fatalf("unexpected cmd after thread switch: %v", cmd2)
	}
	out := updated.(*Model)
	if session.CurrentThreadID != "thread-123" {
		t.Errorf("CurrentThreadID = %q", session.CurrentThreadID)
	}
	found := false
	for _, s := range out.Scrollback {
		if strings.Contains(s, "Switched to thread") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Scrollback = %v", out.Scrollback)
	}
}

func TestModel_ThreadCommand_SwitchListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"list failed"}`))
	}))
	defer server.Close()
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.Client.SetToken("tok")
	m := NewModel(session)
	m.Input = "/thread switch 1"
	m.Scrollback = nil
	mod, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("handleKey(/thread switch) expected async cmd")
	}
	updated, cmd2 := mod.Update(cmd())
	if cmd2 != nil {
		t.Fatalf("unexpected cmd: %v", cmd2)
	}
	out := updated.(*Model)
	found := false
	for _, s := range out.Scrollback {
		if strings.Contains(s, "Error:") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected list error in scrollback: %v", out.Scrollback)
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
