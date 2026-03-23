// Package tui provides the full-screen TUI for cynork. See docs/tech_specs/cynork_tui.md.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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

// ctrlCExitThreshold is the number of successive Ctrl+C presses (when idle) required to exit the TUI.
const ctrlCExitThreshold = 2

// proactiveTokenRefreshInterval is how often we try to refresh the access token while the TUI is active.
const proactiveTokenRefreshInterval = 8 * time.Minute

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

// copyClipboardResultMsg is sent after CopyToClipboard finishes (Ctrl+Y or /copy).
type copyClipboardResultMsg struct {
	err error
	// successDetail is shown in ClipNote and as a system scrollback line on success (required for /copy).
	successDetail string
}

// clipNoteClearMsg clears the ephemeral clipboard status line.
type clipNoteClearMsg struct{}

// proactiveTokenRefreshMsg triggers a background token refresh when a refresh token is available.
type proactiveTokenRefreshMsg struct{}

// tokenRefreshResultMsg is sent after POST /v1/auth/refresh completes (proactive or manual).
type tokenRefreshResultMsg struct {
	resp *userapi.LoginResponse
	err  error
}

const streamPollInterval = 80 * time.Millisecond
const clipNoteDuration = 3 * time.Second

// AuthProvider allows the TUI to read/write tokens and persist config (for /auth login, logout, refresh).
// Set via SetAuthProvider when running under the CLI; may be nil in tests or when not available.
type AuthProvider interface {
	Token() string
	RefreshToken() string
	GatewayURL() string
	SetTokens(access, refresh string)
	// SetGatewayURL updates the in-memory gateway base URL. When userExplicit is true
	// (e.g. /connect <url>), the new URL is persisted to config and overrides
	// CYNORK_GATEWAY_URL merge behavior; when false (e.g. in-TUI login), file-backed
	// gateway_url is preserved when the session used an env override.
	SetGatewayURL(url string, userExplicit bool)
	Save() error
	ShowThinkingByDefault() bool
	SetShowThinkingByDefault(bool)
	ShowToolOutputByDefault() bool
	SetShowToolOutputByDefault(bool)
}

// Model holds the TUI state: session, scrollback, composer input, and dimensions.
type Model struct {
	Session      *chat.Session
	AuthProvider AuthProvider // optional; used by /auth logout, refresh
	Scrollback   []string
	Input        string
	// inputCursor is the byte offset of the insertion caret in Input (UTF-8 boundary).
	inputCursor     int
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

	// ShowThinking controls whether thinking parts are expanded (true) or collapsed (false).
	ShowThinking bool
	// ShowToolOutput controls whether tool-call and tool-result parts are expanded (true) or collapsed (false).
	ShowToolOutput bool

	// ScrollVP is the scrollback viewport (mouse wheel, PgUp/PgDn per cynork_tui.md).
	ScrollVP viewport.Model
	// mdRendererCached is reused when width/color options match (glamour is expensive to construct).
	mdRendererCached   *glamour.TermRenderer
	mdRendererCacheKey string

	// Cached ANSI scrollback from renderScrollbackContent (glamour). Invalidated when
	// scrollbackRenderSignature changes — avoids re-running glamour on every keystroke in the composer.
	scrollbackRendered   string
	scrollbackCacheSig   uint64
	scrollbackCacheValid bool

	// ClipNote is a short-lived status after clipboard copy (cleared after clipNoteDuration).
	ClipNote string

	// Slash command popup (filtered catalog, Up/Down navigate, Tab completes).
	slashMenuSel    int
	slashMenuScroll int

	// proactiveTokenRefreshStarted is true after we schedule tea.Every for token refresh.
	proactiveTokenRefreshStarted bool

	// healthPollIntervalSec is seconds between GET /healthz polls (0 = disabled). Set by CLI from config.
	healthPollIntervalSec    int
	gatewayHealth            gatewayHealthState
	gatewayHealthPollStarted bool
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
		ScrollVP:        newTUIViewport(80, 12),
	}
}

// SetAuthProvider sets the optional auth provider (used by /auth logout, refresh).
// When p is non-nil, syncs ShowThinking and ShowToolOutput from the provider's default preferences.
func (m *Model) SetAuthProvider(p AuthProvider) {
	m.AuthProvider = p
	if p != nil {
		m.ShowThinking = p.ShowThinkingByDefault()
		m.ShowToolOutput = p.ShowToolOutputByDefault()
	}
}

