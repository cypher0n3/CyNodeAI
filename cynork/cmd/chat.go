package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat with the Project Manager (POST /v1/chat)",
	Long:  "Reads lines from stdin; /exit or /quit or EOF exits. Each message is sent via POST /v1/chat. No token yields exit 3.",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func runChat(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "> ")
		if !scanner.Scan() {
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			return nil
		}
		resp, err := client.Chat(line)
		if err != nil {
			return exitFromGatewayErr(err)
		}
		if resp.Response != "" {
			fmt.Println(resp.Response)
		}
	}
}
