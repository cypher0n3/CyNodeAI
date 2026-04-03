package tui

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

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
	m.LoginUsername = loginTestUsername
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
	m.LoginUsername = loginTestUsername
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
	m.pushInputHistory(testSampleWordHello)
	m.pushInputHistory(testSampleWordHello)
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
		if r.URL.Path == testHealthzPath {
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
	if client.BaseURL() != srv.URL {
		t.Errorf("expected BaseURL=%s, got %q", srv.URL, client.BaseURL())
	}
}

func TestModel_SlashConnect_UpdateURL_HealthWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testHealthzPath {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	client := gateway.NewClient("http://old:1")
	m := NewModel(&chat.Session{Client: client})
	m.SetAuthProvider(&mockAuthProvider{gatewayURL: "http://old:1"})
	msg := m.slashConnectCmd(srv.URL)()
	res, ok := msg.(slashResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	found := false
	for _, l := range res.lines {
		if strings.Contains(l, "Warning") || strings.Contains(l, "health") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected health warning in lines: %v", res.lines)
	}
}

func TestModel_HandleEnterKey_BlocksPlainChatWhenLoading(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	m.Input = testSampleWordHello
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected nil cmd, got %v", cmd)
	}
	if m.Input != testSampleWordHello {
		t.Errorf("Input cleared: %q", m.Input)
	}
}

func TestModel_HandleEnterKey_AllowsSlashWhileLoading(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	m.Input = "/version"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for /version while loading")
	}
	if m.Input != "" {
		t.Errorf("Input should be cleared after dispatch: %q", m.Input)
	}
}

func TestModel_HandleEnterKey_AllowsShellWhileLoading(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Loading = true
	m.Input = "!echo hi"
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for shell escape while loading")
	}
}