// SetResumeThreadSelector sets the thread selector for --resume-thread (used after in-session login to ensure thread).
func (m *Model) SetResumeThreadSelector(s string) { m.ResumeThreadSelector = s }

// SetHealthPollInterval sets the interval in seconds for gateway /healthz polling (0 disables). Called from the CLI.
func (m *Model) SetHealthPollInterval(seconds int) { m.healthPollIntervalSec = seconds }

// Init runs once at startup. When OpenLoginFormOnInit is true (startup token failure), opens login form.
// When a token is already present, ensures thread (new or --resume-thread) asynchronously so the
// TUI surface appears before gateway I/O (same as post-login ensureThreadCmd).
func (m *Model) Init() tea.Cmd {
	if m.OpenLoginFormOnInit {
		return func() tea.Msg { return openLoginFormMsg{} }
	}
	if m.Session != nil && m.Session.Client != nil && m.Session.Client.Token != "" {
		return m.ensureThreadCmd()
	}
	return nil
}

// Update handles key events, window resize, and async send results.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.ShowLoginForm {
			return m, nil
		}
		var cmd tea.Cmd
		m.ScrollVP, cmd = m.ScrollVP.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if !m.ShowLoginForm && m.isViewportScrollKey(msg) {
			var cmd tea.Cmd
			m.ScrollVP, cmd = m.ScrollVP.Update(msg)
			return m, cmd
		}
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, m.maybeStartGatewayHealthPollOnce()
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
		return m.applyStreamPoll()
	case copyClipboardResultMsg:
		return m.applyCopyClipboardResult(msg)
	case clipNoteClearMsg:
		m.ClipNote = ""
		return m, nil
	case proactiveTokenRefreshMsg:
		return m.handleProactiveTokenRefresh()
	case tokenRefreshResultMsg:
		return m.applyTokenRefreshResult(msg)
	case gatewayHealthPollMsg:
		return m.handleGatewayHealthPoll()
	case gatewayHealthResultMsg:
		return m.applyGatewayHealthResult(msg)
	default:
		return m, nil
	}
}

func (m *Model) applyCopyClipboardResult(msg copyClipboardResultMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	m.scrollbackCacheValid = false
	if msg.err != nil {
		m.ClipNote = "Copy failed: " + msg.err.Error()
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Copy failed: "+msg.err.Error())
	} else {
		line := msg.successDetail
		if line == "" {
			line = "Copied to clipboard."
		}
		m.ClipNote = line
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+line)
	}
	return m, m.scheduleClipNoteClear()
}

func (m *Model) scheduleClipNoteClear() tea.Cmd {
	return tea.Tick(clipNoteDuration, func(time.Time) tea.Msg { return clipNoteClearMsg{} })
}

func (m *Model) cmdCopyLastAssistant() tea.Cmd {
	text := lastAssistantPlain(m.Scrollback)
	if strings.TrimSpace(text) == "" {
		return func() tea.Msg {
			return copyClipboardResultMsg{
				err:           nil,
				successDetail: "No assistant message to copy.",
			}
		}
	}
	return tea.Sequence(
		func() tea.Msg {
			return copyClipboardResultMsg{err: nil, successDetail: "Last message copied to clipboard."}
		},
		func() tea.Msg {
			if err := CopyToClipboard(text); err != nil {
				return copyClipboardResultMsg{err: err, successDetail: ""}
			}
			return nil
		},
	)
}

