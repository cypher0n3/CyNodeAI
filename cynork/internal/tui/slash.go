// Package tui – slash command and shell-escape dispatcher for the TUI.
// See docs/tech_specs/cynork_tui_slash_commands.md and
// docs/tech_specs/cli_management_app_commands_chat.md (CliChatShellEscape).
package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// slashResultMsg carries the scrollback lines produced by a slash command.
type slashResultMsg struct {
	lines     []string
	exitModel bool // true when /exit or /quit
}

// shellExecDoneMsg is returned by tea.ExecProcess after a shell command finishes.
type shellExecDoneMsg struct {
	output   string
	exitCode int
	err      error
}

// slashHelpCatalog lists only slash commands that are actually implemented (/help shows this list per spec).
var slashHelpCatalog = []struct{ name, desc string }{
	{"/auth", "login, logout, whoami, refresh"},
	{"/clear", "clear scrollback"},
	{"/connect", "show or set gateway URL"},
	{"/exit", "end session"},
	{"/help", "list slash commands"},
	{"/hide-thinking", "collapse retained thinking parts"},
	{"/model", "show or set session model"},
	{"/models", "list available models"},
	{"/project", "show or set project context"},
	{"/quit", "end session (synonym for /exit)"},
	{"/show-thinking", "reveal retained thinking parts"},
	{"/status", "gateway reachability"},
	{"/thread", "new, list, switch <selector>, rename"},
	{"/version", "print cynork version"},
	{"/whoami", "current identity"},
}

// composerHint is the discoverability hint shown in the status bar (REQ-CLIENT-0206).
const composerHint = "/ commands · @ files · ! shell"

// parseSlashTUI splits a /command line into (cmd, rest). cmd is lowercase.
func parseSlashTUI(line string) (cmd, rest string) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "/"))
	idx := strings.Index(line, " ")
	if idx < 0 {
		return strings.ToLower(line), ""
	}
	return strings.ToLower(line[:idx]), strings.TrimSpace(line[idx+1:])
}

// handleSlashCmd dispatches a /command from the TUI.
// It returns a tea.Cmd that will produce a slashResultMsg (or a tea.ExecProcess for interactive).
// If the command is /thread, it returns (nil, false) so the existing thread handler can take over.
func (m *Model) handleSlashCmd(line string) (tea.Cmd, bool) {
	cmd, rest := parseSlashTUI(line)
	switch cmd {
	case "help":
		return m.slashHelpCmd(), true
	case "clear":
		return m.slashClearCmd(), true
	case "version":
		return m.slashVersionCmd(), true
	case "exit", "quit":
		return func() tea.Msg { return slashResultMsg{exitModel: true} }, true
	case "model":
		return m.slashModelCmd(rest), true
	case "models":
		return m.slashModelsCmd(), true
	case "project":
		return m.slashProjectCmd(rest), true
	case "auth":
		return m.slashAuthCmd(rest), true
	case "connect":
		return m.slashConnectCmd(rest), true
	case "show-thinking", "hide-thinking":
		return m.slashSetThinkingCmd(cmd == "show-thinking"), true
	case "status":
		return m.slashStatusCmd(), true
	case "whoami":
		return func() tea.Msg { return m.authWhoami() }, true
	case "thread":
		// Handled by existing handleThreadCommand path.
		return nil, false
	case "":
		return func() tea.Msg {
			return slashResultMsg{lines: []string{"Usage: type /help for available commands."}}
		}, true
	default:
		return func() tea.Msg {
			return slashResultMsg{lines: []string{
				fmt.Sprintf("Unknown command: /%s. Type /help for available commands.", cmd),
			}}
		}, true
	}
}

// handleShellEscape handles lines starting with !.
// For non-interactive commands it captures output inline.
// For empty ! it shows usage.
func (m *Model) handleShellEscape(line string) tea.Cmd {
	cmd := strings.TrimSpace(strings.TrimPrefix(line, "!"))
	if cmd == "" {
		return func() tea.Msg {
			return slashResultMsg{lines: []string{"usage: ! <shell command>"}}
		}
	}
	// Run the command; capture combined output. For truly interactive commands,
	// tea.ExecProcess hands the TTY to the subprocess and restores the TUI (REQ-CLIENT-0189).
	return m.shellRunCmd(cmd)
}

// shellRunCmd runs a shell command, capturing combined output inline.
// If the command needs a real TTY (interactive), callers may switch to tea.ExecProcess.
// For the current spec surface (inline output), we capture and display.
func (m *Model) shellRunCmd(shellCmd string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("sh", "-c", shellCmd)
		var buf bytes.Buffer
		c.Stdout = &buf
		c.Stderr = &buf
		err := c.Run()
		output := buf.String()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		return shellExecDoneMsg{output: output, exitCode: exitCode, err: err}
	}
}

// shellInteractiveCmd suspends the TUI and hands the real TTY to the subprocess (REQ-CLIENT-0189).
// Used when the command is explicitly interactive (e.g. a text editor).
func shellInteractiveCmd(shellCmd string) tea.Cmd {
	c := exec.Command("sh", "-c", shellCmd)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		return shellExecDoneMsg{err: err, exitCode: exitCode}
	})
}

