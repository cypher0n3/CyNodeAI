package tui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestModel_Update_UnknownMsg(t *testing.T) {
	m := NewModel(&chat.Session{})
	type unknownMsg struct{}
	upd, cmd := m.Update(unknownMsg{})
	if cmd != nil {
		t.Fatalf("unexpected cmd %v", cmd)
	}
	_ = upd.(*Model)
}

func TestModel_UpdateForKey_ViewportPageKeys(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Scrollback = []string{"You: hi", "Assistant: there"}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	_ = cmd
	_, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	_ = cmd2
}

func TestModel_Update_MouseIgnoredWhenLoginForm(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	upd, cmd := m.Update(tea.MouseMsg{})
	if cmd != nil {
		t.Errorf("cmd = %v", cmd)
	}
	_ = upd.(*Model)
}

func TestModel_Update_ComposerUp_Multiline(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Input = "first\nsecond"
	m.syncInputCursorEnd()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Errorf("cmd = %v", cmd)
	}
	if strings.Count(m.Input, "\n") != 1 {
		t.Fatalf("unexpected input: %q", m.Input)
	}
}

func TestModel_Update_CopyClipboardErr(t *testing.T) {
	m := NewModel(&chat.Session{})
	upd, _ := m.Update(copyClipboardResultMsg{err: errors.New("no clipboard"), successDetail: ""})
	mod := upd.(*Model)
	if !strings.Contains(mod.ClipNote, "no clipboard") {
		t.Errorf("ClipNote = %q", mod.ClipNote)
	}
}

func TestModel_Update_CopyClipboardDefaultSuccessDetail(t *testing.T) {
	m := NewModel(&chat.Session{})
	upd, _ := m.Update(copyClipboardResultMsg{err: nil, successDetail: ""})
	mod := upd.(*Model)
	if mod.ClipNote != "Copied to clipboard." {
		t.Errorf("ClipNote = %q", mod.ClipNote)
	}
}

func TestModel_ThreadCommand_New_GatewayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathChatThreads && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := chat.NewSession(client)
	m := NewModel(session)
	m.Input = testThreadNewInput
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
	found := false
	for _, line := range m.Scrollback {
		if strings.Contains(line, "Error:") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error in scrollback: %v", m.Scrollback)
	}
}

func TestModel_SlashUnknownCommand(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Input = "/notacommand"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	msg := cmd()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if len(res.lines) == 0 || !strings.Contains(res.lines[0], "Unknown") {
		t.Errorf("lines = %v", res.lines)
	}
}

func TestModel_HandleComposerTextInput_MultiRune(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab")})
	if m.Input != "ab" {
		t.Errorf("Input = %q", m.Input)
	}
}

func TestModel_View_LoginOverlayNarrow(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.Width = 24
	m.Height = 12
	m.LoginGatewayURL = "http://gw"
	if v := m.View(); v == "" {
		t.Fatal("empty view")
	}
}

func TestLastAssistantPlain(t *testing.T) {
	if LastAssistantPlain(nil) != "" || LastAssistantPlain([]string{"You: x"}) != "" {
		t.Fatal("expected empty")
	}
	if got := LastAssistantPlain([]string{"meta", "Assistant: last"}); got != "last" {
		t.Fatalf("got %q", got)
	}
}