func (m *Model) applyStreamPoll() (tea.Model, tea.Cmd) {
	if m.streamCh == nil {
		return m, nil
	}
	return m, scheduleNextDelta(m.streamCh)
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.ShowLoginForm {
		return m.handleLoginFormKey(msg)
	}
	switch msg.String() {
	case "ctrl+y":
		return m, m.cmdCopyLastAssistant()
	case "ctrl+c":
		return m.handleCtrlC()
	case "ctrl+d":
		return m, tea.Quit
	}
	m.ctrlCCount = 0
	switch msg.String() {
	// Multiline: insert newline without sending. Most terminals send the same bytes for
	// Shift+Enter as for Enter, so Bubble Tea reports both as "enter" — use alt+enter or ctrl+j.
	case "shift+enter", "alt+enter", "ctrl+j":
		m.insertAtCursor("\n")
		m.clampSlashMenuSelection()
		return m, nil
	case "enter":
		return m.handleEnterKey()
	case "up":
		if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
			m.navSlashMenu(true)
			return m, nil
		}
		m.navigateInputHistory(true)
		return m, nil
	case "down":
		if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
			m.navSlashMenu(false)
			return m, nil
		}
		m.navigateInputHistory(false)
		return m, nil
	case "tab":
		if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
			m.applySlashCompletion()
			m.clampSlashMenuSelection()
			return m, nil
		}
		return m, nil
	case "esc":
		if m.slashMenuVisible() {
			m.replaceActiveComposerLine("")
			m.slashMenuSel = 0
			m.slashMenuScroll = 0
			return m, nil
		}
		return m, nil
	case "left":
		m.moveInputCursorRune(-1)
		m.clampSlashMenuSelection()
		return m, nil
	case "right":
		m.moveInputCursorRune(1)
		m.clampSlashMenuSelection()
		return m, nil
	case "ctrl+left":
		m.moveInputCursorWordLeft()
		m.clampSlashMenuSelection()
		return m, nil
	case "ctrl+right":
		m.moveInputCursorWordRight()
		m.clampSlashMenuSelection()
		return m, nil
	case "backspace":
		m.deleteRuneBeforeCursor()
		m.clampSlashMenuSelection()
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			if len(msg.Runes) == 1 && msg.Runes[0] == '\n' {
				m.insertAtCursor("\n")
				m.clampSlashMenuSelection()
				return m, nil
			}
			m.insertAtCursor(string(msg.Runes))
			m.clampSlashMenuSelection()
		}
		return m, nil
	}
}

func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.Input)
	if m.Loading && line != "" {
		return m, nil
	}
	if m.slashMenuVisible() {
		filtered := m.filteredSlashCommands()
		active := strings.TrimSpace(activeComposerLine(m.Input))
		if len(filtered) > 0 {
			matched := false
			for _, e := range filtered {
				if active == e.name || strings.HasPrefix(active, e.name+" ") {
					matched = true
					break
				}
			}
			if !matched {
				m.applySlashCompletion()
				m.clampSlashMenuSelection()
				return m, nil
			}
		}
	}
	m.Input = ""
	m.inputCursor = 0
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
// Slash input is not echoed as a "You:" chat line; only command output appears (as system lines).
func (m *Model) handleSlashLine(line string) (tea.Model, tea.Cmd) {
	cmdName, rest := parseSlashTUI(line)
	if cmdName == "copy" {
		return m, slashCopyCmd(m, rest)
	}
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
	m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Unknown command. Type /help for available commands.")
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
	if m.ctrlCCount >= ctrlCExitThreshold {
		return m, tea.Quit
	}
	m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"(Press Ctrl+C again to exit)")
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
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Error: "+err.Error())
		return nil
	}
	m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+chat.LandmarkThreadSwitched+" New thread: "+threadID)
	return nil
}

func (m *Model) threadCommandSwitch(parts []string, rest string) tea.Cmd {
	if len(parts) < 2 {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Usage: /thread switch <selector> (use ordinal, id, or title from /thread list)")
		return nil
	}
	selector := strings.TrimSpace(strings.TrimPrefix(rest, "switch"))
	id, err := m.Session.ResolveThreadSelector(selector, 50)
	if err != nil {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Error: "+err.Error())
		return nil
	}
	m.Session.SetCurrentThreadID(id)
	m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+chat.LandmarkThreadSwitched+" Switched to thread: "+id)
	return nil
}

func (m *Model) threadCommandRename(parts []string, rest string) tea.Cmd {
	if len(parts) < 2 {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Usage: /thread rename <title>")
		return nil
	}
	title := strings.TrimSpace(strings.TrimPrefix(rest, "rename"))
	title = strings.TrimSpace(title)
	if title == "" {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Usage: /thread rename <title>")
		return nil
	}
	return m.threadRenameCmd(title)
}

