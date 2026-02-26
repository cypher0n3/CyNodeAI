package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

const subList = "list"
const subGet = "get"

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
	"exit":   func(*gateway.Client, string) (bool, error) { return true, nil },
	"quit":   func(*gateway.Client, string) (bool, error) { return true, nil },
	"help":   func(*gateway.Client, string) (bool, error) { printSlashHelp(); return false, nil },
	"clear":  func(*gateway.Client, string) (bool, error) { clearTerminal(); return false, nil },
	"version": func(*gateway.Client, string) (bool, error) { fmt.Println("cynork", version); return false, nil },
	"models": func(*gateway.Client, string) (bool, error) { return false, runModelsList(nil, nil) },
	"model":  func(c *gateway.Client, rest string) (bool, error) { return false, runSlashModel(c, rest) },
	"project": func(c *gateway.Client, rest string) (bool, error) { return false, runSlashProject(c, rest) },
	"task":   func(c *gateway.Client, rest string) (bool, error) { return false, runSlashTask(c, rest) },
	"status": func(*gateway.Client, string) (bool, error) { return false, runStatus(nil, nil) },
	"whoami": func(*gateway.Client, string) (bool, error) { return false, runAuthWhoami(nil, nil) },
	"auth":   func(_ *gateway.Client, rest string) (bool, error) { return false, runSlashAuth(rest) },
	"nodes":  func(_ *gateway.Client, rest string) (bool, error) { return false, runSlashNodes(rest) },
	"prefs":  func(_ *gateway.Client, rest string) (bool, error) { return false, runSlashPrefs(rest) },
	"skills": func(_ *gateway.Client, rest string) (bool, error) { return false, runSlashSkills(rest) },
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

func clearTerminal() {
	// ANSI clear screen; if not a TTY, do nothing (spec: MAY print message that clearing is not available)
	if os.Stdout == nil {
		return
	}
	_, _ = fmt.Fprint(os.Stdout, "\033[H\033[2J")
}

func runSlashAuth(rest string) error {
	rest = strings.TrimSpace(rest)
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		fmt.Fprintln(os.Stderr, "usage: /auth login|logout|whoami|refresh [login flags: -u username -p password]")
		return nil
	}
	sub := strings.ToLower(parts[0])
	args := parts[1:]
	switch sub {
	case "whoami":
		return runAuthWhoami(nil, nil)
	case "logout":
		return runAuthLogout(nil, nil)
	case "refresh":
		return runAuthRefresh(nil, nil)
	case "login":
		fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
		u := fs.String("u", "", "username")
		p := fs.String("p", "", "password")
		if err := fs.Parse(args); err != nil {
			return nil
		}
		authLoginHandle = *u
		authLoginPassword = *p
		defer func() { authLoginHandle = ""; authLoginPassword = "" }()
		return runAuthLogin(nil, nil)
	default:
		fmt.Fprintln(os.Stderr, "usage: /auth login|logout|whoami|refresh [login flags: -u username -p password]")
		return nil
	}
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

func runSlashProject(_ *gateway.Client, rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		printChatSessionProject()
		return nil
	}
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return nil
	}
	first := strings.ToLower(parts[0])
	switch first {
	case subList:
		return runProjectList(nil, nil)
	case subGet:
		return runSlashProjectGet(parts)
	case "set":
		return runSlashProjectSet(parts)
	case "help", "--help", "-h":
		fmt.Fprintln(os.Stderr, "usage: /project [list|get <project_id>|set <project_id>] or /project <project_id> to set")
		return nil
	}
	setChatSessionProject(parts[0])
	return nil
}

func printChatSessionProject() {
	if chatSessionProjectID == "" {
		fmt.Fprintln(os.Stderr, "project: (default)")
	} else {
		fmt.Fprintln(os.Stderr, "project:", chatSessionProjectID)
	}
}

func setChatSessionProject(id string) {
	chatSessionProjectID = id
	if chatSessionProjectID == "none" || chatSessionProjectID == `""` {
		chatSessionProjectID = ""
	}
	fmt.Fprintln(os.Stderr, "project set to:", chatSessionProjectID)
}

func runSlashProjectGet(parts []string) error {
	if len(parts) < 2 {
		fmt.Fprintln(os.Stderr, "usage: /project get <project_id>")
		return nil
	}
	return runProjectGet(nil, parts[1:])
}

func runSlashProjectSet(parts []string) error {
	if len(parts) < 2 {
		fmt.Fprintln(os.Stderr, "usage: /project set <project_id>")
		return nil
	}
	setChatSessionProject(parts[1])
	return nil
}

