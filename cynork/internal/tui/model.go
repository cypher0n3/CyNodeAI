// Package tui provides the full-screen TUI for cynork. See docs/tech_specs/cynork_tui.md.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

// defaultPlaceholder is shown for empty project/model in the status bar and orEmpty.
const defaultPlaceholder = "(default)"

// loginFormCursor is the cursor shown in the focused login form field.
const loginFormCursor = "▌"

// assistantPrefix is the prefix for assistant messages in the scrollback.
const assistantPrefix = "Assistant: "

const maxInputHistory = 50

// sendResult is the message returned when a SendMessage completes (non-streaming fallback).
type sendResult struct {
	visible string
	err     error
}

// streamDeltaMsg carries one incremental delta or an amendment (secret_redaction) for the in-flight turn.
type streamDeltaMsg struct {
	delta     string
	amendment string
}

// streamDoneMsg signals that the active streaming turn is complete.
type streamDoneMsg struct {
	responseID string
	err        error
}

// streamStartMsg is sent when StreamMessage succeeds and carries the channel to the
// main Update loop so that m.streamCh is never written from a goroutine (data-race prevention).
type streamStartMsg struct {
	ch <-chan chat.ChatStreamDelta
}

// threadListResult is the message when ListThreads completes.
type threadListResult struct {
	lines []string
	err   error
}

// threadRenameResult is the message when PatchThreadTitle completes.
type threadRenameResult struct {
	err error
}

// ensureThreadResult is the message when EnsureThread completes (after login).
type ensureThreadResult struct {
	threadID string
	err      error
}

// openLoginFormMsg opens the in-TUI login overlay (per REQ-CLIENT-0190, Auth Recovery).
type openLoginFormMsg struct{}

// loginResultMsg is sent after a login attempt (success or failure).
type loginResultMsg struct {
	GatewayURL   string
	AccessToken  string
	RefreshToken string
	Err          error
}

// streamPollMsg is sent when a stream read times out so we reschedule without blocking the event loop.
type streamPollMsg struct{}

const streamPollInterval = 80 * time.Millisecond

// AuthProvider allows the TUI to read/write tokens and persist config (for /auth login, logout, refresh).
// Set via SetAuthProvider when running under the CLI; may be nil in tests or when not available.
type AuthProvider interface {
	Token() string
	RefreshToken() string
	GatewayURL() string
	SetTokens(access, refresh string)
	SetGatewayURL(url string)
	Save() error
}

// Model holds the TUI state: session, scrollback, composer input, and dimensions.
type Model struct {
	Session         *chat.Session
	AuthProvider    AuthProvider // optional; used by /auth logout, refresh
	Scrollback      []string
	Input           string
	InputHistory    []string // newest first; Up/Down cycle through
	InputHistoryIdx int      // -1 = not browsing; 0 = newest, 1 = older, ...
	Width           int
	Height          int
	Loading         bool
	Err             string

	// streaming state
	streamCancel context.CancelFunc          // cancel the active stream; nil when idle
	streamCh     <-chan chat.ChatStreamDelta // active stream channel; nil when idle
	streamBuf    strings.Builder             // accumulates in-flight visible text
	ctrlCCount   int                         // successive Ctrl+C when idle → exit

	// login form overlay (REQ-CLIENT-0190: in-session login; password not echoed)
	ShowLoginForm     bool
	LoginGatewayURL   string
	LoginUsername     string
	LoginPassword     string
	LoginFocusedField int // 0=gateway, 1=username, 2=password
	LoginErr          string

	// Startup: when token was empty, show login on init; after login ensure thread.
	OpenLoginFormOnInit  bool
	ResumeThreadSelector string
}

// NewModel returns an initial TUI model for the given session.
func NewModel(session *chat.Session) *Model {
	return &Model{
		Session:         session,
		AuthProvider:    nil,
		Scrollback:      []string{},
		Input:           "",
		InputHistoryIdx: -1,
		Width:           80,
		Height:          24,
	}
}

// SetAuthProvider sets the optional auth provider (used by /auth logout, refresh).
func (m *Model) SetAuthProvider(p AuthProvider) { m.AuthProvider = p }

