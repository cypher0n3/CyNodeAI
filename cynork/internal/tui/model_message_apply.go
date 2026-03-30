// Package tui provides the full-screen TUI for cynork. See docs/tech_specs/cynork_tui.md.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

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
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+fmt.Sprintf("exit status %d", msg.exitCode))
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
	case m.Session != nil && m.Session.Client != nil && m.Session.Client.BaseURL() != "":
		m.LoginGatewayURL = m.Session.Client.BaseURL()
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
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Login failed: "+msg.Err.Error())
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
			m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Logged in but config save failed: "+err.Error())
			return m, nil
		}
	}
	m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Logged in.")
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
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Login cancelled.")
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
		resp, err := client.Login(context.Background(), userapi.LoginRequest{Handle: username, Password: password})
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
		if gateway.IsUnauthorized(msg.err) {
			m.Err = ""
			_, _ = m.applyOpenLoginForm()
			m.Scrollback = append(m.Scrollback,
				ScrollbackSystemLinePrefix+chat.LandmarkAuthRecoveryReady+" Session expired. Sign in to continue.")
			return
		}
		m.Err = msg.err.Error()
		m.Scrollback = append(m.Scrollback, "Error: "+m.Err)
	} else if msg.visible != "" {
		m.Scrollback = append(m.Scrollback, assistantPrefix+msg.visible)
	}
}

// applyStreamDoneNoVisibleError handles terminal error with no accumulated visible text.
func (m *Model) applyStreamDoneNoVisibleError(msg streamDoneMsg) bool {
	if msg.err == nil {
		return false
	}
	// User canceled before any visible token: still surface interrupted semantics (PTY/E2E).
	if errors.Is(msg.err, context.Canceled) {
		m.Err = ""
		m.Scrollback = append(m.Scrollback, "(stream interrupted)")
		return true
	}
	// Stream failed with no partial content.
	m.Err = msg.err.Error()
	prefix := assistantPrefix
	if len(m.Scrollback) > 0 && strings.HasPrefix(m.Scrollback[len(m.Scrollback)-1], prefix) {
		m.Scrollback[len(m.Scrollback)-1] = "Error: " + m.Err
	} else {
		m.Scrollback = append(m.Scrollback, "Error: "+m.Err)
	}
	return true
}

// applyUnauthorizedStreamEnd opens the in-TUI login overlay after a streaming turn ends with HTTP 401
// (REQ-CLIENT-0190 in-session auth recovery). Preserves partial visible content when present.
func (m *Model) applyUnauthorizedStreamEnd(final string) {
	if len(m.Transcript) > 0 {
		last := &m.Transcript[len(m.Transcript)-1]
		if last.Role == RoleAssistant && last.InFlight {
			last.InFlight = false
			last.Content = final
			last.Interrupted = true
		}
	}
	if final == "" {
		prefix := assistantPrefix
		if len(m.Scrollback) > 0 && strings.HasPrefix(m.Scrollback[len(m.Scrollback)-1], prefix) {
			m.Scrollback = m.Scrollback[:len(m.Scrollback)-1]
		}
	} else {
		m.reconcileFinalAssistantScrollback(final)
	}
	m.Err = ""
	_, _ = m.applyOpenLoginForm()
	m.Scrollback = append(m.Scrollback,
		ScrollbackSystemLinePrefix+chat.LandmarkAuthRecoveryReady+" Session expired. Sign in to continue.")
}

func (m *Model) reconcileFinalAssistantScrollback(final string) {
	prefix := assistantPrefix
	if len(m.Scrollback) == 0 || !strings.HasPrefix(m.Scrollback[len(m.Scrollback)-1], prefix) {
		return
	}
	if final == "" {
		m.Scrollback[len(m.Scrollback)-1] = prefix + "(no response)"
		return
	}
	m.Scrollback[len(m.Scrollback)-1] = prefix + final
}