// --- individual slash command implementations ---

func (m *Model) slashHelpCmd() tea.Cmd {
	return func() tea.Msg {
		lines := make([]string, 0, len(slashHelpCatalog)+1)
		lines = append(lines, "--- Slash Commands ---")
		for _, e := range slashHelpCatalog {
			lines = append(lines, fmt.Sprintf("  %-14s %s", e.name, e.desc))
		}
		return slashResultMsg{lines: lines}
	}
}

func (m *Model) slashClearCmd() tea.Cmd {
	return func() tea.Msg {
		// slashResultMsg with cleared=true; Update handles clearing scrollback.
		return slashResultMsg{lines: nil}
	}
}

func (m *Model) slashVersionCmd() tea.Cmd {
	return func() tea.Msg {
		return slashResultMsg{lines: []string{"cynork " + tuiVersion}}
	}
}

// tuiVersion is set at link time by the main binary; defaults to "dev".
// We expose it as a package-level variable so tests can override it.
var tuiVersion = "dev"

// SetTUIVersion allows the cmd layer to inject the build version at startup.
func SetTUIVersion(v string) { tuiVersion = v }

func (m *Model) slashModelCmd(rest string) tea.Cmd {
	return func() tea.Msg {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			model := "(default)"
			if m.Session != nil && m.Session.Model != "" {
				model = m.Session.Model
			}
			return slashResultMsg{lines: []string{"model: " + model}}
		}
		if m.Session != nil {
			m.Session.SetModel(rest)
		}
		return slashResultMsg{lines: []string{"model set to: " + rest}}
	}
}

func (m *Model) slashModelsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil || m.Session.Client == nil {
			return slashResultMsg{lines: []string{"Error: not connected"}}
		}
		resp, err := m.Session.Client.ListModels()
		if err != nil {
			return slashResultMsg{lines: []string{"Error: " + err.Error()}}
		}
		lines := make([]string, 0, len(resp.Data)+1)
		lines = append(lines, "--- Models ---")
		for _, mm := range resp.Data {
			lines = append(lines, "  "+mm.ID)
		}
		if len(resp.Data) == 0 {
			lines = append(lines, "  (no models)")
		}
		return slashResultMsg{lines: lines}
	}
}

func (m *Model) slashProjectCmd(rest string) tea.Cmd {
	return func() tea.Msg {
		return m.dispatchProjectCmd(strings.TrimSpace(rest))
	}
}

// slashAuthCmd handles /auth [whoami|logout|refresh|login]. whoami uses session client;
// logout/refresh use AuthProvider when set; login shows a hint (password required in terminal).
func (m *Model) slashAuthCmd(rest string) tea.Cmd {
	return func() tea.Msg {
		parts := strings.Fields(rest)
		sub := ""
		if len(parts) > 0 {
			sub = strings.ToLower(parts[0])
		}
		switch sub {
		case "":
			return slashResultMsg{lines: []string{
				"auth login    — log in (run 'cynork auth login' in a terminal)",
				"auth logout   — clear stored token",
				"auth whoami   — show current user",
				"auth refresh  — refresh access token",
			}}
		case "whoami":
			return m.authWhoami()
		case "logout":
			return m.authLogout()
		case "refresh":
			return m.authRefresh()
		case "login":
			return openLoginFormMsg{}
		default:
			return slashResultMsg{lines: []string{
				fmt.Sprintf("Unknown subcommand: /auth %s. Type /auth for usage.", sub),
			}}
		}
	}
}

func (m *Model) authWhoami() slashResultMsg {
	if m.Session == nil || m.Session.Client == nil {
		return slashResultMsg{lines: []string{"Error: not connected"}}
	}
	user, err := m.Session.Client.GetMe()
	if err != nil {
		return slashResultMsg{lines: []string{"Error: " + err.Error()}}
	}
	return slashResultMsg{lines: []string{fmt.Sprintf("id=%s user=%s", user.ID, user.Handle)}}
}

func (m *Model) authLogout() slashResultMsg {
	if m.AuthProvider == nil {
		return slashResultMsg{lines: []string{"auth logout: not available (no config in this session)"}}
	}
	m.AuthProvider.SetTokens("", "")
	if err := m.AuthProvider.Save(); err != nil {
		return slashResultMsg{lines: []string{"Error: " + err.Error()}}
	}
	if m.Session != nil && m.Session.Client != nil {
		m.Session.SetToken("")
	}
	return slashResultMsg{lines: []string{"logged_out=true"}}
}