// SetResumeThreadSelector sets the thread selector for --resume-thread (used after in-session login to ensure thread).
func (m *Model) SetResumeThreadSelector(s string) { m.ResumeThreadSelector = s }

// Init runs once at startup. When OpenLoginFormOnInit is true (startup token failure), opens login form.
func (m *Model) Init() tea.Cmd {
	if m.OpenLoginFormOnInit {
		return func() tea.Msg { return openLoginFormMsg{} }
	}
	return nil
}

// Update handles key events, window resize, and async send results.
//
//nolint:gocyclo // message dispatch is inherently a large switch over tea and internal msg types
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	case streamStartMsg:
		// Store the channel in the main loop (safe — no goroutine write to model fields).
		m.streamCh = msg.ch
		return m, scheduleNextDelta(m.streamCh)
	case streamDeltaMsg:
		return m.applyStreamDelta(msg)
	case streamDoneMsg:
		m.applyStreamDone(msg)
		return m, nil
	case sendResult:
		m.applySendResult(msg)
		return m, nil
	case threadListResult:
		return m.applyThreadListResult(msg)
	case threadRenameResult:
		return m.applyThreadRenameResult(msg)
	case ensureThreadResult:
		return m.applyEnsureThreadResult(msg)
	case slashResultMsg:
		return m.applySlashResult(msg)
	case shellExecDoneMsg:
		return m.applyShellExecDone(msg)
	case openLoginFormMsg:
		return m.applyOpenLoginForm()
	case loginResultMsg:
		return m.applyLoginResult(msg)
	case streamPollMsg:
		if m.streamCh != nil {
			return m, scheduleNextDelta(m.streamCh)
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.ShowLoginForm {
		return m.handleLoginFormKey(msg)
	}
	switch msg.String() {
	case "ctrl+c":
		return m.handleCtrlC()
	case "ctrl+d":
		return m, tea.Quit
	}
	m.ctrlCCount = 0
	switch msg.String() {
	case "enter":
		return m.handleEnterKey()
	case "up":
		m.navigateInputHistory(true)
		return m, nil
	case "down":
		m.navigateInputHistory(false)
		return m, nil
	case "shift+enter":
		m.Input += "\n"
		return m, nil
	case "backspace":
		if m.Input != "" {
			m.Input = m.Input[:len(m.Input)-1]
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.Input += string(msg.Runes)
		}
		return m, nil
	}
}

func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.Input)
	if m.Loading && line != "" {
		return m, nil
	}
	m.Input = ""
	if line == "" {
		return m, nil
	}
	m.Err = ""
	if strings.HasPrefix(line, "!") {
		m.Scrollback = append(m.Scrollback, "You: "+line)
		m.Loading = true
		cmd := m.handleShellEscape(line)
		return m, cmd
	}
	if strings.HasPrefix(line, "/") {
		return m.handleSlashLine(line)
	}
	m.Scrollback = append(m.Scrollback, "You: "+line)
	m.pushInputHistory(line)
	m.InputHistoryIdx = -1
	m.Loading = true
	cmd := m.streamCmd(line)
	return m, cmd
}

// handleSlashLine dispatches a slash-prefixed line to the correct handler.
func (m *Model) handleSlashLine(line string) (tea.Model, tea.Cmd) {
	m.Scrollback = append(m.Scrollback, "You: "+line)
	// Thread commands use the existing thread path.
	if strings.HasPrefix(strings.ToLower(line), "/thread") {
		if cmd := m.handleThreadCommand(line); cmd != nil {
			m.Loading = true
			return m, cmd
		}
		return m, nil
	}
	tuiCmd, handled := m.handleSlashCmd(line)
	if handled {
		m.Loading = true
		return m, tuiCmd
	}
	m.Scrollback = append(m.Scrollback, "Unknown command. Type /help for available commands.")
	return m, nil
}