func (m *Model) threadCommandUsage(rest string) {
	if rest != "" {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Unknown: /thread "+rest+" (use new, list, switch, rename)")
	} else {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Thread: new, list, switch <id>, rename <title>")
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
		m.syncInputCursorEnd()
	default:
		if m.InputHistoryIdx <= 0 {
			m.InputHistoryIdx = -1
			m.Input = ""
			m.inputCursor = 0
			return
		}
		m.InputHistoryIdx--
		m.Input = m.InputHistory[m.InputHistoryIdx]
		m.syncInputCursorEnd()
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
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Error: "+msg.err.Error())
	} else {
		m.Scrollback = append(m.Scrollback, wrapSystemScrollbackLines(msg.lines)...)
	}
	return m, nil
}

func (m *Model) applyThreadRenameResult(msg threadRenameResult) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.err != nil {
		m.Err = msg.err.Error()
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Error: "+msg.err.Error())
	} else {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Thread renamed.")
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
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Error: "+msg.err.Error())
	} else if msg.threadID != "" {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+chat.LandmarkThreadSwitched+" Thread: "+msg.threadID)
	}
	_, tok := m.maybeStartProactiveTokenRefresh()
	return m, combineTeaCmds(m.maybeStartGatewayHealthPollOnce(), tok)
}

func (m *Model) maybeStartProactiveTokenRefresh() (tea.Model, tea.Cmd) {
	if m.proactiveTokenRefreshStarted {
		return m, nil
	}
	if m.AuthProvider == nil || m.AuthProvider.RefreshToken() == "" {
		return m, nil
	}
	m.proactiveTokenRefreshStarted = true
	return m, tea.Every(proactiveTokenRefreshInterval, func(time.Time) tea.Msg {
		return proactiveTokenRefreshMsg{}
	})
}

func (m *Model) handleProactiveTokenRefresh() (tea.Model, tea.Cmd) {
	if m.ShowLoginForm || m.Loading {
		return m, nil
	}
	cmd := m.tokenRefreshCmd()
	if cmd == nil {
		return m, nil
	}
	return m, cmd
}

func (m *Model) tokenRefreshCmd() tea.Cmd {
	rt := ""
	if m.AuthProvider != nil {
		rt = m.AuthProvider.RefreshToken()
	}
	if rt == "" || m.Session == nil || m.Session.Client == nil {
		return nil
	}
	return func() tea.Msg {
		resp, err := m.Session.Client.Refresh(rt)
		if err != nil {
			return tokenRefreshResultMsg{err: err}
		}
		return tokenRefreshResultMsg{resp: resp}
	}
}

func (m *Model) applyTokenRefreshResult(msg tokenRefreshResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil || msg.resp == nil {
		return m, nil
	}
	newR := msg.resp.RefreshToken
	if newR == "" && m.AuthProvider != nil {
		newR = m.AuthProvider.RefreshToken()
	}
	if m.AuthProvider != nil {
		m.AuthProvider.SetTokens(msg.resp.AccessToken, newR)
	}
	if m.Session != nil {
		m.Session.SetToken(msg.resp.AccessToken)
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
		m.Scrollback = append(m.Scrollback, wrapSystemScrollbackLines(msg.lines)...)
	}
	return m, nil
}

func (m *Model) applyShellExecDone(msg shellExecDoneMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.output != "" {
		m.Scrollback = append(m.Scrollback, wrapSystemScrollbackLines(strings.Split(strings.TrimRight(msg.output, "\n"), "\n"))...)
	}
	if msg.exitCode != 0 {
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+fmt.Sprintf("exit status %d", msg.exitCode))
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
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Login failed: "+msg.Err.Error())
		return m, nil
	}
	client := gateway.NewClient(msg.GatewayURL)
	client.SetToken(msg.AccessToken)
	if m.Session != nil {
		m.Session.SetClient(client)
	}
	if m.AuthProvider != nil {
		m.AuthProvider.SetGatewayURL(msg.GatewayURL, false)
		m.AuthProvider.SetTokens(msg.AccessToken, msg.RefreshToken)
		if err := m.AuthProvider.Save(); err != nil {
			m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Logged in but config save failed: "+err.Error())
			return m, nil
		}
	}
	m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Logged in.")
	// Ensure thread after login: new (default) or resolve --resume-thread (cynork_tui.md).
	return m, combineTeaCmds(m.ensureThreadCmd(), m.maybeStartGatewayHealthPollOnce())
}

