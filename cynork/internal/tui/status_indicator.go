package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

// gatewayHealthState is the last result of GET /healthz (when polling is enabled).
type gatewayHealthState int

const (
	gatewayHealthUnknown gatewayHealthState = iota
	gatewayHealthOK
	gatewayHealthDown
	gatewayHealthNoClient
)

// Status glyphs (single cell). Filled states use U+2B24 BLACK LARGE CIRCLE for a bigger dot than U+25CF;
// outline uses U+25EF LARGE CIRCLE. Busy keeps U+25D0 half-circle so it differs from idle in no-color mode.
const (
	statusGlyphBusy     = "◐"
	statusGlyphOK       = "⬤"
	statusGlyphDown     = "⬤"
	statusGlyphUnknown  = "◯"
	statusGlyphNoClient = "◯"
)

// gatewayHealthPollMsg requests a background GET /healthz check.
type gatewayHealthPollMsg struct{}

// gatewayHealthResultMsg carries the outcome of a health check.
type gatewayHealthResultMsg struct {
	state gatewayHealthState
}

func (m *Model) maybeStartGatewayHealthPollOnce() tea.Cmd {
	if m.gatewayHealthPollStarted || m.healthPollIntervalSec <= 0 {
		return nil
	}
	if m.Session == nil || m.Session.Client == nil {
		return nil
	}
	m.gatewayHealthPollStarted = true
	d := time.Duration(m.healthPollIntervalSec) * time.Second
	return tea.Batch(
		func() tea.Msg { return gatewayHealthPollMsg{} },
		tea.Every(d, func(time.Time) tea.Msg { return gatewayHealthPollMsg{} }),
	)
}

func (m *Model) handleGatewayHealthPoll() (tea.Model, tea.Cmd) {
	if m.healthPollIntervalSec <= 0 {
		return m, nil
	}
	cmd := m.gatewayHealthCheckCmd()
	return m, cmd
}

func (m *Model) gatewayHealthCheckCmd() tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil || m.Session.Client == nil {
			return gatewayHealthResultMsg{state: gatewayHealthNoClient}
		}
		if err := m.Session.Client.Health(); err != nil {
			return gatewayHealthResultMsg{state: gatewayHealthDown}
		}
		return gatewayHealthResultMsg{state: gatewayHealthOK}
	}
}

func (m *Model) applyGatewayHealthResult(msg gatewayHealthResultMsg) (tea.Model, tea.Cmd) {
	m.gatewayHealth = msg.state
	return m, nil
}

// renderGatewayStatusIndicator returns a short styled glyph for the status bar (may include ANSI).
func (m *Model) renderGatewayStatusIndicator() string {
	if m.Loading {
		return statusIndicatorStyleBusy(m).Render(statusGlyphBusy)
	}
	// Polling disabled: legacy idle glyph (bold).
	if m.healthPollIntervalSec <= 0 {
		return lipgloss.NewStyle().Bold(true).Render(chat.TUIStatusIdle)
	}
	if m.wantNoColor() {
		switch m.gatewayHealth {
		case gatewayHealthOK:
			return lipgloss.NewStyle().Bold(true).Render(statusGlyphOK)
		case gatewayHealthDown:
			return lipgloss.NewStyle().Bold(true).Render(statusGlyphDown)
		default:
			return lipgloss.NewStyle().Bold(true).Render(statusGlyphUnknown)
		}
	}
	switch m.gatewayHealth {
	case gatewayHealthOK:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true).Render(statusGlyphOK)
	case gatewayHealthDown:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render(statusGlyphDown)
	case gatewayHealthNoClient:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true).Render(statusGlyphNoClient)
	default: // unknown (before first poll)
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true).Render(statusGlyphUnknown)
	}
}

func statusIndicatorStyleBusy(m *Model) lipgloss.Style {
	st := lipgloss.NewStyle().Bold(true)
	if m.wantNoColor() {
		return st
	}
	return st.Foreground(lipgloss.Color("214"))
}

func combineTeaCmds(a, b tea.Cmd) tea.Cmd {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return tea.Batch(a, b)
}