// handleCtrlC implements Ctrl+C semantics per spec:
// - When a stream is in flight: cancel it (reconcile the partial turn).
// - When idle: increment counter; successive Ctrl+C exits.
func (m *Model) handleCtrlC() (tea.Model, tea.Cmd) {
	if m.Loading && m.streamCancel != nil {
		// Cancel the active stream; streamDoneMsg will reconcile.
		m.streamCancel()
		m.streamCancel = nil
		return m, nil
	}
	m.ctrlCCount++
	if m.ctrlCCount >= 2 { //nolint:mnd // two successive Ctrl+C exits per spec
		return m, tea.Quit
	}
	m.Scrollback = append(m.Scrollback, "(Press Ctrl+C again to exit)")
	return m, nil
}

// handleThreadCommand handles /thread new, list, switch, rename. Returns a tea.Cmd for async ops, or nil.
func (m *Model) handleThreadCommand(line string) tea.Cmd {
	if m.Session == nil {
		return nil
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/thread") {
		return nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "/thread"))
	parts := strings.Fields(rest)
	sub := ""
	if len(parts) > 0 {
		sub = strings.ToLower(parts[0])
	}
	switch sub {
	case "new":
		return m.threadCommandNew()
	case "list":
		return m.threadListCmd()
	case "switch":
		return m.threadCommandSwitch(parts, rest)
	case "rename":
		return m.threadCommandRename(parts, rest)
	default:
		m.threadCommandUsage(rest)
		return nil
	}
}

func (m *Model) threadCommandNew() tea.Cmd {
	threadID, err := m.Session.NewThread()
	if err != nil {
		m.Scrollback = append(m.Scrollback, "Error: "+err.Error())
		return nil
	}
	m.Scrollback = append(m.Scrollback, chat.LandmarkThreadSwitched+" New thread: "+threadID)
	return nil
}

func (m *Model) threadCommandSwitch(parts []string, rest string) tea.Cmd {
	if len(parts) < 2 {
		m.Scrollback = append(m.Scrollback, "Usage: /thread switch <selector> (use ordinal, id, or title from /thread list)")
		return nil
	}
	selector := strings.TrimSpace(strings.TrimPrefix(rest, "switch"))
	id, err := m.Session.ResolveThreadSelector(selector, 50)
	if err != nil {
		m.Scrollback = append(m.Scrollback, "Error: "+err.Error())
		return nil
	}
	m.Session.SetCurrentThreadID(id)
	m.Scrollback = append(m.Scrollback, chat.LandmarkThreadSwitched+" Switched to thread: "+id)
	return nil
}

func (m *Model) threadCommandRename(parts []string, rest string) tea.Cmd {
	if len(parts) < 2 {
		m.Scrollback = append(m.Scrollback, "Usage: /thread rename <title>")
		return nil
	}
	title := strings.TrimSpace(strings.TrimPrefix(rest, "rename"))
	title = strings.TrimSpace(title)
	if title == "" {
		m.Scrollback = append(m.Scrollback, "Usage: /thread rename <title>")
		return nil
	}
	return m.threadRenameCmd(title)
}

func (m *Model) threadCommandUsage(rest string) {
	if rest != "" {
		m.Scrollback = append(m.Scrollback, "Unknown: /thread "+rest+" (use new, list, switch, rename)")
	} else {
		m.Scrollback = append(m.Scrollback, "Thread: new, list, switch <id>, rename <title>")
	}
}

func (m *Model) threadListCmd() tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil {
			return threadListResult{err: fmt.Errorf("no session")}
		}
		items, err := m.Session.ListThreads(20, 0)
		if err != nil {
			return threadListResult{err: err}
		}
		lines := []string{"--- Threads (use ordinal, id, or title with /thread switch <selector>) ---"}
		for i, t := range items {
			title := ""
			if t.Title != nil {
				title = *t.Title
			}
			if title == "" {
				title = "(no title)"
			}
			ordinal := fmt.Sprintf("%d", i+1)
			lines = append(lines, fmt.Sprintf("  %s  %s  %s", ordinal, t.ID, title))
		}
		return threadListResult{lines: lines}
	}
}

func (m *Model) threadRenameCmd(title string) tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil {
			return threadRenameResult{err: fmt.Errorf("no session")}
		}
		err := m.Session.PatchThreadTitle("", title)
		return threadRenameResult{err: err}
	}
}

