package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestBDDApplyLoginSuccess_DismissesOverlay(t *testing.T) {
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://gw")})
	m.ShowLoginForm = true
	nm := m.BDDApplyLoginSuccess("http://gw", "acc", "ref")
	mm, ok := nm.(*Model)
	if !ok || mm.ShowLoginForm {
		t.Fatalf("expected login overlay dismissed after BDDApplyLoginSuccess")
	}
}

func TestBDDApplyLoginFailure_DismissesOverlay(t *testing.T) {
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://gw")})
	m.ShowLoginForm = true
	nm := m.BDDApplyLoginFailure("http://gw", errors.New("nope"))
	mm, ok := nm.(*Model)
	if !ok || mm.ShowLoginForm {
		t.Fatalf("expected login overlay dismissed after BDDApplyLoginFailure")
	}
}

func TestBDDApplyKey_EscapeDismissesOverlay(t *testing.T) {
	m := NewModel(&chat.Session{Client: gateway.NewClient("http://gw")})
	m.ShowLoginForm = true
	nm := m.BDDApplyKey(tea.KeyMsg{Type: tea.KeyEscape})
	mm, ok := nm.(*Model)
	if !ok || mm.ShowLoginForm {
		t.Fatalf("expected login overlay dismissed after Esc")
	}
}
