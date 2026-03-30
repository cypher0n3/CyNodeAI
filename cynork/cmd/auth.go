package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/secretutil"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	authLoginHandle        string
	authLoginPasswordStdin bool
)

// authCmd represents the auth command group.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication (login, logout, whoami)",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in (session stored in OS keyring or XDG cache, not config.yaml)",
	RunE:  runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored session and in-memory token state",
	RunE:  runAuthLogout,
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user from token",
	RunE:  runAuthWhoami,
}

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh access token using stored session, CYNORK_REFRESH_TOKEN, or in-memory refresh token",
	Long:  "Calls POST /v1/auth/refresh. Uses the persisted refresh token after login, or CYNORK_REFRESH_TOKEN, or the current process session.",
	RunE:  runAuthRefresh,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authWhoamiCmd, authRefreshCmd)
	authLoginCmd.Flags().StringVarP(&authLoginHandle, "user", "u", "", "login user handle")
	authLoginCmd.Flags().StringVar(&authLoginHandle, "handle", "", "login handle (alias of --user)")
	authLoginCmd.Flags().BoolVar(&authLoginPasswordStdin, "password-stdin", false, "read password from stdin")
}

func runAuthLogin(cmd *cobra.Command, _ []string) error {
	handle, password, err := gatherLoginCredentials()
	if err != nil {
		return err
	}
	client := gateway.NewClient(cfg.GatewayURL)
	resp, err := client.Login(cmdContext(cmd), userapi.LoginRequest{Handle: handle, Password: password})
	if err != nil {
		return exitFromGatewayErr(err)
	}
	secretutil.RunWithSecret(func() {
		cfg.Token = resp.AccessToken
		cfg.RefreshToken = resp.RefreshToken
	})
	if err := config.PersistSession(cfg.Token, cfg.RefreshToken); err != nil {
		return exit.Usage(fmt.Errorf("persist session: %w", err))
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(map[string]any{"logged_in": true, "user": handle})
	} else {
		fmt.Printf("logged_in=true user=%s\n", handle)
	}
	return nil
}

func gatherLoginCredentials() (handle, password string, err error) {
	handle = authLoginHandle
	if authLoginPasswordStdin && strings.TrimSpace(handle) == "" {
		return "", "", exit.Usage(fmt.Errorf("--password-stdin requires --user"))
	}
	if handle == "" {
		handle, err = readPromptLine("User: ")
		if err != nil {
			return "", "", err
		}
	}
	if authLoginPasswordStdin {
		password, err = readPasswordFromStdin()
	} else {
		password, err = readPassword("Password: ")
	}
	return handle, password, err
}

func saveConfig() error {
	path := configPath
	if path == "" {
		var err error
		path, err = getDefaultConfigPath()
		if err != nil {
			return fmt.Errorf("config path: %w", err)
		}
	}
	toSave := cfg
	switch {
	case cfgGatewayPersistExplicit:
		// User ran /connect <url>; persist in-memory gateway_url as-is.
	case fileExists(path):
		// Never replace gateway_url in an existing file from in-memory drift (env,
		// login form, etc.); only TUI prefs and other fields update.
		base, err := config.LoadFileWithoutEnvOverrides(path)
		if err != nil {
			return fmt.Errorf("read file-backed config for save: %w", err)
		}
		merged := *toSave
		merged.GatewayURL = base.GatewayURL
		toSave = &merged
	case cfgGatewayFromEnv:
		// No file yet: do not persist CYNORK_GATEWAY_URL on first write.
		base, err := config.LoadFileWithoutEnvOverrides(path)
		if err != nil {
			return fmt.Errorf("read file-backed config for save: %w", err)
		}
		merged := *toSave
		merged.GatewayURL = base.GatewayURL
		toSave = &merged
	}
	if err := config.Save(path, toSave); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readPromptLine(prompt string) (string, error) {
	_, _ = fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	return strings.TrimSpace(scanner.Text()), nil
}

func readPassword(prompt string) (string, error) {
	_, _ = fmt.Fprint(os.Stderr, prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		password, err := term.ReadPassword(fd)
		_, _ = fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(password), nil
	}
	return readPasswordFromStdin()
}

func readPasswordFromStdin() (string, error) {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	text := string(raw)
	if strings.HasSuffix(text, "\r\n") {
		return strings.TrimSuffix(text, "\r\n"), nil
	}
	if strings.HasSuffix(text, "\n") {
		return strings.TrimSuffix(text, "\n"), nil
	}
	return text, nil
}

// persistSessionAndConfig writes the session store and non-secret config.yaml (gateway, TUI prefs).
func persistSessionAndConfig() error {
	if err := config.PersistSession(cfg.Token, cfg.RefreshToken); err != nil {
		return fmt.Errorf("persist session: %w", err)
	}
	return saveConfig()
}

func runAuthLogout(_ *cobra.Command, _ []string) error {
	secretutil.RunWithSecret(func() {
		cfg.Token = ""
		cfg.RefreshToken = ""
	})
	if err := config.PersistSession("", ""); err != nil {
		return exit.Usage(fmt.Errorf("clear session: %w", err))
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(map[string]any{"logged_out": true})
	} else {
		fmt.Println("logged_out=true")
	}
	return nil
}

func runAuthWhoami(cmd *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login' or set CYNORK_TOKEN"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	user, err := client.GetMe(cmdContext(cmd))
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(map[string]any{"id": user.ID, "user": user.Handle})
	} else {
		fmt.Printf("id=%s user=%s\n", user.ID, user.Handle)
	}
	return nil
}

func runAuthRefresh(cmd *cobra.Command, _ []string) error {
	if cfg.RefreshToken == "" {
		return exit.Auth(fmt.Errorf("no refresh token: run 'cynork auth login', or set CYNORK_REFRESH_TOKEN"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	resp, err := client.Refresh(cmdContext(cmd), cfg.RefreshToken)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	secretutil.RunWithSecret(func() {
		cfg.Token = resp.AccessToken
		cfg.RefreshToken = resp.RefreshToken
	})
	if err := config.PersistSession(cfg.Token, cfg.RefreshToken); err != nil {
		return exit.Usage(fmt.Errorf("persist session: %w", err))
	}
	fmt.Println("Token refreshed successfully.")
	return nil
}