func (m *Model) applyStreamDone(msg streamDoneMsg) {
	m.Loading = false
	m.streamCancel = nil
	m.streamCh = nil
	m.streamHeartbeatNote = ""
	final := strings.TrimSpace(m.streamBuf.String())
	m.streamBuf.Reset()
	if msg.err != nil && gateway.IsUnauthorized(msg.err) {
		m.applyUnauthorizedStreamEnd(final)
		return
	}
	if len(m.Transcript) > 0 {
		last := &m.Transcript[len(m.Transcript)-1]
		if last.Role == RoleAssistant && last.InFlight {
			last.InFlight = false
			last.Content = final
			// Any terminal stream error (cancel, disconnect, upstream failure) marks an interrupted turn.
			last.Interrupted = msg.err != nil
		}
	}
	if msg.err != nil && final == "" && m.applyStreamDoneNoVisibleError(msg) {
		return
	}
	m.reconcileFinalAssistantScrollback(final)
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
	m.seedTranscriptAssistantInFlight()
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	m.streamBuf.Reset()
	m.streamRecoveryGen++
	m.clearConnectionRecovery()

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

// isAgentStreaming reports whether an assistant stream is in progress (Bug 4 queue model).
func (m *Model) isAgentStreaming() bool {
	return m.streamCancel != nil
}

// beginUserTurnStream starts a user message turn with streaming (scrollback + transcript + streamCmd).
func (m *Model) beginUserTurnStream(line string, skipHistory bool) tea.Cmd {
	m.Scrollback = append(m.Scrollback, "You: "+line)
	m.appendTranscriptUser(line)
	if !skipHistory {
		m.pushInputHistory(line)
		m.InputHistoryIdx = -1
	}
	m.Loading = true
	return m.streamCmd(line)
}

// maybeStartNextQueuedUserTurn sends the next queued or interrupted user message after a stream ends.
// When autoOnly is true, only the auto-queue (Enter-while-streaming) is consumed; explicit Ctrl+Q drafts remain.
func (m *Model) maybeStartNextQueuedUserTurn(autoOnly bool) tea.Cmd {
	if m.isAgentStreaming() {
		return nil
	}
	var line string
	var skipHist bool
	switch {
	case m.pendingInterruptSend != "":
		line = m.pendingInterruptSend
		m.pendingInterruptSend = ""
		skipHist = false
	case len(m.queuedAutoSend) > 0:
		line = m.queuedAutoSend[0]
		m.queuedAutoSend = m.queuedAutoSend[1:]
		skipHist = true
	case !autoOnly && len(m.queuedExplicit) > 0:
		line = m.queuedExplicit[0]
		m.queuedExplicit = m.queuedExplicit[1:]
		skipHist = true
	default:
		return nil
	}
	return m.beginUserTurnStream(line, skipHist)
}

// handleCtrlEnterKey sends the composer now (interrupting streaming) or drains the next queued draft (Bug 4).
func (m *Model) handleCtrlEnterKey() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.Input)
	if m.maybeApplySlashMenuEnterCompletion() {
		return m, nil
	}
	m.Input = ""
	m.inputCursor = 0
	if line != "" {
		if m.isAgentStreaming() {
			m.pendingInterruptSend = line
			if m.streamCancel != nil {
				m.streamCancel()
			}
			return m, nil
		}
		cmd := m.beginUserTurnStream(line, false)
		return m, cmd
	}
	if m.isAgentStreaming() {
		var next string
		switch {
		case len(m.queuedAutoSend) > 0:
			next = m.queuedAutoSend[0]
			m.queuedAutoSend = m.queuedAutoSend[1:]
		case len(m.queuedExplicit) > 0:
			next = m.queuedExplicit[0]
			m.queuedExplicit = m.queuedExplicit[1:]
		default:
			return m, nil
		}
		m.pendingInterruptSend = next
		if m.streamCancel != nil {
			m.streamCancel()
		}
		return m, nil
	}
	cmd := m.maybeStartNextQueuedUserTurn(false)
	return m, cmd
}

// handleCtrlQKey queues the composer line without sending (Bug 4).
func (m *Model) handleCtrlQKey() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.Input)
	if line == "" {
		return m, nil
	}
	m.queuedExplicit = append(m.queuedExplicit, line)
	m.pushInputHistory(line)
	m.InputHistoryIdx = -1
	m.Input = ""
	m.inputCursor = 0
	return m, nil
}

func chatStreamDeltaToMsg(delta *chat.ChatStreamDelta) tea.Msg {
	if delta == nil {
		return streamDeltaMsg{}
	}
	if delta.Done {
		return streamDoneMsg{responseID: delta.ResponseID, err: delta.Err}
	}
	if delta.Amendment != "" {
		return streamDeltaMsg{amendment: delta.Amendment}
	}
	if delta.Thinking != "" {
		return streamDeltaMsg{thinking: delta.Thinking}
	}
	if delta.ToolName != "" || delta.ToolArgs != "" {
		return streamDeltaMsg{toolName: delta.ToolName, toolArgs: delta.ToolArgs}
	}
	if delta.IsHeartbeat {
		return streamDeltaMsg{isHeartbeat: true, hbElapsed: delta.HeartbeatElapsed, hbStatus: delta.HeartbeatStatus}
	}
	if delta.IterationStart {
		return streamDeltaMsg{iterationStart: true, iteration: delta.Iteration}
	}
	return streamDeltaMsg{delta: delta.Delta}
}

// readNextDelta returns the next delta, amendment, or streamPollMsg if nothing is ready.
func readNextDelta(ch <-chan chat.ChatStreamDelta) tea.Msg {
	select {
	case delta, ok := <-ch:
		if !ok {
			return streamDoneMsg{}
		}
		return chatStreamDeltaToMsg(&delta)
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
	hb := ""
	if strings.TrimSpace(m.streamHeartbeatNote) != "" {
		hb = " | " + m.streamHeartbeatNote
	}
	rec := ""
	switch m.connectionRecoveryState {
	case ConnectionStateReconnecting:
		rec = " | " + string(ConnectionStateReconnecting)
	case ConnectionStateDisconnected:
		rec = " | " + string(ConnectionStateDisconnected)
	}
	tail := fmt.Sprintf(" | project: %s | model: %s | thread: %s | %s%s%s",
		projectID, modelName, thread, composerHint, hb, rec)
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

// LastAssistantPlain returns the raw text of the most recent assistant scrollback line (no "Assistant: " prefix).
func LastAssistantPlain(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], assistantPrefix) {
			return strings.TrimPrefix(lines[i], assistantPrefix)
		}
	}
	return ""
}

// PlainTranscript joins scrollback lines for /copy all (plain text).
// Lines marked as system feedback (slash/thread/shell) are omitted so the transcript is chat turns only.
func PlainTranscript(lines []string) string {
	var b strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, ScrollbackSystemLinePrefix) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}
