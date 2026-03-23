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

// Login overlay: label column width (longest label is "Gateway URL:").
const (
	loginLabelColWidth = 14
	loginBoxMaxInnerW  = 62
	loginBoxMinInnerW  = 32
)

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
	InputHistory    []string // newest first; Ctrl+Up/Ctrl+Down cycle through
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
	ShowLoginForm   bool
	LoginGatewayURL string
	LoginUsername   string
	LoginPassword   string
	// Login*Cursor are byte offsets (UTF-8 boundaries) in each field, like inputCursor in the composer.
	LoginGatewayCursor  int
	LoginUsernameCursor int
	LoginPasswordCursor int
	LoginFocusedField   int // 0=gateway, 1=username, 2=password
	LoginErr            string

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

	// Slash command popup (filtered catalog, Up/Down navigate menu, Tab completes).
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
		cmd := m.maybeStartGatewayHealthPollOnce()
		return m, cmd
	default:
		return m.dispatchAsyncMsg(msg)
	}
}

func (m *Model) dispatchAsyncMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mm, cmd, ok := m.applyStreamMsgs(msg); ok {
		return mm, cmd
	}
	if mm, cmd, ok := m.applyThreadMsgs(msg); ok {
		return mm, cmd
	}
	if mm, cmd, ok := m.applySlashShellLoginMsgs(msg); ok {
		return mm, cmd
	}
	if mm, cmd, ok := m.applyClipboardMsgs(msg); ok {
		return mm, cmd
	}
	if mm, cmd, ok := m.applyTokenAndGatewayMsgs(msg); ok {
		return mm, cmd
	}
	return m, nil
}

func (m *Model) applyStreamMsgs(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case streamStartMsg:
		m.streamCh = msg.ch
		return m, scheduleNextDelta(m.streamCh), true
	case streamDeltaMsg:
		mm, cmd := m.applyStreamDelta(msg)
		return mm, cmd, true
	case streamDoneMsg:
		m.applyStreamDone(msg)
		return m, nil, true
	case sendResult:
		m.applySendResult(msg)
		return m, nil, true
	case streamPollMsg:
		mm, cmd := m.applyStreamPoll()
		return mm, cmd, true
	default:
		return m, nil, false
	}
}

func (m *Model) applyThreadMsgs(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case threadListResult:
		mm, cmd := m.applyThreadListResult(msg)
		return mm, cmd, true
	case threadRenameResult:
		mm, cmd := m.applyThreadRenameResult(msg)
		return mm, cmd, true
	case ensureThreadResult:
		mm, cmd := m.applyEnsureThreadResult(msg)
		return mm, cmd, true
	default:
		return m, nil, false
	}
}

func (m *Model) applySlashShellLoginMsgs(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case slashResultMsg:
		mm, cmd := m.applySlashResult(msg)
		return mm, cmd, true
	case shellExecDoneMsg:
		mm, cmd := m.applyShellExecDone(msg)
		return mm, cmd, true
	case openLoginFormMsg:
		mm, cmd := m.applyOpenLoginForm()
		return mm, cmd, true
	case loginResultMsg:
		mm, cmd := m.applyLoginResult(msg)
		return mm, cmd, true
	default:
		return m, nil, false
	}
}

func (m *Model) applyClipboardMsgs(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case copyClipboardResultMsg:
		mm, cmd := m.applyCopyClipboardResult(msg)
		return mm, cmd, true
	case clipNoteClearMsg:
		m.ClipNote = ""
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m *Model) applyTokenAndGatewayMsgs(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case proactiveTokenRefreshMsg:
		mm, cmd := m.handleProactiveTokenRefresh()
		return mm, cmd, true
	case tokenRefreshResultMsg:
		mm, cmd := m.applyTokenRefreshResult(msg)
		return mm, cmd, true
	case gatewayHealthPollMsg:
		mm, cmd := m.handleGatewayHealthPoll()
		return mm, cmd, true
	case gatewayHealthResultMsg:
		mm, cmd := m.applyGatewayHealthResult(msg)
		return mm, cmd, true
	default:
		return m, nil, false
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
	cmd := m.scheduleClipNoteClear()
	return m, cmd
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
		cmd := m.cmdCopyLastAssistant()
		return m, cmd
	case "ctrl+c":
		return m.handleCtrlC()
	case "ctrl+d":
		return m, tea.Quit
	}
	m.ctrlCCount = 0
	return m.handleComposerAfterGlobalChords(msg)
}

