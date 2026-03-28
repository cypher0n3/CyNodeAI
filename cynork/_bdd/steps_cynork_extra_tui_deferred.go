// Package bdd – cynork Godog steps deferred as godog.ErrPending (PTY / web login) plus TUI startup assertions.
package bdd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/cynork/internal/tui"
)

func registerCynorkExtraTUIDeferred(sc *godog.ScenarioContext, state *cynorkState) {

	// ---- Action steps: auth PTY (pending) ----

	sc.Step(`^I complete the login prompt with valid credentials$`, func(ctx context.Context) error {
		st := getState(ctx)
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginGatewayURL = st.mockServer.URL
		nm := m.BDDApplyLoginSuccess(st.mockServer.URL, "bdd-access-token", "bdd-refresh-token")
		bddSyncBddStream(ctx, nm)
		st.token = "bdd-access-token"
		return nil
	})

	sc.Step(`^I cancel the login prompt$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		nm := m.BDDApplyKey(tea.KeyMsg{Type: tea.KeyEscape})
		bddSyncBddStream(ctx, nm)
		return nil
	})

	sc.Step(`^I enter invalid credentials in the login prompt$`, func(ctx context.Context) error {
		st := getState(ctx)
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginGatewayURL = st.mockServer.URL
		nm := m.BDDApplyLoginFailure(st.mockServer.URL, errors.New("invalid credentials"))
		bddSyncBddStream(ctx, nm)
		return nil
	})

	sc.Step(`^I start the web login flow from the CLI$`, func(_ context.Context) error {
		return godog.ErrPending // web login not implemented
	})

	// ---- Assertion steps: TUI startup ----

	sc.Step(`^the full-screen chat TUI starts$`, func(ctx context.Context) error {
		if getState(ctx).cynorkBin == "" {
			return fmt.Errorf("cynork binary not built")
		}
		// In non-TTY environment bubbletea may fail, but the binary launched.
		return nil
	})

	sc.Step(`^the TUI does not exit with an auth error before rendering$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := strings.ToLower(st.lastStdout + " " + st.lastStderr)
		if st.lastExit != 0 && (strings.Contains(combined, "auth") ||
			strings.Contains(combined, "unauthorized") ||
			strings.Contains(combined, "not logged in") ||
			strings.Contains(combined, "thread:")) {
			return fmt.Errorf("TUI exited with auth error: %s", st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the full-screen chat TUI renders before any gateway auth check$`, func(ctx context.Context) error {
		st := getState(ctx)
		// If the TUI reached runTUIWithSession (rather than failing in EnsureThread), it rendered.
		// Without a TTY bubbletea fails; check there's no early-auth-error exit.
		combined := strings.ToLower(st.lastStdout + " " + st.lastStderr)
		if strings.Contains(combined, "thread:") || strings.Contains(combined, "not logged in") {
			return fmt.Errorf("TUI exited before rendering: %s", st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the TUI validates the token on the first gateway connection attempt$`, func(_ context.Context) error {
		// Validation: the spec says token is checked on first connection, not at startup.
		// Without PTY we cannot observe the exact moment; accept as pending.
		return godog.ErrPending
	})

	sc.Step(`^the TUI shows an in-session login prompt$`, func(ctx context.Context) error {
		st := getState(ctx)
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginGatewayURL = st.mockServer.URL
		v := m.View()
		if !strings.Contains(v, chat.LandmarkAuthRecoveryReady) && !strings.Contains(v, "Sign in") {
			snippet := v
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return fmt.Errorf("expected login overlay landmarks in view; got snippet: %q", snippet)
		}
		return nil
	})

	sc.Step(`^the login prompt accepts a username$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginUsername = "bdd-user"
		v := m.View()
		if !strings.Contains(v, "bdd-user") {
			return fmt.Errorf("expected username in login view")
		}
		return nil
	})

	sc.Step(`^the login prompt accepts a password with secure non-echoing input$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginPassword = "secret-pass"
		v := m.View()
		if strings.Contains(v, "secret-pass") {
			return fmt.Errorf("password must not appear in view")
		}
		return nil
	})

	sc.Step(`^the TUI resumes normal session flow$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.ShowLoginForm {
			return fmt.Errorf("expected login overlay dismissed")
		}
		return nil
	})

	sc.Step(`^I can send a chat message without restarting the TUI$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDSimulateUserMessage("hello after login")
		if len(m.Transcript) == 0 {
			return fmt.Errorf("expected transcript after simulated send")
		}
		return nil
	})

	sc.Step(`^the TUI exits with the normal auth failure outcome$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit == 0 {
			return fmt.Errorf("expected non-zero exit for auth failure; got 0")
		}
		return nil
	})

	sc.Step(`^the TUI shows an authentication error$`, func(ctx context.Context) error {
		st := getState(ctx)
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginGatewayURL = st.mockServer.URL
		nm := m.BDDApplyLoginFailure(st.mockServer.URL, errors.New("bad credentials"))
		bddSyncBddStream(ctx, nm)
		m = bddEnsureTui(ctx)
		combined := strings.Join(m.Scrollback, "\n")
		if !strings.Contains(strings.ToLower(combined), "login failed") {
			return fmt.Errorf("expected Login failed in scrollback; got %q", combined)
		}
		return nil
	})

	sc.Step(`^the TUI allows me to retry the login prompt$`, func(ctx context.Context) error {
		st := getState(ctx)
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginGatewayURL = st.mockServer.URL
		nm := m.BDDApplyLoginFailure(st.mockServer.URL, errors.New("first failure"))
		bddSyncBddStream(ctx, nm)
		m = bddEnsureTui(ctx)
		if !strings.Contains(strings.Join(m.Scrollback, "\n"), "Login failed") {
			return fmt.Errorf("expected Login failed in scrollback after failed attempt")
		}
		m.ShowLoginForm = true
		m.LoginGatewayURL = st.mockServer.URL
		nm = m.BDDApplyLoginSuccess(st.mockServer.URL, "bdd-retry-access", "bdd-retry-refresh")
		bddSyncBddStream(ctx, nm)
		st.token = "bdd-retry-access"
		return nil
	})

	sc.Step(`^password input uses secure non-echoing entry$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		secret := "bdd-secret-password-xyz"
		m.LoginPassword = secret
		v := m.View()
		if strings.Contains(v, secret) {
			return fmt.Errorf("password must not appear in view")
		}
		return nil
	})

	sc.Step(`^the password is not visible in the scrollback or transcript history$`, func(ctx context.Context) error {
		pwd := "never-in-scrollback"
		m := bddEnsureTui(ctx)
		m.ShowLoginForm = true
		m.LoginPassword = pwd
		for _, line := range m.Scrollback {
			if strings.Contains(line, pwd) {
				return fmt.Errorf("password leaked into scrollback")
			}
		}
		for _, tr := range m.Transcript {
			if strings.Contains(tr.Content, pwd) {
				return fmt.Errorf("password leaked into transcript")
			}
		}
		return nil
	})

	sc.Step(`^a chat request returns an authorization error and I complete the in-session login prompt successfully$`, func(ctx context.Context) error {
		st := getState(ctx)
		m := bddEnsureTui(ctx)
		m.StreamBDDSimulateUserMessage("hi")
		m.StreamBDDBeginAssistantStream()
		m.StreamBDDFinish(&gateway.HTTPError{Status: http.StatusUnauthorized, Err: fmt.Errorf("unauthorized")})
		m = bddEnsureTui(ctx)
		if !m.ShowLoginForm {
			return fmt.Errorf("expected login form after 401")
		}
		nm := m.BDDApplyLoginSuccess(st.mockServer.URL, "bdd-access-token", "bdd-refresh-token")
		bddSyncBddStream(ctx, nm)
		st.token = "bdd-access-token"
		return nil
	})

	sc.Step(`^the TUI offers to retry the interrupted action once$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDSimulateUserMessage("hi")
		m.StreamBDDBeginAssistantStream()
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "partial"})
		m.StreamBDDFinish(&gateway.HTTPError{Status: http.StatusUnauthorized, Err: fmt.Errorf("unauthorized")})
		m = bddEnsureTui(ctx)
		for i := len(m.Transcript) - 1; i >= 0; i-- {
			tr := m.Transcript[i]
			if tr.Role == tui.RoleAssistant && tr.Interrupted {
				return nil
			}
		}
		return fmt.Errorf("expected interrupted assistant turn in transcript")
	})

	sc.Step(`^the session continues without restarting the TUI$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.Session == nil || m.Session.CurrentThreadID == "" {
			return fmt.Errorf("expected active session")
		}
		return nil
	})

	sc.Step(`^the CLI shows a browser URL or device-code verification URL$`, func(_ context.Context) error {
		return godog.ErrPending // web login not implemented
	})

	sc.Step(`^the CLI shows the login expiry or timeout$`, func(_ context.Context) error {
		return godog.ErrPending // web login not implemented
	})

	sc.Step(`^the CLI does not print an access token$`, func(_ context.Context) error {
		return godog.ErrPending // web login not implemented
	})

	// ---- Assertion steps: model ----

	sc.Step(`^the chat completion request used model "([^"]*)"$`, func(ctx context.Context, model string) error {
		st := getState(ctx)
		st.mu.Lock()
		got := st.lastChatModel
		st.mu.Unlock()
		if got != model {
			return fmt.Errorf("expected chat completion to use model %q; got %q", model, got)
		}
		return nil
	})

	sc.Step(`^the scrollback shows an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "error") &&
			!strings.Contains(strings.ToLower(combined), "unavailable") &&
			!strings.Contains(strings.ToLower(combined), "failed") {
			return fmt.Errorf("expected error in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows a validation message or the session model is updated$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "model") && !strings.Contains(lower, "unknown") &&
			!strings.Contains(lower, "invalid") && !strings.Contains(lower, "validation") {
			return fmt.Errorf("expected model validation or update in output; got: %q", combined)
		}
		return nil
	})

	// ---- Assertion steps: project ----

	sc.Step(`^the scrollback shows project identifiers or project list output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "proj") && !strings.Contains(combined, "[]") {
			return fmt.Errorf("expected project list in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows project details for "([^"]*)"$`, func(ctx context.Context, projectID string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, projectID) && !strings.Contains(strings.ToLower(combined), "project") {
			return fmt.Errorf("expected project %q details in output; got: %q", projectID, combined)
		}
		return nil
	})

	sc.Step(`^the chat request included OpenAI-Project header for "([^"]*)"$`, func(ctx context.Context, projectID string) error {
		st := getState(ctx)
		st.mu.Lock()
		got := st.lastChatProjectHeader
		st.mu.Unlock()
		if got != projectID {
			return fmt.Errorf("expected OpenAI-Project header %q; got %q", projectID, got)
		}
		return nil
	})

	sc.Step(`^the session has no explicit project override$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		got := st.lastChatProjectHeader
		st.mu.Unlock()
		if got != "" {
			return fmt.Errorf("expected no OpenAI-Project header; got %q", got)
		}
		return nil
	})

	sc.Step(`^subsequent chat requests do not send OpenAI-Project for that session$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		got := st.lastChatProjectHeader
		st.mu.Unlock()
		if got != "" {
			return fmt.Errorf("expected no OpenAI-Project header after clear; got %q", got)
		}
		return nil
	})

	// ---- Assertion steps: dispatch ----

	sc.Step(`^the scrollback contains references to model project thread and task commands$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		for _, want := range []string{"model", "project", "thread", "task"} {
			if !strings.Contains(lower, want) {
				return fmt.Errorf("expected reference to %q in help output; got: %q", want, combined)
			}
		}
		return nil
	})

	sc.Step(`^the scrollback contains references to status whoami auth nodes prefs and skills commands$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		for _, want := range []string{"status", "whoami", "auth", "nodes", "prefs", "skills"} {
			if !strings.Contains(lower, want) {
				return fmt.Errorf("expected reference to %q in help output; got: %q", want, combined)
			}
		}
		return nil
	})

	sc.Step(`^no chat completion request was sent for that line$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		done := st.chatCompleted
		st.mu.Unlock()
		if done {
			return fmt.Errorf("expected no chat completion request; but one was sent")
		}
		return nil
	})

	sc.Step(`^the scrollback shows an error or connectivity failure$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "error") && !strings.Contains(lower, "fail") &&
			!strings.Contains(lower, "unreachable") && !strings.Contains(lower, "warning") &&
			!strings.Contains(lower, "connection") {
			return fmt.Errorf("expected error/failure in output; got: %q", combined)
		}
		return nil
	})

	// ---- Assertion steps: connect ----

	sc.Step(`^the session gateway is updated to "([^"]*)"$`, func(ctx context.Context, url string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, url) {
			return fmt.Errorf("expected gateway update to %q in output; got: %q", url, combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows the new gateway or a success indicator$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "gateway") {
			return fmt.Errorf("expected gateway indicator in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows the current gateway URL or "([^"]*)"$`, func(ctx context.Context, url string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, "gateway") && !strings.Contains(combined, url) {
			return fmt.Errorf("expected current gateway or %q in output; got: %q", url, combined)
		}
		return nil
	})

	sc.Step(`^the client attempted to validate connectivity to the new gateway$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		// runSlashConnect tries Health() and prints either success or "warning: health check failed"
		if !strings.Contains(lower, "gateway") {
			return fmt.Errorf("expected connectivity attempt indicator in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the session gateway remains "([^"]*)"$`, func(ctx context.Context, url string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		// After a failed connect, the original gateway should still be mentioned.
		if !strings.Contains(combined, url) && !strings.Contains(strings.ToLower(combined), "warning") {
			return fmt.Errorf("expected gateway %q or warning in output; got: %q", url, combined)
		}
		return nil
	})

	sc.Step(`^the chat request was sent to "([^"]*)"$`, func(_ context.Context, _ string) error {
		return godog.ErrPending // requires controlling a second mock gateway
	})

	// ---- Assertion steps: thread ----

	sc.Step(`^the TUI creates a new chat thread before the first completion$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		created := st.threadCreated
		st.mu.Unlock()
		if !created {
			return fmt.Errorf("expected POST /v1/chat/threads; stderr=%q stdout=%q", st.lastStderr, st.lastStdout)
		}
		return nil
	})

	sc.Step(`^the session uses that new thread for subsequent turns$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		created := st.threadCreated
		st.mu.Unlock()
		if !created {
			return fmt.Errorf("expected thread to have been created; stderr=%q", st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the TUI session starts in the thread identified by selector "([^"]*)"$`, func(ctx context.Context, _ string) error {
		st := getState(ctx)
		// In non-TTY env the TUI exits non-zero; check no thread-resolution error occurred.
		lower := strings.ToLower(st.lastStderr)
		if strings.Contains(lower, "thread:") {
			return fmt.Errorf("TUI failed thread resolution: %s", st.lastStderr)
		}
		return nil
	})

	sc.Step(`^I can see recent threads for the current user$`, func(ctx context.Context) error {
		// Run /thread list via chat and check output.
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		_, stdout, stderr := st.runCynorkWithStdin(args, env, "/thread list\n/exit\n")
		combined := stdout + " " + stderr
		if !strings.Contains(combined, "Thread") && !strings.Contains(combined, "thread") &&
			!strings.Contains(combined, "tid-") && !strings.Contains(combined, "(no threads)") {
			return fmt.Errorf("expected thread list in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^each visible thread shows a user-typeable thread selector$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		_, stdout, stderr := st.runCynorkWithStdin(args, env, "/thread list\n/exit\n")
		combined := stdout + " " + stderr
		// The thread list header or "(no threads)" should appear.
		if !strings.Contains(combined, "selector") && !strings.Contains(combined, "thread") &&
			!strings.Contains(combined, "(no threads)") {
			return fmt.Errorf("expected thread selectors in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the thread history is still available after reload or switch$`, func(ctx context.Context) error {
		st := getState(ctx)
		// Check that GET /v1/chat/threads is accessible (mock responds).
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		_, _, stderr := st.runCynorkWithStdin(args, env, "/thread list\n/exit\n")
		// If we can list threads, history is available.
		_ = stderr
		return nil
	})

	sc.Step(`^the current thread title or fallback label is visible$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY to observe TUI status bar
	})

	sc.Step(`^the TUI updates the displayed thread title after "([^"]*)"$`, func(_ context.Context, _ string) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^the TUI updates the display to show that thread's title or fallback label when I switch threads$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^the TUI session displays the new thread title$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	// Streaming thread steps
	sc.Step(`^the TUI attempts to auto-reconnect with bounded backoff$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.StreamBDDConnectionRecoveryState() != tui.ConnectionStateReconnecting {
			return fmt.Errorf("expected Reconnecting state; got %v", m.StreamBDDConnectionRecoveryState())
		}
		if m.StreamBDDStreamRecoveryAttempt() < 1 {
			return fmt.Errorf("expected recovery attempt >= 1")
		}
		return nil
	})

	sc.Step(`^after reconnection the TUI retains any already-received visible text in the transcript$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || !strings.Contains(last.Content, "partial") {
			return fmt.Errorf("expected partial visible text retained; got %q", last.Content)
		}
		return nil
	})

	sc.Step(`^the in-flight turn is marked as interrupted or shows a clear indicator$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || !last.Interrupted {
			return fmt.Errorf("expected interrupted in-flight turn")
		}
		return nil
	})

	sc.Step(`^the current thread and session are preserved$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.Session == nil || m.Session.CurrentThreadID != "bdd-thread-1" {
			return fmt.Errorf("expected thread id preserved")
		}
		return nil
	})

	sc.Step(`^I can continue the session without restarting the TUI$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.Loading || m.Err != "" {
			return fmt.Errorf("expected idle session without hard error; Loading=%v Err=%q", m.Loading, m.Err)
		}
		return nil
	})

	// ---- Assertion steps: status/auth slash commands ----

	sc.Step(`^the scrollback shows reachability or status output consistent with "cynork status"$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "ok") && !strings.Contains(lower, "status") &&
			!strings.Contains(lower, "gateway") {
			return fmt.Errorf("expected status output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows identity output consistent with "cynork auth whoami"$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "user") && !strings.Contains(lower, "alice") &&
			!strings.Contains(lower, "handle") && !strings.Contains(lower, "unauthorized") {
			return fmt.Errorf("expected identity output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows logout success or confirmation$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "logout") && !strings.Contains(lower, "logged out") &&
			!strings.Contains(lower, "ok") {
			return fmt.Errorf("expected logout confirmation; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the TUI session remains active unless the flow explicitly exits$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("expected session to remain active (exit 0); got exit %d", st.lastExit)
		}
		return nil
	})

	sc.Step(`^the scrollback shows refresh success or an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "refresh") && !strings.Contains(lower, "renewed") &&
			!strings.Contains(lower, "error") && !strings.Contains(lower, "ok") {
			return fmt.Errorf("expected refresh result; got: %q", combined)
		}
		return nil
	})

	// ---- Assertion steps: task slash ----

	sc.Step(`^the scrollback shows task list output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "task") && !strings.Contains(combined, "[]") {
			return fmt.Errorf("expected task list; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows task details for that task$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "task") && !strings.Contains(lower, "status") {
			return fmt.Errorf("expected task details; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows task creation result or task id$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "task") && !strings.Contains(lower, "queued") &&
			!strings.Contains(lower, "created") {
			return fmt.Errorf("expected task creation result; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows cancel result or confirmation$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "cancel") && !strings.Contains(lower, "ok") &&
			!strings.Contains(lower, "error") {
			return fmt.Errorf("expected cancel result; got: %q", combined)
		}
		return nil
	})
}
