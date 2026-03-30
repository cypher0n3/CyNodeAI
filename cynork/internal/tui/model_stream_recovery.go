// Connection recovery after streaming transport failures (CYNAI.CLIENT.CynorkTui.ConnectionRecovery).
package tui

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const streamRecoveryMaxAttempts = 5

// streamRecoveryTickMsg schedules a GET /healthz check after bounded backoff.
type streamRecoveryTickMsg struct {
	attempt int
	gen     int
}

func streamRecoveryBackoff(attempt int) time.Duration {
	d := 200 * time.Millisecond
	for i := 1; i < attempt && i < 12; i++ {
		d *= 2
		if d > 5*time.Second {
			return 5 * time.Second
		}
	}
	return d
}

// isRecoverableGatewayStreamError reports transport-level failures where reconnecting may help.
// User cancel and HTTP semantic errors are not recoverable here.
func isRecoverableGatewayStreamError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var oe *net.OpError
	if errors.As(err, &oe) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection reset") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "eof") ||
		strings.Contains(s, "timeout") ||
		strings.Contains(s, "refused") ||
		strings.Contains(s, "network is unreachable")
}

func (m *Model) clearConnectionRecovery() {
	m.connectionRecoveryState = ConnectionStateUnknown
	m.streamRecoveryAttempt = 0
}

func (m *Model) maybeScheduleStreamRecovery(done streamDoneMsg) tea.Cmd {
	if done.err == nil {
		m.clearConnectionRecovery()
		return nil
	}
	if errors.Is(done.err, context.Canceled) {
		m.clearConnectionRecovery()
		return nil
	}
	if !isRecoverableGatewayStreamError(done.err) {
		m.clearConnectionRecovery()
		return nil
	}
	if m.Session == nil || m.Session.Client == nil {
		m.connectionRecoveryState = ConnectionStateDisconnected
		return nil
	}
	m.streamRecoveryAttempt = 1
	m.connectionRecoveryState = ConnectionStateReconnecting
	return m.streamRecoveryTickCmd(1, m.streamRecoveryGen)
}

func (m *Model) streamRecoveryTickCmd(attempt, gen int) tea.Cmd {
	d := streamRecoveryBackoff(attempt)
	return tea.Tick(d, func(time.Time) tea.Msg {
		return streamRecoveryTickMsg{attempt: attempt, gen: gen}
	})
}

func (m *Model) applyStreamRecoveryTick(msg streamRecoveryTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.streamRecoveryGen {
		return m, nil
	}
	if m.Session == nil || m.Session.Client == nil {
		m.connectionRecoveryState = ConnectionStateDisconnected
		m.streamRecoveryAttempt = 0
		return m, nil
	}
	if err := m.Session.Client.Health(context.Background()); err != nil {
		if msg.attempt >= streamRecoveryMaxAttempts {
			m.connectionRecoveryState = ConnectionStateDisconnected
			m.streamRecoveryAttempt = 0
			m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Gateway unreachable after reconnect attempts.")
			return m, nil
		}
		next := msg.attempt + 1
		m.streamRecoveryAttempt = next
		cmd := m.streamRecoveryTickCmd(next, msg.gen)
		return m, cmd
	}
	m.clearConnectionRecovery()
	m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Gateway connection restored.")
	return m, nil
}