func (m *Model) handleComposerAfterGlobalChords(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "shift+enter", "alt+enter", "ctrl+j":
		m.insertAtCursor("\n")
		m.clampSlashMenuSelection()
		return m, nil
	case "enter":
		return m.handleEnterKey()
	case "up":
		return m.handleComposerUpKey()
	case "down":
		return m.handleComposerDownKey()
	case "ctrl+up":
		return m.handleComposerCtrlUpKey()
	case "ctrl+down":
		return m.handleComposerCtrlDownKey()
	case "tab":
		return m.handleComposerTabKey()
	case "esc":
		return m.handleComposerEscKey()
	case "left":
		return m.handleComposerMoveRuneKey(-1)
	case "right":
		return m.handleComposerMoveRuneKey(1)
	case "ctrl+left":
		return m.handleComposerWordKey(m.moveInputCursorWordLeft)
	case "ctrl+right":
		return m.handleComposerWordKey(m.moveInputCursorWordRight)
	case "backspace":
		m.deleteRuneBeforeCursor()
		m.clampSlashMenuSelection()
		return m, nil
	default:
		return m.handleComposerRuneInsert(msg)
	}
}

func (m *Model) handleComposerUpKey() (tea.Model, tea.Cmd) {
	if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
		m.navSlashMenu(true)
		return m, nil
	}
	m.moveInputCursorVertical(-1)
	m.clampSlashMenuSelection()
	return m, nil
}

func (m *Model) handleComposerDownKey() (tea.Model, tea.Cmd) {
	if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
		m.navSlashMenu(false)
		return m, nil
	}
	m.moveInputCursorVertical(1)
	m.clampSlashMenuSelection()
	return m, nil
}

func (m *Model) handleComposerCtrlUpKey() (tea.Model, tea.Cmd) {
	if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
		return m, nil
	}
	m.navigateInputHistory(true)
	return m, nil
}

func (m *Model) handleComposerCtrlDownKey() (tea.Model, tea.Cmd) {
	if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
		return m, nil
	}
	m.navigateInputHistory(false)
	return m, nil
}

func (m *Model) handleComposerTabKey() (tea.Model, tea.Cmd) {
	if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
		m.applySlashCompletion()
		m.clampSlashMenuSelection()
		return m, nil
	}
	return m, nil
}

func (m *Model) handleComposerEscKey() (tea.Model, tea.Cmd) {
	if m.slashMenuVisible() {
		m.replaceActiveComposerLine("")
		m.slashMenuSel = 0
		m.slashMenuScroll = 0
		return m, nil
	}
	return m, nil
}

func (m *Model) handleComposerMoveRuneKey(dir int) (tea.Model, tea.Cmd) {
	m.moveInputCursorRune(dir)
	m.clampSlashMenuSelection()
	return m, nil
}

func (m *Model) handleComposerWordKey(move func()) (tea.Model, tea.Cmd) {
	move()
	m.clampSlashMenuSelection()
	return m, nil
}

