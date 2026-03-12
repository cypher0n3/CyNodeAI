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
	Short: "Log in and store token",
	RunE:  runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored token",
	RunE:  runAuthLogout,
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user from token",
	RunE:  runAuthWhoami,
}

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh access token using stored refresh token",
	Long:  "Calls POST /v1/auth/refresh with the refresh token saved at login and updates the stored access token.",
	RunE:  runAuthRefresh,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authWhoamiCmd, authRefreshCmd)
	authLoginCmd.Flags().StringVarP(&authLoginHandle, "user", "u", "", "login user handle")
	authLoginCmd.Flags().StringVar(&authLoginHandle, "handle", "", "login handle (alias of --user)")
	authLoginCmd.Flags().BoolVar(&authLoginPasswordStdin, "password-stdin", false, "read password from stdin")
}

func runAuthLogin(_ *cobra.Command, _ []string) error {
	handle, password, err := gatherLoginCredentials()
	if err != nil {
		return err
	}
	client := gateway.NewClient(cfg.GatewayURL)
	resp, err := client.Login(userapi.LoginRequest{Handle: handle, Password: password})
	if err != nil {
		return exitFromGatewayErr(err)
	}
	cfg.Token = resp.AccessToken
	cfg.RefreshToken = resp.RefreshToken
	if err := saveConfig(); err != nil {
		return err
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
	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("save token: %w", err)
	}
	return nil
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

func runAuthLogout(_ *cobra.Command, _ []string) error {
	cfg.Token = ""
	cfg.RefreshToken = ""
	path := configPath
	if path == "" {
		var err error
		path, err = getDefaultConfigPath()
		if err != nil {
			return fmt.Errorf("config path: %w", err)
		}
	}
	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(map[string]any{"logged_out": true})
	} else {
		fmt.Println("logged_out=true")
	}
	return nil
}

func runAuthWhoami(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	user, err := client.GetMe()
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

func runAuthRefresh(_ *cobra.Command, _ []string) error {
	if cfg.RefreshToken == "" {
		return exit.Auth(fmt.Errorf("no refresh token: run 'cynork auth login' first"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	resp, err := client.Refresh(cfg.RefreshToken)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	cfg.Token = resp.AccessToken
	cfg.RefreshToken = resp.RefreshToken
	path := configPath
	if path == "" {
		var err error
		path, err = getDefaultConfigPath()
		if err != nil {
			return fmt.Errorf("config path: %w", err)
		}
	}
	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("save token: %w", err)
	}
	fmt.Println("Token refreshed successfully.")
	return nil
}
