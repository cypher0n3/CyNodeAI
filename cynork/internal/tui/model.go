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

// sendResult is the message returned when a SendMessage completes.
type sendResult struct {
	visible string
	err     error
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
	Session    *chat.Session
	Scrollback []string
	Input      string
	Width      int
	Height     int
	Loading    bool
	Err        string
}

// NewModel returns an initial TUI model for the given session.
func NewModel(session *chat.Session) *Model {
	return &Model{
		Session:    session,
		Scrollback: []string{},
		Input:      "",
		Width:      80,
		Height:     24,
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
	if m.Loading {
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c", "ctrl+d":
		return m, tea.Quit
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
		// Not a thread command or sync thread command already applied; maybe send chat
		if !strings.HasPrefix(line, "/") {
			m.Loading = true
			cmd := m.sendCmd(line)
			return m, cmd
		}
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

func (m *Model) applySendResult(msg sendResult) {
	m.Loading = false
	if msg.err != nil {
		m.Err = msg.err.Error()
		m.Scrollback = append(m.Scrollback, "Error: "+m.Err)
	} else if msg.visible != "" {
		m.Scrollback = append(m.Scrollback, "Assistant: "+msg.visible)
	}
}

func (m *Model) sendCmd(line string) tea.Cmd {
	return func() tea.Msg {
		turn, err := m.Session.SendMessage(context.Background(), line)
		if err != nil {
			return sendResult{err: err}
		}
		visible := ""
		if turn != nil {
			visible = turn.VisibleText
		}
		return sendResult{visible: visible}
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

	// Scrollback area (top)
	sb := strings.Join(m.Scrollback, "\n")
	if sb == "" {
		sb = " (scrollback)"
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