func (m *Model) handleComposerRuneInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.Input)
	if m.Loading && line != "" {
		return m, nil
	}
	if m.maybeApplySlashMenuEnterCompletion() {
		return m, nil
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

// maybeApplySlashMenuEnterCompletion applies completion when the menu is open and input does not
// yet match a listed command. Returns true if Enter was consumed without sending.
func (m *Model) maybeApplySlashMenuEnterCompletion() bool {
	if !m.slashMenuVisible() {
		return false
	}
	filtered := m.filteredSlashCommands()
	active := strings.TrimSpace(activeComposerLine(m.Input))
	if len(filtered) == 0 {
		return false
	}
	for _, e := range filtered {
		if active == e.name || strings.HasPrefix(active, e.name+" ") {
			return false
		}
	}
	m.applySlashCompletion()
	m.clampSlashMenuSelection()
	return true
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
	m.syncLoginFormCursors()
	return m, nil
}

func (m *Model) syncLoginFormCursors() {
	m.LoginGatewayCursor = len(m.LoginGatewayURL)
	m.LoginUsernameCursor = len(m.LoginUsername)
	m.LoginPasswordCursor = len(m.LoginPassword)
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
	case "left":
		m.loginFormMoveRune(-1)
		m.LoginErr = ""
		return m, nil
	case "right":
		m.loginFormMoveRune(1)
		m.LoginErr = ""
		return m, nil
	case "ctrl+left":
		if m.LoginFocusedField != 2 {
			m.loginFormWordLeft()
		}
		m.LoginErr = ""
		return m, nil
	case "ctrl+right":
		if m.LoginFocusedField != 2 {
			m.loginFormWordRight()
		}
		m.LoginErr = ""
		return m, nil
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

func (m *Model) loginFieldString() *string {
	switch m.LoginFocusedField {
	case 0:
		return &m.LoginGatewayURL
	case 1:
		return &m.LoginUsername
	case 2:
		return &m.LoginPassword
	default:
		return &m.LoginGatewayURL
	}
}

func (m *Model) loginFieldCursorPtr() *int {
	switch m.LoginFocusedField {
	case 0:
		return &m.LoginGatewayCursor
	case 1:
		return &m.LoginUsernameCursor
	case 2:
		return &m.LoginPasswordCursor
	default:
		return &m.LoginGatewayCursor
	}
}

func (m *Model) loginFormMoveRune(dir int) {
	s := m.loginFieldString()
	cur := m.loginFieldCursorPtr()
	*cur = moveStringCursorRune(*s, *cur, dir)
}

func (m *Model) loginFormWordLeft() {
	s := m.loginFieldString()
	cur := m.loginFieldCursorPtr()
	*cur = moveStringCursorWordLeft(*s, *cur)
}

func (m *Model) loginFormWordRight() {
	s := m.loginFieldString()
	cur := m.loginFieldCursorPtr()
	*cur = moveStringCursorWordRight(*s, *cur)
}

func (m *Model) loginFormBackspace() {
	s := m.loginFieldString()
	cur := m.loginFieldCursorPtr()
	ns, nc := deleteRuneBeforeCursorString(*s, *cur)
	*s = ns
	*cur = nc
}

func (m *Model) loginFormAppend(s string) {
	f := m.loginFieldString()
	cur := m.loginFieldCursorPtr()
	ns, nc := insertStringAtCursor(*f, *cur, s)
	*f = ns
	*cur = nc
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
	const maxComposerLines = 5
	m.clampInputCursor()
	composerBox := m.renderComposerBox(m.buildComposerDisplayLines(maxComposerLines))
	composerVisualH := max(1, lipgloss.Height(composerBox))
	slashMenuBlock, slashMenuH := m.viewSlashMenuSection()
	copyHintBlock, copyHintH := m.viewCopyHintSection()
	errLines, clipNoteBlock, clipNoteH := m.viewErrAndClipSections()
	const statusLines = 1
	scrollbackH := m.Height - composerVisualH - slashMenuH - copyHintH - statusLines - errLines - clipNoteH
	m.ensureScrollViewport(scrollbackH)
	scrollbackView := m.renderScrollbackViewport()
	statusBar := m.viewStatusBar()
	errLine := m.viewErrLine()
	mainView := scrollbackView + "\n" + composerBox + "\n" + slashMenuBlock + copyHintBlock + clipNoteBlock + statusBar + errLine
	if m.ShowLoginForm {
		mainView = m.renderLoginOverlay(mainView)
	}
	return mainView
}

func (m *Model) viewSlashMenuSection() (block string, h int) {
	if !m.slashMenuVisible() {
		return "", 0
	}
	block = m.renderSlashMenuBlock() + "\n"
	h = lipgloss.Height(strings.TrimSuffix(block, "\n"))
	if h < 1 {
		h = 1
	}
	return block, h
}

func (m *Model) viewCopyHintSection() (block string, h int) {
	block = m.renderCopyHintLine() + "\n"
	h = lipgloss.Height(block)
	if h < 1 {
		h = 1
	}
	return block, h
}

func (m *Model) viewErrAndClipSections() (errLines int, clipBlock string, clipH int) {
	if m.Err != "" {
		errLines = 1
	}
	if m.ClipNote == "" {
		return errLines, "", 0
	}
	clipBlock = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Width(m.Width).Render(m.ClipNote) + "\n"
	clipH = lipgloss.Height(clipBlock)
	if clipH < 1 {
		clipH = 1
	}
	return errLines, clipBlock, clipH
}

func (m *Model) renderScrollbackViewport() string {
	sig := m.scrollbackRenderSignature()
	if !m.scrollbackCacheValid || m.scrollbackCacheSig != sig {
		m.scrollbackRendered = m.renderScrollbackContent()
		m.scrollbackCacheSig = sig
		m.scrollbackCacheValid = true
	}
	prevLines := m.ScrollVP.TotalLineCount()
	wasAtBottom := m.ScrollVP.AtBottom()
	m.ScrollVP.SetContent(m.scrollbackRendered)
	if wasAtBottom && m.ScrollVP.TotalLineCount() > prevLines {
		m.ScrollVP.GotoBottom()
	}
	return m.ScrollVP.View()
}

func (m *Model) viewStatusBar() string {
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
	return lipgloss.NewStyle().Width(m.Width).Render(statusLine)
}

func (m *Model) viewErrLine() string {
	if m.Err == "" {
		return ""
	}
	st := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	return st.Render(" "+m.Err) + "\n"
}

// loginPanelTheme holds lipgloss styles for the login overlay card.
type loginPanelTheme struct {
	panelBG         lipgloss.Color
	labelStyle      lipgloss.Style
	valueStyle      lipgloss.Style
	focusValueStyle lipgloss.Style
	titleStyle      lipgloss.Style
	landmarkStyle   lipgloss.Style
	hintKeyStyle    lipgloss.Style
	hintDimStyle    lipgloss.Style
	loginErrStyle   lipgloss.Style
	boxStyle        lipgloss.Style
	gapStr          string
	blankRow        string
	innerContentW   int
}

func (m *Model) loginOverlayInnerWidth() int {
	innerW := m.Width - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerW > loginBoxMaxInnerW {
		innerW = loginBoxMaxInnerW
	}
	if innerW < loginBoxMinInnerW && m.Width > loginBoxMinInnerW+2 {
		innerW = loginBoxMinInnerW
	}
	return innerW
}

func (m *Model) newLoginPanelTheme(innerW int) *loginPanelTheme {
	loginErrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	labelStyle := lipgloss.NewStyle().Width(loginLabelColWidth).Align(lipgloss.Right)
	valueStyle := lipgloss.NewStyle()
	focusValueStyle := lipgloss.NewStyle().Bold(true)
	titleStyle := lipgloss.NewStyle().Bold(true)
	landmarkStyle := lipgloss.NewStyle()
	hintKeyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	hintDimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	panelBG := lipgloss.Color("#000000")
	if !m.wantNoColor() {
		labelStyle = labelStyle.Background(panelBG).Foreground(lipgloss.Color("244"))
		valueStyle = valueStyle.Background(panelBG).Foreground(lipgloss.Color("252"))
		focusValueStyle = focusValueStyle.Background(panelBG).Foreground(lipgloss.Color("86"))
		titleStyle = titleStyle.Background(panelBG).Foreground(lipgloss.Color("86"))
		landmarkStyle = landmarkStyle.Background(panelBG).Foreground(lipgloss.Color("241"))
		hintKeyStyle = hintKeyStyle.Background(panelBG)
		hintDimStyle = hintDimStyle.Background(panelBG)
		loginErrStyle = loginErrStyle.Background(panelBG)
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(innerW)
	if !m.wantNoColor() {
		boxStyle = boxStyle.BorderForeground(lipgloss.Color("86")).Background(panelBG)
	}
	innerContentW := innerW - 4
	if innerContentW < 1 {
		innerContentW = 1
	}
	gapStr := " "
	if !m.wantNoColor() {
		gapStr = lipgloss.NewStyle().Background(panelBG).Render(" ")
	}
	var blankRow string
	if !m.wantNoColor() {
		blankRow = lipgloss.NewStyle().Width(innerContentW).Background(panelBG).Render(strings.Repeat(" ", innerContentW))
	}
	return &loginPanelTheme{
		panelBG:         panelBG,
		labelStyle:      labelStyle,
		valueStyle:      valueStyle,
		focusValueStyle: focusValueStyle,
		titleStyle:      titleStyle,
		landmarkStyle:   landmarkStyle,
		hintKeyStyle:    hintKeyStyle,
		hintDimStyle:    hintDimStyle,
		loginErrStyle:   loginErrStyle,
		boxStyle:        boxStyle,
		gapStr:          gapStr,
		blankRow:        blankRow,
		innerContentW:   innerContentW,
	}
}

func (m *Model) renderLoginFieldLine(th *loginPanelTheme, label, disp string, cursorByte int, focused bool) string {
	lbl := th.labelStyle.Render(label)
	var valPart string
	switch {
	case focused:
		valPart = renderStyledLineWithCursor(&th.focusValueStyle, disp, cursorByte)
	case disp == "":
		valPart = th.valueStyle.Render(" ")
	default:
		valPart = th.valueStyle.Render(disp)
	}
	line := lbl + th.gapStr + valPart
	if focused && !m.wantNoColor() {
		return lipgloss.NewStyle().Width(th.innerContentW).Background(th.panelBG).Render(line)
	}
	return line
}

func (m *Model) mergeLoginBoxOntoMainView(mainView, box string, th *loginPanelTheme) string {
	boxLines := strings.Split(box, "\n")
	mainLines := strings.Split(mainView, "\n")
	if len(mainLines) < len(boxLines) {
		return mainView + "\n" + box
	}
	startRow := (len(mainLines) - len(boxLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	centerStyle := lipgloss.NewStyle().Width(m.Width).AlignHorizontal(lipgloss.Center)
	if !m.wantNoColor() {
		centerStyle = centerStyle.Background(th.panelBG)
	}
	for i, line := range boxLines {
		idx := startRow + i
		if idx < len(mainLines) {
			mainLines[idx] = centerStyle.Render(line)
		}
	}
	return strings.Join(mainLines, "\n")
}

// renderLoginOverlay draws the login box over the main view (password not echoed; REQ-CLIENT-0190).
func (m *Model) renderLoginOverlay(mainView string) string {
	innerW := m.loginOverlayInnerWidth()
	th := m.newLoginPanelTheme(innerW)
	passStars := strings.Repeat("*", len(m.LoginPassword))
	titleLine := th.titleStyle.Render(" Sign in ") + lipgloss.NewStyle().Background(th.panelBG).Render(" ") + th.landmarkStyle.Render(chat.LandmarkAuthRecoveryReady)
	if m.wantNoColor() {
		titleLine = th.titleStyle.Render(" Sign in ") + " " + th.landmarkStyle.Render(chat.LandmarkAuthRecoveryReady)
	}
	hintLine := th.hintDimStyle.Render(" ") +
		th.hintKeyStyle.Render("[Enter]") + th.hintDimStyle.Render(" Login  ") +
		th.hintKeyStyle.Render("[Esc]") + th.hintDimStyle.Render(" Cancel")
	lines := []string{
		titleLine,
		th.blankRow,
		m.renderLoginFieldLine(th, "Gateway URL:", m.LoginGatewayURL, clampStringCursor(m.LoginGatewayURL, m.LoginGatewayCursor), m.LoginFocusedField == 0),
		m.renderLoginFieldLine(th, "Username:", m.LoginUsername, clampStringCursor(m.LoginUsername, m.LoginUsernameCursor), m.LoginFocusedField == 1),
		m.renderLoginFieldLine(th, "Password:", passStars, clampStringCursor(m.LoginPassword, m.LoginPasswordCursor), m.LoginFocusedField == 2),
		th.blankRow,
		hintLine,
	}
	if m.LoginErr != "" {
		if !m.wantNoColor() {
			lines = append(lines, th.blankRow)
		} else {
			lines = append(lines, "")
		}
		lines = append(lines, th.loginErrStyle.Render(m.LoginErr))
	}
	box := th.boxStyle.Render(strings.Join(lines, "\n"))
	return m.mergeLoginBoxOntoMainView(mainView, box, th)
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
