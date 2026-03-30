package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/peterh/liner"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var chatPlain bool
var chatMessage string
var chatModel string        // --model flag (model ID for chat completions)
var chatProjectID string    // --project-id flag (initial project for session)
var chatThreadNew bool      // --thread-new flag: create a fresh thread before sending/opening session
var chatResumeThread string // --resume-thread flag: resolve selector and resume existing thread

var chatCmd = &cobra.Command{
	Use:          "chat",
	Short:        "Interactive chat with the Project Manager (POST /v1/chat/completions)",
	Long:         "Reads lines from stdin; /exit or /quit or EOF exits. Each message is sent via POST /v1/chat/completions (OpenAI format). Use --message for one-shot (send one message and print response). No token yields exit 3. Use --plain for raw output (no Markdown rendering).",
	RunE:         runChat,
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVar(&chatPlain, "plain", false, "print model responses as raw text without Markdown formatting (for scripting or piping)")
	chatCmd.Flags().StringVarP(&chatMessage, "message", "m", "", "send one message and print response (non-interactive)")
	chatCmd.Flags().StringVar(&chatModel, "model", "", "model ID for chat completions (sent as OpenAI model field; gateway default if omitted)")
	chatCmd.Flags().StringVar(&chatProjectID, "project-id", "", "project to associate with chat session (sent as OpenAI-Project header)")
	chatCmd.Flags().BoolVar(&chatThreadNew, "thread-new", false, "start a new conversation thread before sending the first message")
	chatCmd.Flags().StringVar(&chatResumeThread, "resume-thread", "", "start in an existing thread (selector: ordinal, id, or title from /thread list)")
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

func runChat(cmd *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	ctx := cmdContext(cmd)
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	session := chat.NewSession(client)
	session.Model = chatModel
	session.ProjectID = chatProjectID
	session.Plain = chatPlain
	session.NoColor = noColor
	switch {
	case chatResumeThread != "":
		if err := session.EnsureThread(ctx, chatResumeThread); err != nil {
			return fmt.Errorf("resume thread: %w", err)
		}
	case chatThreadNew:
		threadID, err := session.NewThread(ctx)
		if err != nil {
			return fmt.Errorf("start new thread: %w", err)
		}
		fmt.Fprintf(os.Stderr, "New thread started: %s\n", threadID)
	}
	if chatMessage != "" {
		return sendAndPrintChat(session, chatMessage)
	}
	// Discoverability: show slash commands and shell escape at session start (spec CliChatSlashCommands, CliChatShellEscape).
	fmt.Fprintln(os.Stderr, "Slash commands: /help for list. ! <cmd> run in shell.")
	printSlashHelp()
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		return runChatLoopLiner(session)
	}
	return runChatLoopScanner(session)
}

// chatLineReader is set by tests to inject a line source; nil means use liner/scanner.
var chatLineReader func(prompt string) (string, error)

// chatLinerGetLine is set by tests to use an injected getLine in runChatLoopLiner instead of creating liner state.
var chatLinerGetLine func(prompt string) (string, error)

func runChatLoopLiner(session *chat.Session) error {
	if chatLinerGetLine != nil {
		return runChatLoopWithReader(session, "> ", chatLinerGetLine)
	}
	if chatLineReader != nil {
		return runChatLoopWithReader(session, "> ", chatLineReader)
	}
	state := liner.NewLiner()
	defer func() { _ = state.Close() }()
	state.SetCompleter(slashCompleter)
	state.SetCtrlCAborts(true)
	return runChatLoopWithReader(session, "> ", func(prompt string) (string, error) {
		line, err := state.Prompt(prompt)
		if err == liner.ErrPromptAborted {
			return "", liner.ErrPromptAborted
		}
		return line, err
	})
}

// runChatLoopWithReader runs the chat loop using getLine for input; getLine returns ("", err) on EOF/abort.
func runChatLoopWithReader(session *chat.Session, prompt string, getLine func(string) (string, error)) error {
	for {
		line, err := getLine(prompt)
		if err != nil {
			if err == liner.ErrPromptAborted || err == io.EOF {
				return nil
			}
			return err
		}
		exitSession, err := processChatLine(session, strings.TrimSpace(line))
		if err != nil {
			return err
		}
		if exitSession {
			return nil
		}
	}
}

// processChatLine handles one line of input; returns (exitSession, err). Empty line is no-op and returns (false, nil).
// Slash, shell-escape, and chat message gateway errors are printed to stderr and do not exit the session
// (spec CliChatSubcommandErrors). Only session-exit actions return exitSession=true.
func processChatLine(session *chat.Session, line string) (bool, error) {
	if line == "" {
		return false, nil
	}
	if strings.HasPrefix(line, "!") {
		runChatShellCommand(strings.TrimSpace(line[1:]))
		return false, nil
	}
	if strings.HasPrefix(line, "/") {
		exitSession, err := runSlashCommand(session, line)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return false, nil
		}
		return exitSession, nil
	}
	if strings.HasPrefix(line, "@") {
		path := strings.TrimSpace(line[1:])
		if path == "" {
			fmt.Fprintln(os.Stderr, "error: @ requires a file path")
			return false, nil
		}
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			fmt.Fprintf(os.Stderr, "error: file not found: %s\n", path)
			return false, nil
		}
	}
	if err := sendAndPrintChat(session, line); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return false, nil
}

// runChatShellCommand runs cmd in the shell and prints stdout/stderr inline. Never returns an error (spec CliChatShellEscape).
func runChatShellCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		fmt.Fprintln(os.Stderr, "usage: ! <shell command>")
		return
	}
	c := exec.Command("sh", "-c", cmd)
	out, err := c.CombinedOutput()
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "exit status %d\n", exitErr.ExitCode())
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
	}
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

func runChatLoopScanner(session *chat.Session) error {
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
	return runChatLoopWithReader(session, "> ", getLine)
}

// sendAndPrintChat sends the line via the session transport and prints the visible text (formatted or raw).
func sendAndPrintChat(session *chat.Session, line string) error {
	turn, err := session.SendMessage(context.Background(), line)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if turn == nil || turn.VisibleText == "" {
		return nil
	}
	out, err := formatChatResponseFn(turn.VisibleText, session.Plain, session.NoColor)
	if err != nil {
		fmt.Println(turn.VisibleText)
		return nil
	}
	fmt.Print(out)
	return nil
}
