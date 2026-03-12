package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// SlashCommand describes one slash command for help and autocomplete.
type SlashCommand struct {
	Name        string
	Description string
}

// AllSlashCommands returns every slash command with short description (spec: discoverability), in alphabetical order.
func AllSlashCommands() []SlashCommand {
	return []SlashCommand{
		{"/auth", "auth login, logout, whoami, refresh"},
		{"/clear", "clear terminal display"},
		{"/exit", "end chat session"},
		{"/help", "list slash commands"},
		{"/model", "show or set session model"},
		{"/models", "list available models"},
		{"/nodes", "nodes list, get"},
		{"/prefs", "preferences list, get, set, delete, effective"},
		{"/project", "show or set project context"},
		{"/quit", "end chat session"},
		{"/skills", "skills list, get"},
		{"/status", "gateway reachability"},
		{"/task", "task list, get, create, cancel, result, logs, artifacts"},
		{"/thread", "thread new — start a fresh conversation thread"},
		{"/version", "print cynork version"},
		{"/whoami", "current identity"},
	}
}

// parseSlash splits line into command (lowercase) and rest; ok is true if line starts with /.
func parseSlash(line string) (cmd, rest string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "/") {
		return "", "", false
	}
	line = strings.TrimSpace(line[1:])
	if line == "" {
		return "", "", true
	}
	idx := strings.Index(line, " ")
	if idx < 0 {
		return strings.ToLower(line), "", true
	}
	return strings.ToLower(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

type slashHandler func(*gateway.Client, string) (bool, error)

var slashHandlers = map[string]slashHandler{
	"exit":    func(*gateway.Client, string) (bool, error) { return true, nil },
	"quit":    func(*gateway.Client, string) (bool, error) { return true, nil },
	"help":    func(*gateway.Client, string) (bool, error) { printSlashHelp(); return false, nil },
	"clear":   func(*gateway.Client, string) (bool, error) { clearTerminal(); return false, nil },
	"version": func(*gateway.Client, string) (bool, error) { fmt.Println("cynork", version); return false, nil },
	"models": func(_ *gateway.Client, rest string) (bool, error) {
		r := strings.TrimSpace(rest)
		if r == "" {
			r = "list"
		}
		return false, runCynorkSubcommandForSlash("models", r)
	},
	"model":   func(c *gateway.Client, rest string) (bool, error) { return false, runSlashModel(c, rest) },
	"project": func(c *gateway.Client, rest string) (bool, error) { return false, runSlashProjectDelegated(c, rest) },
	"task": func(_ *gateway.Client, rest string) (bool, error) {
		return false, runCynorkSubcommandForSlash("task", rest)
	},
	"status": func(_ *gateway.Client, rest string) (bool, error) {
		return false, runCynorkSubcommandForSlash("status", rest)
	},
	"whoami": func(_ *gateway.Client, rest string) (bool, error) {
		return false, runCynorkSubcommandForSlash("auth", "whoami")
	},
	"auth": func(c *gateway.Client, rest string) (bool, error) { return false, runSlashAuthDelegated(c, rest) },
	"nodes": func(_ *gateway.Client, rest string) (bool, error) {
		return false, runCynorkSubcommandForSlash("nodes", rest)
	},
	"prefs": func(_ *gateway.Client, rest string) (bool, error) {
		return false, runCynorkSubcommandForSlash("prefs", rest)
	},
	"skills": func(_ *gateway.Client, rest string) (bool, error) {
		return false, runCynorkSubcommandForSlash("skills", rest)
	},
	"thread": func(c *gateway.Client, rest string) (bool, error) {
		return false, runSlashThread(c, rest)
	},
}

// runSlashCommand executes a slash command. Returns (exitSession, err). exitSession true means chat should exit.
func runSlashCommand(client *gateway.Client, line string) (exitSession bool, err error) {
	cmd, rest, ok := parseSlash(line)
	if !ok {
		return false, nil
	}
	if h, ok := slashHandlers[cmd]; ok {
		return h(client, rest)
	}
	fmt.Fprintln(os.Stderr, "Unknown command. Type /help for available commands.")
	return false, nil
}

func printSlashHelp() {
	for _, c := range AllSlashCommands() {
		fmt.Fprintf(os.Stderr, "  %-12s %s\n", c.Name, c.Description)
	}
}

// runCynorkSubcommandForSlash is the function used to run a subcommand from a slash command.
// Tests may set this to runCynorkSubcommandInProcess so the test binary doesn't exec itself.
var runCynorkSubcommandForSlash = runCynorkSubcommand

// runCynorkSubcommandInProcess runs the subcommand in-process via cobra (for tests; avoids exec).
// On error it prints to stderr and returns nil so behavior matches runCynorkSubcommand (child prints and exits).
func runCynorkSubcommandInProcess(subcommand, rest string) error {
	args := parseArgs(rest)
	rootCmd.SetArgs(append([]string{subcommand}, args...))
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	return nil
}

// getCynorkExeForSubcommand returns the executable to run for delegated slash commands. Tests may override.
var getCynorkExeForSubcommand = os.Executable

// runCynorkSubcommand runs the cynork binary with the given subcommand and rest as args (e.g. "task", "create --help").
// This ensures slash commands use the same code paths and flags as the CLI (e.g. /task create --help shows help).
func runCynorkSubcommand(subcommand, rest string) error {
	exe, err := getCynorkExeForSubcommand()
	if err != nil {
		return fmt.Errorf("cynork subcommand: %w", err)
	}
	effectiveConfig := configPath
	if effectiveConfig == "" {
		effectiveConfig, _ = getDefaultConfigPath()
	}
	args := make([]string, 0, 4+8)
	if effectiveConfig != "" {
		args = append(args, "--config", effectiveConfig)
	}
	args = append(args, subcommand)
	args = append(args, parseArgs(rest)...)
	cmd := exec.Command(exe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// ExitError already printed by child; return nil so chat doesn't print "exit status 1" again.
			_ = exitErr
			return nil
		}
		return err
	}
	return nil
}