func runSlashTask(client *gateway.Client, rest string) error {
	rest = strings.TrimSpace(rest)
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		fmt.Fprintln(os.Stderr, "usage: /task list|get|create|cancel|result|logs|artifacts list ...")
		return nil
	}
	sub := strings.ToLower(parts[0])
	args := parts[1:]
	switch sub {
	case subList:
		return runSlashTaskList(args)
	case subGet:
		return runSlashTaskGet(args)
	case "create":
		return runSlashTaskCreate(rest)
	case "cancel":
		return runSlashTaskCancel(args)
	case "result":
		return runSlashTaskResult(args)
	case "logs":
		return runSlashTaskLogs(args)
	case "artifacts":
		return runSlashTaskArtifacts(args)
	default:
		fmt.Fprintln(os.Stderr, "usage: /task list|get|create|cancel|result|logs|artifacts list ...")
		return nil
	}
}

func runSlashTaskList(args []string) error {
	fs := flag.NewFlagSet("task list", flag.ContinueOnError)
	limit := fs.Int("limit", 50, "")
	status := fs.String("status", "", "")
	_ = fs.Parse(args)
	taskListLimit = *limit
	taskListStatus = *status
	return runTaskList(nil, nil)
}

func runSlashTaskGet(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: /task get <task_id>")
		return nil
	}
	return runTaskGet(nil, args)
}

func runSlashTaskCreate(rest string) error {
	prompt := strings.TrimSpace(strings.TrimPrefix(rest, "create"))
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "usage: /task create [prompt text or --prompt \"...\"]")
		return nil
	}
	taskCreatePrompt = prompt
	taskCreateInputMode = "prompt"
	return runTaskCreate(nil, nil)
}

func runSlashTaskCancel(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: /task cancel <task_id>")
		return nil
	}
	taskCancelYes = true
	return runTaskCancel(nil, args)
}

func runSlashTaskResult(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: /task result <task_id> [--wait]")
		return nil
	}
	taskResultWait = false
	taskArgs := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--wait" || a == "-w" {
			taskResultWait = true
			continue
		}
		taskArgs = append(taskArgs, a)
	}
	if len(taskArgs) < 1 {
		fmt.Fprintln(os.Stderr, "usage: /task result <task_id> [--wait]")
		return nil
	}
	return runTaskResult(nil, taskArgs)
}

func runSlashTaskLogs(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: /task logs <task_id>")
		return nil
	}
	return runTaskLogs(nil, args)
}

func runSlashTaskArtifacts(args []string) error {
	if len(args) >= 1 && strings.EqualFold(args[0], subList) {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: /task artifacts list <task_id>")
			return nil
		}
		return runTaskArtifactsList(nil, args[1:])
	}
	fmt.Fprintln(os.Stderr, "usage: /task artifacts list <task_id>")
	return nil
}

func runSlashNodes(rest string) error {
	return runSlashListGet(rest, "/nodes list|get <node_id>", "usage: /nodes get <node_id>",
		func() error { return runNodesList(nil, nil) },
		func(args []string) error { return runNodesGet(nil, args) })
}

func runSlashListGet(rest, usageAll, usageGet string, runList func() error, runGet func([]string) error) error {
	rest = strings.TrimSpace(rest)
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		fmt.Fprintln(os.Stderr, "usage:", usageAll)
		return nil
	}
	switch strings.ToLower(parts[0]) {
	case subList:
		return runList()
	case subGet:
		if len(parts) < 2 {
			fmt.Fprintln(os.Stderr, usageGet)
			return nil
		}
		return runGet(parts[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage:", usageAll)
		return nil
	}
}

func runSlashPrefs(rest string) error {
	rest = strings.TrimSpace(rest)
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		fmt.Fprintln(os.Stderr, "usage: /prefs list|get|set|delete|effective ...")
		return nil
	}
	switch strings.ToLower(parts[0]) {
	case subList:
		return runPrefsList(nil, nil)
	case subGet:
		return runPrefsGet(nil, parts[1:])
	case "set":
		return runPrefsSet(nil, parts[1:])
	case "delete":
		return runPrefsDelete(nil, parts[1:])
	case "effective":
		return runPrefsEffective(nil, parts[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: /prefs list|get|set|delete|effective ...")
		return nil
	}
}

func runSlashSkills(rest string) error {
	return runSlashListGet(rest, "/skills list|get <skill_id>", "usage: /skills get <skill_id>",
		func() error { return runSkillsList(nil, nil) },
		func(args []string) error { return runSkillsGet(nil, args) })
}