func (m *Model) pushInputHistory(line string) {
	if line == "" {
		return
	}
	// Prepend so newest is index 0; drop duplicates of last sent
	if len(m.InputHistory) > 0 && m.InputHistory[0] == line {
		return
	}
	m.InputHistory = append([]string{line}, m.InputHistory...)
	if len(m.InputHistory) > maxInputHistory {
		m.InputHistory = m.InputHistory[:maxInputHistory]
	}
}

func (m *Model) navigateInputHistory(up bool) {
	if len(m.InputHistory) == 0 {
		return
	}
	switch {
	case up:
		switch {
		case m.InputHistoryIdx < 0:
			m.InputHistoryIdx = 0
		case m.InputHistoryIdx < len(m.InputHistory)-1:
			m.InputHistoryIdx++
		default:
			return
		}
		m.Input = m.InputHistory[m.InputHistoryIdx]
	default:
		if m.InputHistoryIdx <= 0 {
			m.InputHistoryIdx = -1
			m.Input = ""
			return
		}
		m.InputHistoryIdx--
		m.Input = m.InputHistory[m.InputHistoryIdx]
	}
}

func (m *Model) applyStreamDelta(msg streamDeltaMsg) (tea.Model, tea.Cmd) {
	if msg.amendment != "" {
		m.streamBuf.Reset()
		m.streamBuf.WriteString(msg.amendment)
	} else {
		m.streamBuf.WriteString(msg.delta)
	}
	prefix := assistantPrefix
	if len(m.Scrollback) > 0 && strings.HasPrefix(m.Scrollback[len(m.Scrollback)-1], prefix) {
		m.Scrollback[len(m.Scrollback)-1] = prefix + m.streamBuf.String()
	}
	if m.streamCh != nil {
		return m, scheduleNextDelta(m.streamCh)
	}
	return m, nil
}

func (m *Model) applyThreadListResult(msg threadListResult) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.err != nil {
		m.Err = msg.err.Error()
		m.Scrollback = append(m.Scrollback, "Error: "+msg.err.Error())
	} else {
		m.Scrollback = append(m.Scrollback, msg.lines...)
	}
	return m, nil
}

func (m *Model) applyThreadRenameResult(msg threadRenameResult) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.err != nil {
		m.Err = msg.err.Error()
		m.Scrollback = append(m.Scrollback, "Error: "+msg.err.Error())
	} else {
		m.Scrollback = append(m.Scrollback, "Thread renamed.")
	}
	return m, nil
}

func (m *Model) ensureThreadCmd() tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil {
			return ensureThreadResult{err: fmt.Errorf("no session")}
		}
		err := m.Session.EnsureThread(m.ResumeThreadSelector)
		if err != nil {
			return ensureThreadResult{err: err}
		}
		return ensureThreadResult{threadID: m.Session.CurrentThreadID}
	}
}

func (m *Model) applyEnsureThreadResult(msg ensureThreadResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.Scrollback = append(m.Scrollback, "Error: "+msg.err.Error())
		return m, nil
	}
	if msg.threadID != "" {
		m.Scrollback = append(m.Scrollback, chat.LandmarkThreadSwitched+" Thread: "+msg.threadID)
	}
	return m, nil
}

func (m *Model) applySlashResult(msg slashResultMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.exitModel {
		return m, tea.Quit
	}
	if msg.lines == nil {
		m.Scrollback = []string{}
	} else {
		m.Scrollback = append(m.Scrollback, msg.lines...)
	}
	return m, nil
}

func (m *Model) applyShellExecDone(msg shellExecDoneMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.output != "" {
		m.Scrollback = append(m.Scrollback, strings.Split(strings.TrimRight(msg.output, "\n"), "\n")...)
	}
	if msg.exitCode != 0 {
		m.Scrollback = append(m.Scrollback, fmt.Sprintf("exit status %d", msg.exitCode))
	}
	return m, nil
}