func (m *Model) authRefresh() slashResultMsg {
	if m.AuthProvider == nil {
		return slashResultMsg{lines: []string{"auth refresh: not available (no config in this session)"}}
	}
	refreshToken := m.AuthProvider.RefreshToken()
	if refreshToken == "" {
		return slashResultMsg{lines: []string{"Error: no refresh token; run 'cynork auth login' first"}}
	}
	if m.Session == nil || m.Session.Client == nil {
		return slashResultMsg{lines: []string{"Error: not connected"}}
	}
	resp, err := m.Session.Client.Refresh(refreshToken)
	if err != nil {
		return slashResultMsg{lines: []string{"Error: " + err.Error()}}
	}
	newRefresh := resp.RefreshToken
	if newRefresh == "" {
		newRefresh = refreshToken
	}
	m.AuthProvider.SetTokens(resp.AccessToken, newRefresh)
	if err := m.AuthProvider.Save(); err != nil {
		return slashResultMsg{lines: []string{"Error saving config: " + err.Error()}}
	}
	m.Session.SetToken(resp.AccessToken)
	return slashResultMsg{lines: []string{"Token refreshed successfully."}}
}

// slashConnectCmd shows or updates the session gateway URL.
func (m *Model) slashConnectCmd(rest string) tea.Cmd {
	return func() tea.Msg {
		url := strings.TrimSpace(rest)
		if url == "" {
			return m.connectShow()
		}
		return m.connectSet(url)
	}
}

func (m *Model) connectShow() slashResultMsg {
	url := "(unknown)"
	if m.Session != nil && m.Session.Client != nil {
		url = m.Session.Client.BaseURL
	}
	return slashResultMsg{lines: []string{"gateway: " + url}}
}

func (m *Model) connectSet(url string) slashResultMsg {
	if m.Session != nil && m.Session.Client != nil {
		m.Session.Client.BaseURL = url
	}
	if m.AuthProvider != nil {
		m.AuthProvider.SetGatewayURL(url)
		_ = m.AuthProvider.Save()
	}
	if m.Session != nil && m.Session.Client != nil {
		if err := m.Session.Client.Health(); err != nil {
			return slashResultMsg{lines: []string{
				"gateway updated to: " + url,
				"Warning: health check failed: " + err.Error(),
			}}
		}
	}
	return slashResultMsg{lines: []string{"gateway updated to: " + url}}
}

// slashSetThinkingCmd toggles thinking visibility and persists the preference.
func (m *Model) slashSetThinkingCmd(show bool) tea.Cmd {
	return func() tea.Msg {
		m.ShowThinking = show
		if m.AuthProvider != nil {
			m.AuthProvider.SetShowThinkingByDefault(show)
			if err := m.AuthProvider.Save(); err != nil {
				return slashResultMsg{lines: []string{
					fmt.Sprintf("thinking %s (warning: config save failed: %v)", thinkingLabel(show), err),
				}}
			}
		}
		return slashResultMsg{lines: []string{
			fmt.Sprintf("thinking: %s", thinkingLabel(show)),
		}}
	}
}

func thinkingLabel(show bool) string {
	if show {
		return "visible"
	}
	return "hidden"
}

// slashStatusCmd checks gateway reachability and returns a scrollback line.
func (m *Model) slashStatusCmd() tea.Cmd {
	return func() tea.Msg {
		if m.Session == nil || m.Session.Client == nil {
			return slashResultMsg{lines: []string{"status: not connected"}}
		}
		if err := m.Session.Client.Health(); err != nil {
			return slashResultMsg{lines: []string{"status: unreachable — " + err.Error()}}
		}
		return slashResultMsg{lines: []string{"status: ok — " + m.Session.Client.BaseURL}}
	}
}

func (m *Model) dispatchProjectCmd(rest string) slashResultMsg {
	parts := strings.Fields(rest)
	sub := ""
	if len(parts) > 0 {
		sub = strings.ToLower(parts[0])
	}
	switch sub {
	case "set":
		return m.projectSetCmd(parts)
	case "list":
		return slashResultMsg{lines: []string{"project list: not yet supported (stub)"}}
	case "get":
		if len(parts) < 2 {
			return slashResultMsg{lines: []string{"Usage: /project get <project_id>"}}
		}
		return slashResultMsg{lines: []string{"project get: not yet supported (stub)"}}
	case "":
		project := "(none)"
		if m.Session != nil && m.Session.ProjectID != "" {
			project = m.Session.ProjectID
		}
		return slashResultMsg{lines: []string{"project: " + project}}
	default:
		if m.Session != nil {
			m.Session.SetProjectID(rest)
		}
		return slashResultMsg{lines: []string{"project set to: " + rest}}
	}
}

func (m *Model) projectSetCmd(parts []string) slashResultMsg {
	if len(parts) < 2 {
		return slashResultMsg{lines: []string{"Usage: /project set <project_id>"}}
	}
	id := parts[1]
	if id == "none" {
		id = ""
	}
	if m.Session != nil {
		m.Session.SetProjectID(id)
	}
	if id == "" {
		return slashResultMsg{lines: []string{"project context cleared"}}
	}
	return slashResultMsg{lines: []string{"project set to: " + id}}
}

// captureToLines runs fn with os.Stdout and os.Stderr redirected to a buffer,
// then splits the combined output into lines. This lets slash commands that
// print to os.Stderr/Stdout be captured for TUI scrollback display.
func captureToLines(fn func()) []string {
	r, w, err := os.Pipe()
	if err != nil {
		fn()
		return nil
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	raw := strings.TrimRight(buf.String(), "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
