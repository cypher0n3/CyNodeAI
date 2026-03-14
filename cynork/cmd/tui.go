// TUI command: full-screen chat interface. See docs/tech_specs/cynork_tui.md.
package cmd

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/cynork/internal/tui"
	"github.com/spf13/cobra"
)

var tuiProjectID string
var tuiResumeThread string

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Full-screen TUI for chat and thread management",
	Long:  "Starts the full-screen TUI. By default starts with a new thread; use --resume-thread <selector> to start in an existing thread. See docs/tech_specs/cynork_tui.md.",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	tuiCmd.Flags().StringVar(&tuiProjectID, "project-id", "", "project to associate with the session (OpenAI-Project header)")
	tuiCmd.Flags().StringVar(&tuiResumeThread, "resume-thread", "", "start in an existing thread (selector: ordinal, id, or title from /thread list)")
}

func runTUI(_ *cobra.Command, _ []string) error {
	client := gateway.NewClient(cfg.GatewayURL)
	if cfg.Token != "" {
		client.SetToken(cfg.Token)
	}
	session := chat.NewSession(client)
	session.ProjectID = tuiProjectID
	session.Plain = false
	session.NoColor = noColor
	// When token present, ensure thread before TUI: new (default) or resolve --resume-thread.
	if cfg.Token != "" {
		if err := session.EnsureThread(tuiResumeThread); err != nil {
			return fmt.Errorf("thread: %w", err)
		}
	}
	return runTUIWithSession(session, tuiResumeThread)
}

// tuiAuthProvider implements tui.AuthProvider so /auth login, logout, refresh can persist tokens and gateway URL.
type tuiAuthProvider struct {
	cfg    *config.Config
	saveFn func() error
}

func (p *tuiAuthProvider) Token() string        { return p.cfg.Token }
func (p *tuiAuthProvider) RefreshToken() string { return p.cfg.RefreshToken }
func (p *tuiAuthProvider) GatewayURL() string   { return p.cfg.GatewayURL }
func (p *tuiAuthProvider) SetTokens(access, refresh string) {
	p.cfg.Token, p.cfg.RefreshToken = access, refresh
}
func (p *tuiAuthProvider) SetGatewayURL(url string)    { p.cfg.GatewayURL = url }
func (p *tuiAuthProvider) Save() error                 { return p.saveFn() }
func (p *tuiAuthProvider) ShowThinkingByDefault() bool { return p.cfg.TUI.ShowThinkingByDefault }
func (p *tuiAuthProvider) SetShowThinkingByDefault(v bool) {
	p.cfg.TUI.ShowThinkingByDefault = v
}

// runTUIWithSession starts the full-screen TUI with the given session. Used by both
// "cynork tui" and interactive "cynork chat" (when stdin/stdout are a TTY).
// resumeThreadSelector is passed so that after in-session login the TUI can ensure thread (new or resume).
func runTUIWithSession(session *chat.Session, resumeThreadSelector string) error {
	tui.SetTUIVersion(version)
	m := tui.NewModel(session)
	m.SetAuthProvider(&tuiAuthProvider{cfg: cfg, saveFn: saveConfig})
	m.SetResumeThreadSelector(resumeThreadSelector)
	// Startup token failure: open in-session login instead of exiting (cynork_tui.md Auth Recovery).
	if session.Client != nil && session.Client.Token == "" {
		m.OpenLoginFormOnInit = true
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := tuiRunProgram(p); err != nil {
		return err
	}
	return nil
}

// tuiRunProgram runs the Tea program; tests may override to avoid blocking.
var tuiRunProgram = func(p *tea.Program) (tea.Model, error) {
	return p.Run()
}
