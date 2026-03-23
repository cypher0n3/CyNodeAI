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
	a := tea.Quit
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
		if r.URL.Path != testHealthzPath {
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

func TestGatewayHealthCheckCmd_NoClient(t *testing.T) {
	m := NewModel(&chat.Session{})
	msg := m.gatewayHealthCheckCmd()()
	res, ok := msg.(gatewayHealthResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if res.state != gatewayHealthNoClient {
		t.Errorf("state = %v", res.state)
	}
}

func TestGatewayHealthCheckCmd_Down(t *testing.T) {
	cl := gateway.NewClient("http://127.0.0.1:9")
	m := NewModel(&chat.Session{Client: cl})
	msg := m.gatewayHealthCheckCmd()()
	res, ok := msg.(gatewayHealthResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if res.state != gatewayHealthDown {
		t.Errorf("state = %v", res.state)
	}
}

func TestMaybeStartGatewayHealthPollOnce_DisabledOrNoClient(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.SetHealthPollInterval(0)
	if m.maybeStartGatewayHealthPollOnce() != nil {
		t.Fatal("expected nil when interval 0")
	}
	m.SetHealthPollInterval(5)
	if m.maybeStartGatewayHealthPollOnce() != nil {
		t.Fatal("expected nil without client")
	}
}

func TestMaybeStartGatewayHealthPollOnce_Starts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	m := NewModel(&chat.Session{Client: gateway.NewClient(srv.URL)})
	m.SetHealthPollInterval(5)
	if m.maybeStartGatewayHealthPollOnce() == nil {
		t.Fatal("expected batch cmd")
	}
	if cmd := m.maybeStartGatewayHealthPollOnce(); cmd != nil {
		t.Fatal("second call should not start again")
	}
}

func TestHandleGatewayHealthPoll(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.SetHealthPollInterval(0)
	_, cmd := m.handleGatewayHealthPoll()
	if cmd != nil {
		t.Fatal("expected nil cmd when polling off")
	}
	m.SetHealthPollInterval(5)
	_, cmd = m.handleGatewayHealthPoll()
	if cmd == nil {
		t.Fatal("expected health check cmd")
	}
}

func TestStatusIndicatorStyleBusy_NoColor(t *testing.T) {
	m := NewModel(&chat.Session{NoColor: true})
	m.Loading = true
	if s := m.renderGatewayStatusIndicator(); s == "" {
		t.Fatal("busy no-color")
	}
}

func TestRenderGatewayStatusIndicator(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	if s := m.renderGatewayStatusIndicator(); s == "" {
		t.Fatal("busy glyph")
	}
	m.Loading = false
	m.SetHealthPollInterval(0)
	if s := m.renderGatewayStatusIndicator(); s == "" {
		t.Fatal("idle legacy")
	}
	m.SetHealthPollInterval(5)
	m.gatewayHealth = gatewayHealthOK
	m.Session.NoColor = true
	if s := m.renderGatewayStatusIndicator(); s == "" {
		t.Fatal("ok no-color")
	}
	m.Session.NoColor = false
	if s := m.renderGatewayStatusIndicator(); s == "" {
		t.Fatal("ok color")
	}
	m.gatewayHealth = gatewayHealthDown
	m.Session.NoColor = true
	_ = m.renderGatewayStatusIndicator()
	m.Session.NoColor = false
	_ = m.renderGatewayStatusIndicator()
	m.gatewayHealth = gatewayHealthNoClient
	_ = m.renderGatewayStatusIndicator()
	m.gatewayHealth = gatewayHealthUnknown
	_ = m.renderGatewayStatusIndicator()
}