func clearTerminal() {
	// ANSI clear screen; if not a TTY, do nothing (spec: MAY print message that clearing is not available)
	if os.Stdout == nil {
		return
	}
	_, _ = fmt.Fprint(os.Stdout, "\033[H\033[2J")
}

// runSlashAuthDelegated runs "cynork auth <rest>" then syncs chat client token on login/refresh/logout.
func runSlashAuthDelegated(chatClient *gateway.Client, rest string) error {
	if err := runCynorkSubcommandForSlash("auth", rest); err != nil {
		return err
	}
	rest = strings.TrimSpace(rest)
	parts := parseArgs(rest)
	if len(parts) == 0 {
		return nil
	}
	sub := strings.ToLower(parts[0])
	if chatClient == nil {
		return nil
	}
	switch sub {
	case "login", "refresh":
		effectivePath := configPath
		if effectivePath == "" {
			effectivePath, _ = getDefaultConfigPath()
		}
		if effectivePath != "" {
			if c, err := config.Load(effectivePath); err == nil {
				cfg = c
				chatClient.SetToken(cfg.Token)
			}
		}
	case "logout":
		chatClient.SetToken("")
	}
	return nil
}

func runSlashModel(_ *gateway.Client, rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		if chatSessionModel == "" {
			fmt.Fprintln(os.Stderr, "model: (default)")
		} else {
			fmt.Fprintln(os.Stderr, "model:", chatSessionModel)
		}
		return nil
	}
	chatSessionModel = rest
	fmt.Fprintln(os.Stderr, "model set to:", rest)
	return nil
}

// runSlashProjectDelegated runs "cynork project <rest>" then syncs chat session project when "set" was used.
func runSlashProjectDelegated(_ *gateway.Client, rest string) error {
	if err := runCynorkSubcommandForSlash("project", rest); err != nil {
		return err
	}
	parts := parseArgs(strings.TrimSpace(rest))
	if len(parts) >= 2 && strings.EqualFold(parts[0], "set") {
		setChatSessionProject(parts[1])
	}
	return nil
}

func setChatSessionProject(id string) {
	chatSessionProjectID = id
	if chatSessionProjectID == "none" || chatSessionProjectID == `""` {
		chatSessionProjectID = ""
	}
	fmt.Fprintln(os.Stderr, "project set to:", chatSessionProjectID)
}

// runSlashThread handles /thread <subcommand>. Currently supports "new".
func runSlashThread(client *gateway.Client, rest string) error {
	sub := strings.ToLower(strings.TrimSpace(rest))
	switch sub {
	case "new", "":
		if client == nil {
			fmt.Fprintln(os.Stderr, "thread: not connected")
			return nil
		}
		return startNewThread(client)
	default:
		fmt.Fprintf(os.Stderr, "thread: unknown subcommand %q — use: /thread new\n", sub)
		return nil
	}
}
