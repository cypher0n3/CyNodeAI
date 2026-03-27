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
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/tuicache"
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
	// priorThreadID is CurrentThreadID immediately before EnsureThread (after cache resume).
	priorThreadID string
	// resumeSelector is the CLI --resume-thread value (empty for default ensure path).
	resumeSelector string
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
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Copy failed: "+msg.err.Error())
	} else {
		line := msg.successDetail
		if line == "" {
			line = "Copied to clipboard."
		}
		m.ClipNote = line
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+line)
	}
	cmd := m.scheduleClipNoteClear()
	return m, cmd
}

func (m *Model) scheduleClipNoteClear() tea.Cmd {
	return tea.Tick(clipNoteDuration, func(time.Time) tea.Msg { return clipNoteClearMsg{} })
}

func (m *Model) cmdCopyLastAssistant() tea.Cmd {
	text := LastAssistantPlain(m.Scrollback)
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
	m.NavigateInputHistory(true)
	return m, nil
}

func (m *Model) handleComposerCtrlDownKey() (tea.Model, tea.Cmd) {
	if m.slashMenuVisible() && len(m.filteredSlashCommands()) > 0 {
		return m, nil
	}
	m.NavigateInputHistory(false)
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
	// Block only plain chat sends while streaming; slash and shell remain dispatchable (Bug 4).
	if m.Loading && line != "" && !strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "!") {
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

// EnterBlockedWhileLoading reports whether handleEnterKey would ignore a non-empty composer line (Bug 4).
func EnterBlockedWhileLoading(loading bool, input string) bool {
	line := strings.TrimSpace(input)
	return loading && line != "" && !strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "!")
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
	m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Unknown command. Type /help for available commands.")
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
	m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"(Press Ctrl+C again to exit)")
	return m, nil
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
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Error: "+msg.err.Error())
	} else {
		m.Scrollback = append(m.Scrollback, wrapSystemScrollbackLines(msg.lines)...)
	}
	return m, nil
}

func (m *Model) applyThreadRenameResult(msg threadRenameResult) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.err != nil {
		m.Err = msg.err.Error()
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Error: "+msg.err.Error())
	} else {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Thread renamed.")
	}
	return m, nil
}

func (m *Model) ensureThreadCmd() tea.Cmd {
	return func() tea.Msg {
		return m.runEnsureThread()
	}
}

func (m *Model) runEnsureThread() tea.Msg {
	if m.Session == nil {
		return ensureThreadResult{err: fmt.Errorf("no session")}
	}
	userID := m.userIDForEnsureThread()
	m.tryResumeThreadFromCache(userID)
	prior := m.Session.CurrentThreadID
	sel := m.ResumeThreadSelector
	if err := m.Session.EnsureThread(sel); err != nil {
		return ensureThreadResult{err: err}
	}
	m.persistCurrentThreadAfterEnsure(userID)
	return ensureThreadResult{
		threadID:       m.Session.CurrentThreadID,
		priorThreadID:  prior,
		resumeSelector: sel,
	}
}

func (m *Model) userIDForEnsureThread() string {
	if m.Session.Client == nil || m.Session.Client.Token == "" {
		return ""
	}
	u, err := m.Session.Client.GetMe()
	if err != nil {
		return ""
	}
	return u.ID
}

func (m *Model) tryResumeThreadFromCache(userID string) {
	if m.Session.CurrentThreadID != "" || m.ResumeThreadSelector != "" || userID == "" || m.Session.Client == nil {
		return
	}
	root, err := tuicache.Root()
	if err != nil {
		return
	}
	tid, ok, _ := tuicache.ReadLastThread(root, m.Session.Client.BaseURL, userID, m.Session.ProjectID)
	if ok {
		m.Session.SetCurrentThreadID(tid)
	}
}

func (m *Model) persistCurrentThreadAfterEnsure(userID string) {
	if userID == "" || m.Session.Client == nil {
		return
	}
	root, err := tuicache.Root()
	if err != nil {
		return
	}
	_ = tuicache.WriteLastThread(root, m.Session.Client.BaseURL, userID, m.Session.ProjectID, m.Session.CurrentThreadID)
}

// persistLastThreadToCache writes CurrentThreadID under the XDG cache dir (keyed by gateway, user, project).
func (m *Model) persistLastThreadToCache() {
	if m.Session == nil || m.Session.Client == nil || m.Session.Client.Token == "" || m.Session.CurrentThreadID == "" {
		return
	}
	u, err := m.Session.Client.GetMe()
	if err != nil {
		return
	}
	root, err := tuicache.Root()
	if err != nil {
		return
	}
	_ = tuicache.WriteLastThread(root, m.Session.Client.BaseURL, u.ID, m.Session.ProjectID, m.Session.CurrentThreadID)
}

// ensureThreadScrollbackLine picks landmark + suffix for thread ensure scrollback (Bug 3: avoid
// spurious [CYNRK_THREAD_SWITCHED] when the active thread id did not change).
func ensureThreadScrollbackLine(priorID, afterID, resumeSelector string) string {
	if priorID == afterID {
		return chat.LandmarkThreadReady + " Thread: " + afterID
	}
	if resumeSelector != "" && priorID != afterID {
		return chat.LandmarkThreadSwitched + " Thread: " + afterID
	}
	if priorID != "" && priorID != afterID {
		return chat.LandmarkThreadSwitched + " Thread: " + afterID
	}
	return chat.LandmarkThreadReady + " Thread: " + afterID
}

// EnsureThreadScrollbackSystemLine returns the full dim-prefixed scrollback line for EnsureThread (Bug 3).
// Exported for BDD steps; matches applyEnsureThreadResult.
func EnsureThreadScrollbackSystemLine(priorID, afterID, resumeSelector string) string {
	return ScrollbackSystemLinePrefix + ensureThreadScrollbackLine(priorID, afterID, resumeSelector)
}

func (m *Model) applyEnsureThreadResult(msg ensureThreadResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Error: "+msg.err.Error())
	} else if msg.threadID != "" {
		line := ScrollbackSystemLinePrefix + ensureThreadScrollbackLine(msg.priorThreadID, msg.threadID, msg.resumeSelector)
		m.Scrollback = append(m.Scrollback, line)
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
