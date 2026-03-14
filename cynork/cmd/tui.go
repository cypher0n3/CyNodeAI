// TUI command: full-screen chat interface. See docs/tech_specs/cynork_tui.md.
package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/cynork/internal/tui"
	"github.com/spf13/cobra"
)

var tuiProjectID string
var tuiThreadNew bool

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Full-screen TUI for chat and thread management",
	Long:  "Starts the full-screen TUI. Use for interactive chat, thread list/switch/rename, and project/model context. See docs/tech_specs/cynork_tui.md.",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	tuiCmd.Flags().StringVar(&tuiProjectID, "project-id", "", "project to associate with the session (OpenAI-Project header)")
	tuiCmd.Flags().BoolVar(&tuiThreadNew, "thread-new", false, "create a new thread before starting the TUI")
}

func runTUI(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	session := chat.NewSession(client)
	session.ProjectID = tuiProjectID
	session.Plain = false
	session.NoColor = noColor
	if tuiThreadNew {
		threadID, err := session.NewThread()
		if err != nil {
			return fmt.Errorf("start new thread: %w", err)
		}
		fmt.Fprintf(os.Stderr, "New thread started: %s\n", threadID)
	}
	return runTUIWithSession(session)
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
func (p *tuiAuthProvider) SetGatewayURL(url string) { p.cfg.GatewayURL = url }
func (p *tuiAuthProvider) Save() error              { return p.saveFn() }

// runTUIWithSession starts the full-screen TUI with the given session. Used by both
// "cynork tui" and interactive "cynork chat" (when stdin/stdout are a TTY).
func runTUIWithSession(session *chat.Session) error {
	tui.SetTUIVersion(version)
	m := tui.NewModel(session)
	m.SetAuthProvider(&tuiAuthProvider{cfg: cfg, saveFn: saveConfig})
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