func (m *Model) applyOpenLoginForm() (tea.Model, tea.Cmd) {
	m.Loading = false
	m.ShowLoginForm = true
	m.LoginErr = ""
	m.LoginUsername = ""
	m.LoginPassword = ""
	m.LoginFocusedField = 0
	switch {
	case m.Session != nil && m.Session.Client != nil && m.Session.Client.BaseURL != "":
		m.LoginGatewayURL = m.Session.Client.BaseURL
	case m.AuthProvider != nil:
		m.LoginGatewayURL = m.AuthProvider.GatewayURL()
	default:
		m.LoginGatewayURL = ""
	}
	return m, nil
}

func (m *Model) applyLoginResult(msg loginResultMsg) (tea.Model, tea.Cmd) {
	m.ShowLoginForm = false
	m.LoginPassword = ""
	m.LoginErr = ""
	if msg.Err != nil {
		m.Scrollback = append(m.Scrollback, "Login failed: "+msg.Err.Error())
		return m, nil
	}
	client := gateway.NewClient(msg.GatewayURL)
	client.SetToken(msg.AccessToken)
	if m.Session != nil {
		m.Session.SetClient(client)
	}
	if m.AuthProvider != nil {
		m.AuthProvider.SetGatewayURL(msg.GatewayURL)
		m.AuthProvider.SetTokens(msg.AccessToken, msg.RefreshToken)
		if err := m.AuthProvider.Save(); err != nil {
			m.Scrollback = append(m.Scrollback, "Logged in but config save failed: "+err.Error())
			return m, nil
		}
	}
	m.Scrollback = append(m.Scrollback, "Logged in.")
	// Ensure thread after login: new (default) or resolve --resume-thread (cynork_tui.md).
	cmd := m.ensureThreadCmd()
	return m, cmd
}

// handleLoginFormKey handles key events when the login overlay is visible (REQ-CLIENT-0190).
func (m *Model) handleLoginFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ShowLoginForm = false
		m.LoginPassword = ""
		m.LoginErr = ""
		m.Scrollback = append(m.Scrollback, "Login cancelled.")
		return m, nil
	case "tab":
		m.LoginFocusedField = (m.LoginFocusedField + 1) % 3
		return m, nil
	case "shift+tab":
		m.LoginFocusedField = (m.LoginFocusedField + 2) % 3
		return m, nil
	case "enter":
		gatewayURL := strings.TrimSpace(m.LoginGatewayURL)
		username := strings.TrimSpace(m.LoginUsername)
		password := m.LoginPassword
		if gatewayURL == "" || username == "" {
			m.LoginErr = "Gateway URL and username are required"
			return m, nil
		}
		return m, runLoginCmd(gatewayURL, username, password)
	case "backspace":
		m.loginFormBackspace()
		m.LoginErr = ""
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.loginFormAppend(string(msg.Runes))
			m.LoginErr = ""
		}
		return m, nil
	}
}

func (m *Model) loginFormBackspace() {
	switch m.LoginFocusedField {
	case 0:
		if m.LoginGatewayURL != "" {
			m.LoginGatewayURL = m.LoginGatewayURL[:len(m.LoginGatewayURL)-1]
		}
	case 1:
		if m.LoginUsername != "" {
			m.LoginUsername = m.LoginUsername[:len(m.LoginUsername)-1]
		}
	case 2:
		if m.LoginPassword != "" {
			m.LoginPassword = m.LoginPassword[:len(m.LoginPassword)-1]
		}
	}
}

func (m *Model) loginFormAppend(s string) {
	switch m.LoginFocusedField {
	case 0:
		m.LoginGatewayURL += s
	case 1:
		m.LoginUsername += s
	case 2:
		m.LoginPassword += s
	}
}

// runLoginCmd performs POST /v1/auth/login and returns loginResultMsg (blocks briefly).
func runLoginCmd(gatewayURL, username, password string) tea.Cmd {
	return func() tea.Msg {
		client := gateway.NewClient(gatewayURL)
		resp, err := client.Login(userapi.LoginRequest{Handle: username, Password: password})
		if err != nil {
			return loginResultMsg{GatewayURL: gatewayURL, Err: err}
		}
		return loginResultMsg{
			GatewayURL:   gatewayURL,
			AccessToken:  resp.AccessToken,
			RefreshToken: resp.RefreshToken,
		}
	}
}

