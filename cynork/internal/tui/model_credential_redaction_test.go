package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestView_LoginOverlayDoesNotLeakPassword(t *testing.T) {
	t.Parallel()
	cl := gateway.NewClient("http://gw")
	sess := chat.NewSession(cl)
	m := NewModel(sess)
	m.Width = 80
	m.Height = 24
	m.ShowLoginForm = true
	m.LoginGatewayURL = "http://gw"
	m.LoginUsername = "alice"
	secret := "correct-horse-battery-staple-99"
	m.LoginPassword = secret
	v := m.View()
	if strings.Contains(v, secret) {
		t.Fatalf("View leaked raw password substring")
	}
}

func TestScrollbackAfterLoginResult_DoesNotContainAccessToken(t *testing.T) {
	t.Parallel()
	cl := gateway.NewClient("http://gw")
	sess := chat.NewSession(cl)
	m := NewModel(sess)
	m.Width = 80
	m.Height = 24
	m.ShowLoginForm = true
	tok := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.leak-me-not"
	nm, _ := m.Update(loginResultMsg{
		GatewayURL:   "http://gw",
		AccessToken:  tok,
		RefreshToken: "refresh-xyz",
	})
	mm := nm.(*Model)
	combined := strings.Join(mm.Scrollback, "\n")
	if strings.Contains(combined, tok) {
		t.Fatalf("scrollback must not contain raw access token; got lines %v", mm.Scrollback)
	}
	if strings.Contains(combined, "refresh-xyz") {
		t.Fatalf("scrollback must not contain refresh token")
	}
}

func TestTranscriptNeverContainsLoginPasswordAfterKeyInput(t *testing.T) {
	t.Parallel()
	cl := gateway.NewClient("http://gw")
	sess := chat.NewSession(cl)
	m := NewModel(sess)
	m.Width = 80
	m.Height = 24
	m.ShowLoginForm = true
	m.LoginFocusedField = 2
	secret := "typeme"
	for _, r := range secret {
		nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(r))})
		m = nm.(*Model)
	}
	for _, turn := range m.Transcript {
		if strings.Contains(turn.Content, secret) {
			t.Fatalf("transcript leaked password in turn: %q", turn.Content)
		}
	}
	combined := strings.Join(collectTranscriptPlain(m), "")
	if strings.Contains(combined, secret) {
		t.Fatalf("plain transcript aggregation leaked password")
	}
}

func collectTranscriptPlain(m *Model) []string {
	var out []string
	for i := range m.Transcript {
		out = append(out, m.Transcript[i].Content)
		for _, p := range m.Transcript[i].Parts {
			if p.Text != "" {
				out = append(out, p.Text)
			}
		}
	}
	return out
}
