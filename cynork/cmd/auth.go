package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var (
	authLoginHandle   string
	authLoginPassword string
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

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authWhoamiCmd)
	authLoginCmd.Flags().StringVarP(&authLoginHandle, "username", "u", "", "username (handle)")
	authLoginCmd.Flags().StringVarP(&authLoginPassword, "password", "p", "", "password")
}

func runAuthLogin(_ *cobra.Command, _ []string) error {
	handle := authLoginHandle
	password := authLoginPassword
	if handle == "" {
		fmt.Print("Username: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return scanner.Err()
		}
		handle = strings.TrimSpace(scanner.Text())
	}
	if password == "" {
		var err error
		password, err = readPassword("Password: ")
		if err != nil {
			return err
		}
	}
	client := gateway.NewClient(cfg.GatewayURL)
	resp, err := client.Login(gateway.LoginRequest{Handle: handle, Password: password})
	if err != nil {
		return exitFromGatewayErr(err)
	}
	cfg.Token = resp.AccessToken
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
	fmt.Println("Logged in successfully.")
	return nil
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	// MVP: read line without terminal echo; fallback to ScanLn if no syscall support
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	return "", scanner.Err()
}

func runAuthLogout(_ *cobra.Command, _ []string) error {
	cfg.Token = ""
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
	fmt.Println("Logged out.")
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
	fmt.Printf("id=%s handle=%s\n", user.ID, user.Handle)
	return nil
}
