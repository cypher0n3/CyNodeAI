package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/peterh/liner"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var chatPlain bool
var chatMessage string
var chatProjectID string // --project-id flag (initial project for session)

// chatSessionModel and chatSessionProjectID are the in-session model and project (set via /model, /project).
var chatSessionModel string
var chatSessionProjectID string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat with the Project Manager (POST /v1/chat/completions)",
	Long:  "Reads lines from stdin; /exit or /quit or EOF exits. Each message is sent via POST /v1/chat/completions (OpenAI format). Use --message for one-shot (send one message and print response). No token yields exit 3. Use --plain for raw output (no Markdown rendering).",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVar(&chatPlain, "plain", false, "print model responses as raw text without Markdown formatting (for scripting or piping)")
	chatCmd.Flags().StringVarP(&chatMessage, "message", "m", "", "send one message and print response (non-interactive)")
	chatCmd.Flags().StringVar(&chatProjectID, "project-id", "", "project to associate with chat session (sent as OpenAI-Project header)")
}

// formatChatResponseFn is the implementation of formatChatResponse; tests may replace it to trigger error path.
var formatChatResponseFn = formatChatResponse

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
	chatSessionModel = ""
	chatSessionProjectID = chatProjectID
	if chatMessage != "" {
		return sendAndPrintChat(client, chatMessage)
	}
	// Discoverability: show slash commands at session start (spec CliChatSlashCommands).
	fmt.Fprintln(os.Stderr, "Slash commands: /help for list.")
	printSlashHelp()
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		return runChatLoopLiner(client)
	}
	return runChatLoopScanner(client)
}

// chatLineReader is set by tests to inject a line source; nil means use liner/scanner.
var chatLineReader func(prompt string) (string, error)

// chatLinerGetLine is set by tests to use an injected getLine in runChatLoopLiner instead of creating liner state.
var chatLinerGetLine func(prompt string) (string, error)

func runChatLoopLiner(client *gateway.Client) error {
	if chatLinerGetLine != nil {
		return runChatLoopWithReader(client, "> ", chatLinerGetLine)
	}
	if chatLineReader != nil {
		return runChatLoopWithReader(client, "> ", chatLineReader)
	}
	state := liner.NewLiner()
	defer func() { _ = state.Close() }()
	state.SetCompleter(slashCompleter)
	state.SetCtrlCAborts(true)
	return runChatLoopWithReader(client, "> ", func(prompt string) (string, error) {
		line, err := state.Prompt(prompt)
		if err == liner.ErrPromptAborted {
			return "", liner.ErrPromptAborted
		}
		return line, err
	})
}

// runChatLoopWithReader runs the chat loop using getLine for input; getLine returns ("", err) on EOF/abort.
func runChatLoopWithReader(client *gateway.Client, prompt string, getLine func(string) (string, error)) error {
	for {
		line, err := getLine(prompt)
		if err != nil {
			if err == liner.ErrPromptAborted || err == io.EOF {
				return nil
			}
			return err
		}
		exitSession, err := processChatLine(client, strings.TrimSpace(line))
		if err != nil {
			return err
		}
		if exitSession {
			return nil
		}
	}
}

// processChatLine handles one line of input; returns (exitSession, err). Empty line is no-op and returns (false, nil).
func processChatLine(client *gateway.Client, line string) (bool, error) {
	if line == "" {
		return false, nil
	}
	if strings.HasPrefix(line, "/") {
		return runSlashCommand(client, line)
	}
	return false, sendAndPrintChat(client, line)
}

func slashCompleter(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "/") {
		return nil
	}
	prefix := strings.ToLower(line)
	var out []string
	for _, c := range AllSlashCommands() {
		if strings.HasPrefix(strings.ToLower(c.Name), prefix) {
			out = append(out, c.Name)
		}
	}
	return out
}

func runChatLoopScanner(client *gateway.Client) error {
	scanner := bufio.NewScanner(os.Stdin)
	getLine := func(prompt string) (string, error) {
		fmt.Fprint(os.Stderr, prompt)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", err
			}
			return "", io.EOF
		}
		return scanner.Text(), nil
	}
	return runChatLoopWithReader(client, "> ", getLine)
}

// sendAndPrintChat sends the line to the gateway and prints the response (formatted or raw).
func sendAndPrintChat(client *gateway.Client, line string) error {
	resp, err := client.ChatWithOptions(line, chatSessionModel, chatSessionProjectID)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if resp.Response == "" {
		return nil
	}
	out, err := formatChatResponseFn(resp.Response, chatPlain, noColor)
	if err != nil {
		fmt.Println(resp.Response)
		return nil
	}
	fmt.Print(out)
	return nil
}
