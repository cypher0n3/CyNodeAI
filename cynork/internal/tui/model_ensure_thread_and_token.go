package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cypher0n3/cynodeai/cynork/internal/tuicache"
)

func (m *Model) applyEnsureThreadResult(msg *ensureThreadResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"Error: "+msg.err.Error())
	} else if msg.threadID != "" {
		if m.Session != nil {
			m.Session.SetCurrentThreadID(msg.threadID)
			uid := msg.userID
			if uid == "" {
				// Async ensure-thread may have run GetMe before the gateway was ready; retry so
				// last_threads.json is written for the next TUI launch (E2E resume UX).
				uid = m.userIDForEnsureThread()
			}
			if uid != "" && m.Session.Client != nil {
				if root, err := tuicache.Root(); err == nil {
					_ = tuicache.WriteLastThread(root, m.Session.Client.BaseURL(), uid, m.Session.ProjectID, msg.threadID)
				}
			}
		}
		line := ScrollbackSystemLinePrefix + ensureThreadScrollbackLine(msg.priorThreadID, msg.threadID, msg.resumeSelector, msg.createdNew, msg.resumedFromCache)
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
	// Refresh runs as an async Cmd and must not be suppressed during Loading (long streams).
	if m.ShowLoginForm {
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
		resp, err := m.Session.Client.Refresh(context.Background(), rt)
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
	if m.Session != nil && m.Session.Client != nil {
		m.restartSessionNatsFromLogin(m.Session.Client, msg.resp)
	}
	return m, nil
}
