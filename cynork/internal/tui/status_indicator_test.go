package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestCombineTeaCmds(t *testing.T) {
	t.Parallel()
	if combineTeaCmds(nil, nil) != nil {
		t.Fatal("expected nil")
	}
	a := func() tea.Msg { return tea.Quit() }
	if combineTeaCmds(a, nil) == nil {
		t.Fatal("expected non-nil")
	}
	if combineTeaCmds(nil, a) == nil {
		t.Fatal("expected non-nil")
	}
	if combineTeaCmds(a, a) == nil {
		t.Fatal("expected non-nil")
	}
}

func TestModel_Update_GatewayHealthResult(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.SetHealthPollInterval(5)
	updated, cmd := m.Update(gatewayHealthResultMsg{state: gatewayHealthOK})
	if cmd != nil {
		t.Errorf("cmd = %v", cmd)
	}
	mod := updated.(*Model)
	if mod.gatewayHealth != gatewayHealthOK {
		t.Errorf("gatewayHealth = %v", mod.gatewayHealth)
	}
}

func TestGatewayHealthCheckCmd_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	cl := gateway.NewClient(srv.URL)
	m := NewModel(&chat.Session{Client: cl})
	msg := m.gatewayHealthCheckCmd()()
	res, ok := msg.(gatewayHealthResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if res.state != gatewayHealthOK {
		t.Errorf("state = %v", res.state)
	}
}