func (m *Model) applySendResult(msg sendResult) {
	m.Loading = false
	if msg.err != nil {
		m.Err = msg.err.Error()
		m.Scrollback = append(m.Scrollback, "Error: "+m.Err)
	} else if msg.visible != "" {
		m.Scrollback = append(m.Scrollback, assistantPrefix+msg.visible)
	}
}

func (m *Model) applyStreamDone(msg streamDoneMsg) {
	m.Loading = false
	m.streamCancel = nil
	m.streamCh = nil
	final := strings.TrimSpace(m.streamBuf.String())
	m.streamBuf.Reset()
	if msg.err != nil && final == "" {
		// Stream failed with no partial content.
		m.Err = msg.err.Error()
		// Replace the placeholder line with an error line.
		prefix := assistantPrefix
		if len(m.Scrollback) > 0 && strings.HasPrefix(m.Scrollback[len(m.Scrollback)-1], prefix) {
			m.Scrollback[len(m.Scrollback)-1] = "Error: " + m.Err
		} else {
			m.Scrollback = append(m.Scrollback, "Error: "+m.Err)
		}
		return
	}
	// Reconcile the final content into the scrollback line.
	prefix := assistantPrefix
	if len(m.Scrollback) > 0 && strings.HasPrefix(m.Scrollback[len(m.Scrollback)-1], prefix) {
		if final == "" {
			// Empty response — keep the line as "(no response)".
			m.Scrollback[len(m.Scrollback)-1] = prefix + "(no response)"
		} else {
			m.Scrollback[len(m.Scrollback)-1] = prefix + final
		}
	}
	if msg.err != nil {
		// Partial content received but stream ended with error (e.g. cancel).
		m.Scrollback = append(m.Scrollback, "(stream interrupted)")
	}
}

// streamCmd starts a streaming send. It seeds the scrollback with a placeholder "Assistant: "
// line that will be updated in-place as deltas arrive.
// Per REQ-CLIENT-0209: the TUI requests streaming by default for interactive turns.
func (m *Model) streamCmd(line string) tea.Cmd {
	if m.Session == nil {
		return func() tea.Msg { return sendResult{err: fmt.Errorf("no session")} }
	}
	// Seed the in-flight line.
	m.Scrollback = append(m.Scrollback, assistantPrefix)
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	m.streamBuf.Reset()
	session := m.Session
	return func() tea.Msg {
		ch, err := session.StreamMessage(ctx, line)
		if err != nil {
			cancel()
			return streamDoneMsg{err: err}
		}
		// Return streamStartMsg so the model stores ch in the main Update loop
		// (avoids writing model fields from a goroutine — data race prevention).
		return streamStartMsg{ch: ch}
	}
}

// readNextDelta returns the next delta, amendment, or streamPollMsg if nothing is ready.
func readNextDelta(ch <-chan chat.ChatStreamDelta) tea.Msg {
	select {
	case delta, ok := <-ch:
		if !ok {
			return streamDoneMsg{}
		}
		if delta.Done {
			return streamDoneMsg{responseID: delta.ResponseID, err: delta.Err}
		}
		if delta.Amendment != "" {
			return streamDeltaMsg{amendment: delta.Amendment}
		}
		return streamDeltaMsg{delta: delta.Delta}
	case <-time.After(streamPollInterval):
		return streamPollMsg{}
	}
}

// scheduleNextDelta returns a tea.Cmd that will read the next item from ch (with timeout).
func scheduleNextDelta(ch <-chan chat.ChatStreamDelta) tea.Cmd {
	return func() tea.Msg {
		return readNextDelta(ch)
	}
}

