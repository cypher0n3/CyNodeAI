// Package tui provides the full-screen TUI for cynork. See docs/tech_specs/cynork_tui.md.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

// defaultPlaceholder is shown for empty project/model in the status bar and orEmpty.
const defaultPlaceholder = "(default)"

// assistantPrefix is the prefix for assistant messages in the scrollback.
const assistantPrefix = "Assistant: "

const maxInputHistory = 50

// sendResult is the message returned when a SendMessage completes (non-streaming fallback).
type sendResult struct {
	visible string
	err     error
}

// streamDeltaMsg carries one incremental delta from the active streaming turn.
type streamDeltaMsg struct {
	delta string
}

// streamDoneMsg signals that the active streaming turn is complete.
type streamDoneMsg struct {
	responseID string
	err        error
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

// Model holds the TUI state: session, scrollback, composer input, and dimensions.
type Model struct {
	Session         *chat.Session
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
	streamCh     <-chan chat.ChatStreamDelta  // active stream channel; nil when idle
	streamBuf    strings.Builder             // accumulates in-flight visible text
	ctrlCCount   int                         // successive Ctrl+C when idle → exit
}

// NewModel returns an initial TUI model for the given session.
func NewModel(session *chat.Session) *Model {
	return &Model{
		Session:         session,
		Scrollback:      []string{},
		Input:           "",
		InputHistoryIdx: -1,
		Width:           80,
		Height:          24,
	}
}

// Init runs once at startup; no initial command.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles key events, window resize, and async send results.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	case streamDeltaMsg:
		m.streamBuf.WriteString(msg.delta)
		// Update the last scrollback line in place.
		prefix := assistantPrefix
		if len(m.Scrollback) > 0 && strings.HasPrefix(m.Scrollback[len(m.Scrollback)-1], prefix) {
			m.Scrollback[len(m.Scrollback)-1] = prefix + m.streamBuf.String()
		}
		// Schedule reading the next delta.
		if m.streamCh != nil {
			return m, scheduleNextDelta(m.streamCh)
		}
		return m, nil
	case streamDoneMsg:
		m.applyStreamDone(msg)
		return m, nil
	case sendResult:
		m.applySendResult(msg)
		return m, nil
	case threadListResult:
		m.Loading = false
		if msg.err != nil {
			m.Err = msg.err.Error()
			m.Scrollback = append(m.Scrollback, "Error: "+msg.err.Error())
		} else {
			m.Scrollback = append(m.Scrollback, msg.lines...)
		}
		return m, nil
	case threadRenameResult:
		m.Loading = false
		if msg.err != nil {
			m.Err = msg.err.Error()
			m.Scrollback = append(m.Scrollback, "Error: "+msg.err.Error())
		} else {
			m.Scrollback = append(m.Scrollback, "Thread renamed.")
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.handleCtrlC()
	case "ctrl+d":
		return m, tea.Quit
	}
	// Reset successive ctrl+c count on any other key.
	m.ctrlCCount = 0
	if m.Loading {
		return m, nil
	}
	switch msg.String() {
	case "enter":
		line := strings.TrimSpace(m.Input)
		m.Input = ""
		if line == "" {
			return m, nil
		}
		m.Scrollback = append(m.Scrollback, "You: "+line)
		m.Err = ""
		if cmd := m.handleThreadCommand(line); cmd != nil {
			m.Loading = true
			return m, cmd
		}
		// Not a thread command; send as chat message.
		if !strings.HasPrefix(line, "/") {
			m.pushInputHistory(line)
			m.InputHistoryIdx = -1
			m.Loading = true
			cmd := m.streamCmd(line)
			return m, cmd
		}
		return m, nil
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
		threadID, err := m.Session.NewThread()
		if err != nil {
			m.Scrollback = append(m.Scrollback, "Error: "+err.Error())
			return nil
		}
		m.Scrollback = append(m.Scrollback, chat.LandmarkThreadSwitched+" New thread: "+threadID)
		return nil
	case "list":
		return m.threadListCmd()
	case "switch":
		if len(parts) < 2 {
			m.Scrollback = append(m.Scrollback, "Usage: /thread switch <id>")
			return nil
		}
		id := parts[1]
		m.Session.SetCurrentThreadID(id)
		m.Scrollback = append(m.Scrollback, chat.LandmarkThreadSwitched+" Switched to thread: "+id)
		return nil
	case "rename":
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
	default:
		if rest != "" {
			m.Scrollback = append(m.Scrollback, "Unknown: /thread "+rest+" (use new, list, switch, rename)")
		} else {
			m.Scrollback = append(m.Scrollback, "Thread: new, list, switch <id>, rename <title>")
		}
		return nil
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
		lines := []string{"--- Threads ---"}
		for _, t := range items {
			title := ""
			if t.Title != nil {
				title = *t.Title
			}
			if title == "" {
				title = "(no title)"
			}
			lines = append(lines, fmt.Sprintf("  %s  %s", t.ID, title))
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
		// Store channel reference so Update can chain subsequent reads.
		m.streamCh = ch
		return readNextDelta(ch)
	}
}

// readNextDelta returns a tea.Cmd that reads the next delta from ch.
// This chains: each delta event produces another tea.Cmd via tea.Sequence or direct Batch,
// but for simplicity we use a self-scheduling approach: return a Msg that is immediately
// handled, and the handler schedules the next read via tea.Cmd.
// We use a direct goroutine-to-channel model instead: the goroutine fills the channel,
// and each model Update call for streamDeltaMsg returns a Cmd to read the next item.
func readNextDelta(ch <-chan chat.ChatStreamDelta) tea.Msg {
	delta, ok := <-ch
	if !ok {
		return streamDoneMsg{}
	}
	if delta.Done {
		return streamDoneMsg{responseID: delta.ResponseID, err: delta.Err}
	}
	return streamDeltaMsg{delta: delta.Delta}
}

// scheduleNextDelta returns a tea.Cmd that will read the next item from ch.
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
	status := fmt.Sprintf("%s | gateway: %s | project: %s | model: %s | thread: %s | Enter send, Shift+Enter newline",
		landmark, gatewayURL, projectID, model, thread)
	statusBar := statusStyle.Width(m.Width).Render(status)

	var errLine string
	if m.Err != "" {
		errLine = errStyle.Render(" " + m.Err)
	}

	return scrollbackBox + "\n" + composer + "\n" + statusBar + errLine
}

func orEmpty(s string) string {
	if s == "" {
		return defaultPlaceholder
	}
	return s
}