func TestModel_SlashMenuOpen_NavigationYieldsNilCmd(t *testing.T) {
	setup := func() *Model {
		m := NewModel(&chat.Session{})
		m.Input = testSlashThreadFilter
		m.Width = 80
		return m
	}
	t.Run("handleKeyCtrlUp", func(t *testing.T) {
		_, cmd := setup().handleKey(tea.KeyMsg{Type: tea.KeyCtrlUp})
		if cmd != nil {
			t.Fatalf("expected nil cmd, got %v", cmd)
		}
	})
	t.Run("updateKeyUp", func(t *testing.T) {
		m := setup()
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		if cmd != nil {
			t.Errorf("slash menu nav cmd = %v", cmd)
		}
	})
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
		if r.URL.Path == pathV1UsersMe {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"id":"u1","handle":"%s"}`, loginTestUsername)
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
		if strings.Contains(l, loginTestUsername) {
			found = true
		}
	}
	if !found {
		t.Errorf("/whoami expected %q in lines; got %v", loginTestUsername, res.lines)
	}
}

func TestView_MainLayoutRendersScrollbackSlashMenuClipErr(t *testing.T) {
	m := NewModel(&chat.Session{ProjectID: "p1", Model: "m1", CurrentThreadID: "thread-12345678"})
	m.Scrollback = []string{"You: hello", "Assistant: world", ScrollbackSystemLinePrefix + "meta"}
	m.Input = testSlashThreadFilter
	m.slashMenuSel = 0
	m.ClipNote = "copied"
	m.Err = "err"
	m.Width = 80
	m.Height = 24
	v := m.View()
	if v == "" || !strings.Contains(v, "hello") {
		t.Fatalf("unexpected view: %q", truncate(v, 200))
	}
}

func TestView_LoginOverlay(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.Width = 80
	m.Height = 24
	v := m.View()
	if v == "" {
		t.Fatal("empty login overlay")
	}
}

func TestNavSlashMenuAndApplyCompletion(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Input = testSlashThreadFilter
	m.slashMenuSel = 0
	m.navSlashMenu(false)
	m.navSlashMenu(true)
	m.applySlashCompletion()
	if !strings.HasPrefix(strings.TrimSpace(activeComposerLine(m.Input)), "/") {
		t.Errorf("composer after completion: %q", m.Input)
	}
}

func TestModel_Update_MouseClipClearStreamPoll(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Update(tea.MouseMsg{})
	upd, _ := m.Update(clipNoteClearMsg{})
	mod := upd.(*Model)
	if mod.ClipNote != "" {
		t.Fatalf("ClipNote = %q", mod.ClipNote)
	}
	m.streamCh = make(chan chat.ChatStreamDelta)
	_, cmd := m.Update(streamPollMsg{})
	if cmd == nil {
		t.Fatal("expected stream poll cmd")
	}
}

func TestModel_HandleKey_CtrlY(t *testing.T) {
	m := NewModel(&chat.Session{})
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlY})
	if cmd == nil {
		t.Fatal("expected ctrl+y cmd")
	}
	m.Scrollback = []string{"Assistant: hello"}
	_, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlY})
	if cmd == nil {
		t.Fatal("expected ctrl+y cmd with assistant text")
	}
}

func TestScheduleClipNoteClear_returnsTick(t *testing.T) {
	m := NewModel(&chat.Session{})
	cmd := m.scheduleClipNoteClear()
	if cmd == nil {
		t.Fatal("expected tick cmd")
	}
}

func TestLoginOverlayInnerWidth_edges(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 2
	if w := m.loginOverlayInnerWidth(); w < 1 {
		t.Fatalf("inner width = %d", w)
	}
	m.Width = 200
	if w := m.loginOverlayInnerWidth(); w > loginBoxMaxInnerW {
		t.Fatalf("expected cap at %d, got %d", loginBoxMaxInnerW, w)
	}
}

func TestMergeLoginBoxOntoMainView_shortMain(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 40
	th := m.newLoginPanelTheme(30)
	out := m.mergeLoginBoxOntoMainView("x", "line1\nline2\nline3", th)
	if !strings.Contains(out, "line1") {
		t.Fatalf("unexpected overlay merge: %q", out)
	}
}

func TestRenderLoginOverlay_smoke(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 80
	m.Height = 24
	m.LoginGatewayURL = loginTestGatewayURL
	m.LoginUsername = "u"
	m.LoginPassword = "p"
	_ = m.renderLoginOverlay("main view line")
}

func TestHandleComposerCtrlDown_movesWithinMultilineInput(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Width = 40
	m.Input = "line1\nline2"
	m.inputCursor = len("line1\n")
	_, cmd := m.handleComposerAfterGlobalChords(tea.KeyMsg{Type: tea.KeyCtrlDown})
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
}

func TestRenderLoginFieldLine_focusedEmpty(t *testing.T) {
	m := NewModel(&chat.Session{})
	th := m.newLoginPanelTheme(40)
	s := m.renderLoginFieldLine(th, "Gateway URL:", "", 0, true)
	if s == "" {
		t.Fatal("expected non-empty line")
	}
}

func TestCmdCopyLastAssistant_noAssistantMessage(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.Scrollback = []string{"You: hi"}
	teaCmd := m.cmdCopyLastAssistant()
	if teaCmd == nil {
		t.Fatal("expected cmd")
	}
	msg := teaCmd()
	cm, ok := msg.(copyClipboardResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if cm.successDetail != "No assistant message to copy." {
		t.Fatalf("detail = %q", cm.successDetail)
	}
}

func TestView_PlainNoColorAndEmptyScrollback(t *testing.T) {
	m := NewModel(&chat.Session{Plain: true})
	m.Width = 80
	m.Height = 24
	_ = m.View()
	m.Scrollback = []string{"You: hi", "Assistant: `inline`"}
	m.Session.Plain = false
	_ = m.View()
	m.Session.NoColor = true
	_ = m.View()
	m.Scrollback = nil
	_ = m.View()
}

// stubAuthProvider implements AuthProvider for tests (minimal).
type stubAuthProvider struct {
	refresh string
}

func (s *stubAuthProvider) Token() string                     { return "t" }
func (s *stubAuthProvider) RefreshToken() string              { return s.refresh }
func (s *stubAuthProvider) GatewayURL() string                { return "http://localhost" }
func (s *stubAuthProvider) SetTokens(_, _ string)             {}
func (s *stubAuthProvider) SetGatewayURL(_ string, _ bool)    {}
func (s *stubAuthProvider) Save() error                       { return nil }
func (s *stubAuthProvider) ShowThinkingByDefault() bool       { return false }
func (s *stubAuthProvider) SetShowThinkingByDefault(_ bool)   {}
func (s *stubAuthProvider) ShowToolOutputByDefault() bool     { return false }
func (s *stubAuthProvider) SetShowToolOutputByDefault(_ bool) {}

func TestModel_Update_ProactiveTokenRefreshWhileLoading(t *testing.T) {
	srv := newStubRefreshHTTPServer(t)
	cl := gateway.NewClient(srv.URL)
	cl.SetToken("old")
	m := NewModel(&chat.Session{Client: cl})
	m.SetAuthProvider(&stubAuthProvider{refresh: "rt"})
	m.Loading = true
	_, cmd := m.Update(proactiveTokenRefreshMsg{})
	if cmd == nil {
		t.Fatal("expected token refresh cmd while Loading (streaming)")
	}
	msg := cmd()
	res, ok := msg.(tokenRefreshResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if res.err != nil || res.resp == nil || res.resp.AccessToken != testStubRefreshedAccessToken {
		t.Fatalf("refresh result: %+v err=%v", res.resp, res.err)
	}
}

func TestModel_HandleProactiveTokenRefreshCmd(t *testing.T) {
	srv := newStubRefreshHTTPServer(t)
	cl := gateway.NewClient(srv.URL)
	cl.SetToken("old")
	m := NewModel(&chat.Session{Client: cl})
	m.SetAuthProvider(&stubAuthProvider{refresh: "rt"})
	_, cmd := m.handleProactiveTokenRefresh()
	if cmd == nil {
		t.Fatal("expected token refresh cmd")
	}
	msg := cmd()
	res, ok := msg.(tokenRefreshResultMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if res.err != nil || res.resp == nil || res.resp.AccessToken != testStubRefreshedAccessToken {
		t.Fatalf("refresh result: %+v err=%v", res.resp, res.err)
	}
}

func TestLoginFieldString_DefaultBranch(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.LoginFocusedField = 42
	if p := m.loginFieldString(); p != &m.LoginGatewayURL {
		t.Fatal("default field")
	}
	if p := m.loginFieldCursorPtr(); p != &m.LoginGatewayCursor {
		t.Fatal("default cursor ptr")
	}
}

func TestModel_HandleLoginFormKey_MotionTabEsc(t *testing.T) {
	m := NewModel(&chat.Session{})
	m.ShowLoginForm = true
	m.LoginGatewayURL = "http://gw"
	m.LoginUsername = "hello world"
	m.LoginFocusedField = 1
	m.LoginUsernameCursor = len(m.LoginUsername)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlLeft})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlRight})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	m.LoginFocusedField = 2
	m.LoginPassword = "secret"
	m.LoginPasswordCursor = len(m.LoginPassword)
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlLeft})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.ShowLoginForm {
		t.Fatal("esc should close login")
	}
}

func TestModel_Update_TokenRefreshResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(srv.Close)
	m := NewModel(&chat.Session{Client: gateway.NewClient(srv.URL)})
	m.SetAuthProvider(&stubAuthProvider{refresh: "r"})
	upd, _ := m.Update(tokenRefreshResultMsg{resp: &userapi.LoginResponse{AccessToken: "na", RefreshToken: "nr"}})
	mod := upd.(*Model)
	if mod.Session.Client.Token() != "na" {
		t.Fatalf("token not updated: %q", mod.Session.Client.Token())
	}
}

func TestModel_Update_TokenRefreshErrIgnored(t *testing.T) {
	m := NewModel(&chat.Session{})
	upd, _ := m.Update(tokenRefreshResultMsg{err: errors.New("fail")})
	if upd.(*Model) == nil {
		t.Fatal("expected model")
	}
}