// View renders the TUI: scrollback, composer, status bar (with landmarks).
func (m *Model) View() string {
	style := lipgloss.NewStyle()
	statusStyle := lipgloss.NewStyle().Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	// Composer (multi-line: Enter send, Shift+Enter newline); cap visible lines so view fits height
	const maxComposerLines = 5
	composerContent := "> " + strings.ReplaceAll(m.Input, "\n", "\n  ")
	composerLines := strings.Split(composerContent, "\n")
	if len(composerLines) > maxComposerLines {
		composerLines = composerLines[len(composerLines)-maxComposerLines:]
	}
	composer := strings.Join(composerLines, "\n")

	// Scrollback area (top). When empty, show scrollback hint and short landmark for E2E.
	sb := strings.Join(m.Scrollback, "\n")
	if sb == "" {
		sb = " (scrollback) " + chat.LandmarkPromptReadyShort
	}
	scrollbackHeight := m.Height - len(composerLines) - 1 - 1 // composer + status
	if scrollbackHeight < 1 {
		scrollbackHeight = 1
	}
	scrollbackBox := style.Height(scrollbackHeight).Width(m.Width).Render(sb)

	// Status bar: landmark when prompt ready or in-flight; gateway hint
	landmark := chat.LandmarkPromptReady
	if m.Loading {
		landmark = chat.LandmarkAssistantInFlight
	}
	gatewayURL := "-"
	projectID := defaultPlaceholder
	model := defaultPlaceholder
	if m.Session != nil {
		if m.Session.Client != nil {
			gatewayURL = m.Session.Client.BaseURL
		}
		projectID = orEmpty(m.Session.ProjectID)
		model = orEmpty(m.Session.Model)
	}
	thread := defaultPlaceholder
	if m.Session != nil && m.Session.CurrentThreadID != "" {
		thread = m.Session.CurrentThreadID
		if len(thread) > 8 {
			thread = thread[:8] + "…"
		}
	}
	status := fmt.Sprintf("%s | gateway: %s | project: %s | model: %s | thread: %s | %s",
		landmark, gatewayURL, projectID, model, thread, composerHint)
	statusBar := statusStyle.Width(m.Width).Render(status)

	var errLine string
	if m.Err != "" {
		errLine = errStyle.Render(" " + m.Err)
	}

	mainView := scrollbackBox + "\n" + composer + "\n" + statusBar + errLine
	if m.ShowLoginForm {
		mainView = m.renderLoginOverlay(mainView)
	}
	return mainView
}

// renderLoginOverlay draws the login box over the main view (password not echoed; REQ-CLIENT-0190).
func (m *Model) renderLoginOverlay(mainView string) string {
	const loginBoxWidth = 56
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(loginBoxWidth)
	labelStyle := lipgloss.NewStyle().Width(12)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	gatewayLabel := labelStyle.Render("Gateway URL:")
	userLabel := labelStyle.Render("Username:")
	passLabel := labelStyle.Render("Password:")

	gatewayVal := m.LoginGatewayURL
	if gatewayVal == "" {
		gatewayVal = " "
	}
	userVal := m.LoginUsername
	if userVal == "" {
		userVal = " "
	}
	passVal := strings.Repeat("*", len(m.LoginPassword))
	if passVal == "" {
		passVal = " "
	}

	cur := " "
	f0, f1, f2 := cur, cur, cur
	switch m.LoginFocusedField {
	case 0:
		f0 = loginFormCursor
	case 1:
		f1 = loginFormCursor
	case 2:
		f2 = loginFormCursor
	}

	// LandmarkAuthRecoveryReady so PTY/E2E can wait for the login form (tui_pty_harness).
	lines := []string{
		" Login " + chat.LandmarkAuthRecoveryReady,
		"",
		gatewayLabel + gatewayVal + f0,
		userLabel + userVal + f1,
		passLabel + passVal + f2,
		"",
		" [Enter] Login   [Esc] Cancel",
	}
	if m.LoginErr != "" {
		lines = append(lines, "", errStyle.Render(m.LoginErr))
	}
	content := strings.Join(lines, "\n")
	box := boxStyle.Render(content)
	boxLines := strings.Split(box, "\n")
	mainLines := strings.Split(mainView, "\n")
	if len(mainLines) < len(boxLines) {
		return mainView + "\n" + box
	}
	startRow := (len(mainLines) - len(boxLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	padStyle := lipgloss.NewStyle().Width(m.Width)
	for i, line := range boxLines {
		idx := startRow + i
		if idx < len(mainLines) {
			mainLines[idx] = padStyle.Render(line)
		}
	}
	return strings.Join(mainLines, "\n")
}

func orEmpty(s string) string {
	if s == "" {
		return defaultPlaceholder
	}
	return s
}
