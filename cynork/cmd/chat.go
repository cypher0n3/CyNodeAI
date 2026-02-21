package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var chatPlain bool

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat with the Project Manager (POST /v1/chat)",
	Long:  "Reads lines from stdin; /exit or /quit or EOF exits. Each message is sent via POST /v1/chat. No token yields exit 3. Use --plain for raw output (no Markdown rendering).",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVar(&chatPlain, "plain", false, "print model responses as raw text without Markdown formatting (for scripting or piping)")
}

// formatChatResponse returns the response ready to print: raw if plain, else Markdown-rendered for terminal.
// When rendering fails, the second return is non-nil; caller may fall back to printing raw.
func formatChatResponse(response string, plain, noColor bool) (string, error) {
	if plain {
		return response + "\n", nil
	}
	opts := []glamour.TermRendererOption{}
	if noColor {
		opts = append(opts, glamour.WithStandardStyle(styles.AsciiStyle))
	} else {
		opts = append(opts, glamour.WithAutoStyle())
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return "", err
	}
	out, err := r.Render(response)
	if err != nil {
		return "", err
	}
	return out, nil
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
		if err := sendAndPrintChat(client, line); err != nil {
			return err
		}
	}
}

// sendAndPrintChat sends the line to the gateway and prints the response (formatted or raw).
func sendAndPrintChat(client *gateway.Client, line string) error {
	resp, err := client.Chat(line)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if resp.Response == "" {
		return nil
	}
	out, err := formatChatResponse(resp.Response, chatPlain, noColor)
	if err != nil {
		fmt.Println(resp.Response)
		return nil
	}
	fmt.Print(out)
	return nil
}
