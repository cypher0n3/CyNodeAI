package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// BDDApplyLoginSuccess simulates a successful in-TUI login (Godog BDD in-memory; no PTY).
func (m *Model) BDDApplyLoginSuccess(gatewayURL, accessToken, refreshToken string) tea.Model {
	nm, _ := m.Update(loginResultMsg{
		GatewayURL:   gatewayURL,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
	return nm
}

// BDDApplyLoginFailure simulates a failed login attempt.
func (m *Model) BDDApplyLoginFailure(gatewayURL string, err error) tea.Model {
	nm, _ := m.Update(loginResultMsg{GatewayURL: gatewayURL, Err: err})
	return nm
}

// BDDApplyKey applies a key message (for example Esc to dismiss the login overlay in BDD).
func (m *Model) BDDApplyKey(msg tea.KeyMsg) tea.Model {
	nm, _ := m.Update(msg)
	return nm
}