// handleLoginFormKey handles key events when the login overlay is visible (REQ-CLIENT-0190).
func (m *Model) handleLoginFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ShowLoginForm = false
		m.LoginPassword = ""
		m.LoginErr = ""
		m.Scrollback = append(m.Scrollback, scrollbackSystemLinePrefix+"Login cancelled.")
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
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	// Composer (Enter send; Alt+Enter or Ctrl+J newline — Shift+Enter is Enter on most terminals); cap visible lines
	const maxComposerLines = 5
	m.clampInputCursor()
	composerLines := m.buildComposerDisplayLines(maxComposerLines)
	composerBox := m.renderComposerBox(composerLines)
	composerVisualH := lipgloss.Height(composerBox)
	if composerVisualH < 1 {
		composerVisualH = 1
	}

	slashMenuBlock := ""
	slashMenuH := 0
	if m.slashMenuVisible() {
		slashMenuBlock = m.renderSlashMenuBlock() + "\n"
		slashMenuH = lipgloss.Height(strings.TrimSuffix(slashMenuBlock, "\n"))
		if slashMenuH < 1 {
			slashMenuH = 1
		}
	}

	copyHintBlock := m.renderCopyHintLine() + "\n"
	copyHintH := lipgloss.Height(copyHintBlock)
	if copyHintH < 1 {
		copyHintH = 1
	}

	errLines := 0
	if m.Err != "" {
		errLines = 1
	}
	clipNoteH := 0
	clipNoteBlock := ""
	if m.ClipNote != "" {
		clipNoteBlock = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Width(m.Width).Render(m.ClipNote) + "\n"
		clipNoteH = lipgloss.Height(clipNoteBlock)
		if clipNoteH < 1 {
			clipNoteH = 1
		}
	}
	const statusLines = 1
	scrollbackH := m.Height - composerVisualH - slashMenuH - copyHintH - statusLines - errLines - clipNoteH
	m.ensureScrollViewport(scrollbackH)

	sig := m.scrollbackRenderSignature()
	if !m.scrollbackCacheValid || m.scrollbackCacheSig != sig {
		m.scrollbackRendered = m.renderScrollbackContent()
		m.scrollbackCacheSig = sig
		m.scrollbackCacheValid = true
	}
	scrollbackText := m.scrollbackRendered
	prevLines := m.ScrollVP.TotalLineCount()
	wasAtBottom := m.ScrollVP.AtBottom()
	m.ScrollVP.SetContent(scrollbackText)
	if wasAtBottom && m.ScrollVP.TotalLineCount() > prevLines {
		m.ScrollVP.GotoBottom()
	}
	scrollbackView := m.ScrollVP.View()

	// Status bar: gateway health (or busy) glyph, project, model, thread, composer hint.
	projectID := defaultPlaceholder
	modelName := defaultPlaceholder
	if m.Session != nil {
		projectID = orEmpty(m.Session.ProjectID)
		modelName = orEmpty(m.Session.Model)
	}
	thread := defaultPlaceholder
	if m.Session != nil && m.Session.CurrentThreadID != "" {
		thread = m.Session.CurrentThreadID
		if len(thread) > 8 {
			thread = thread[:8] + "…"
		}
	}
	tail := fmt.Sprintf(" | project: %s | model: %s | thread: %s | %s",
		projectID, modelName, thread, composerHint)
	tailStyled := lipgloss.NewStyle().Bold(true).Render(tail)
	statusLine := " " + m.renderGatewayStatusIndicator() + tailStyled
	statusBar := lipgloss.NewStyle().Width(m.Width).Render(statusLine)

	var errLine string
	if m.Err != "" {
		errLine = errStyle.Render(" "+m.Err) + "\n"
	}

	mainView := scrollbackView + "\n" + composerBox + "\n" + slashMenuBlock + copyHintBlock + clipNoteBlock + statusBar + errLine
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

// lastAssistantPlain returns the raw text of the most recent assistant scrollback line (no "Assistant: " prefix).
func lastAssistantPlain(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], assistantPrefix) {
			return strings.TrimPrefix(lines[i], assistantPrefix)
		}
	}
	return ""
}

// plainTranscript joins scrollback lines for /copy all (plain text).
// Lines marked as system feedback (slash/thread/shell) are omitted so the transcript is chat turns only.
func plainTranscript(lines []string) string {
	var b strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, scrollbackSystemLinePrefix) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}
