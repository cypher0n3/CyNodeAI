package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var taskCreatePrompt string
var taskCreateTask string
var taskCreateTaskFile string
var taskCreateScript string
var taskCreateCommands []string
var taskCreateCommandsFile string
var taskCreateTaskName string
var taskCreateAttachments []string
var taskCreateUseInference bool
var taskCreateInputMode string
var taskCreateUseSBA bool
var taskCreateProjectID string
var taskCreateResult bool
var taskWatchInterval time.Duration
var taskWatchNoClear bool
var taskListLimit int
var taskListStatus string
var taskListOffset int
var taskListCursor string
var taskCancelYes bool
var taskResultWait bool
var taskResultWaitInterval time.Duration

// taskCmd represents the task command group.
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task operations (create, list, get, result, cancel, logs, watch)",
}

var taskCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a task via POST /v1/tasks",
	RunE:  runTaskCreate,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks via GET /v1/tasks",
	RunE:  runTaskList,
}

var taskGetCmd = &cobra.Command{
	Use:   "get [task-id]",
	Short: "Get task details via GET /v1/tasks/{id}",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskGet,
}

var taskResultCmd = &cobra.Command{
	Use:   "result [task-id]",
	Short: "Get task result via GET /v1/tasks/{id}/result",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskResult,
}

var taskCancelCmd = &cobra.Command{
	Use:   "cancel [task-id]",
	Short: "Cancel a task via POST /v1/tasks/{id}/cancel",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskCancel,
}

var taskLogsCmd = &cobra.Command{
	Use:   "logs [task-id]",
	Short: "Get task logs via GET /v1/tasks/{id}/logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskLogs,
}

var taskWatchCmd = &cobra.Command{
	Use:   "watch [task-id]",
	Short: "Poll task status and display results (like watch(1))",
	Long:  "Repeatedly fetches task result at the given interval, clears the screen and redraws. Exits when the task reaches a terminal status (completed, failed, canceled) or on interrupt.",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskWatch,
}

var taskArtifactsCmd = &cobra.Command{
	Use:   "artifacts",
	Short: "Task artifacts (list, get)",
}

var taskArtifactsListCmd = &cobra.Command{
	Use:   "list [task-id]",
	Short: "List artifacts for a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskArtifactsList,
}

var terminalTaskStatuses = map[string]bool{
	"completed":  true,
	"failed":     true,
	"canceled":   true,
	"cancelled":  true,
	"superseded": true,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskCreateCmd, taskListCmd, taskGetCmd, taskResultCmd, taskCancelCmd, taskLogsCmd, taskWatchCmd, taskArtifactsCmd)
	taskArtifactsCmd.AddCommand(taskArtifactsListCmd)
	taskWatchCmd.Flags().DurationVarP(&taskWatchInterval, "interval", "n", 2*time.Second, "poll interval")
	taskWatchCmd.Flags().BoolVar(&taskWatchNoClear, "no-clear", false, "do not clear screen between polls")
	taskCreateCmd.Flags().StringVarP(&taskCreateTask, "task", "t", "", "task text (inline)")
	taskCreateCmd.Flags().StringVarP(&taskCreatePrompt, "prompt", "p", "", "task prompt (natural language or command)")
	taskCreateCmd.Flags().StringVarP(&taskCreateTaskFile, "task-file", "f", "", "task input file (max 1 MiB)")
	taskCreateCmd.Flags().StringVarP(&taskCreateScript, "script", "s", "", "script file input (max 256 KiB)")
	taskCreateCmd.Flags().StringArrayVar(&taskCreateCommands, "command", nil, "command input (repeatable)")
	taskCreateCmd.Flags().StringVar(&taskCreateCommandsFile, "commands-file", "", "commands input file (max 64 KiB)")
	taskCreateCmd.Flags().StringVar(&taskCreateTaskName, "name", "", "optional task name")
	taskCreateCmd.Flags().StringVar(&taskCreateTaskName, "task-name", "", "optional task name (normalized per Task Naming)")
	taskCreateCmd.Flags().StringArrayVar(&taskCreateAttachments, "attach", nil, "attachment path (repeatable, max 16)")
	taskCreateCmd.Flags().StringArrayVar(&taskCreateAttachments, "attachment", nil, "attachment path (repeatable)")
	taskCreateCmd.Flags().StringVar(&taskCreateProjectID, "project-id", "", "optional project id")
	taskCreateCmd.Flags().BoolVar(&taskCreateResult, "result", false, "wait for terminal status and print result")
	taskCreateCmd.Flags().BoolVar(&taskCreateUseInference, "use-inference", false, "run job in a pod with inference proxy (OLLAMA_BASE_URL in sandbox)")
	taskCreateCmd.Flags().StringVar(&taskCreateInputMode, "input-mode", "prompt", "input mode: prompt (default, use inference), script, or commands (literal shell)")
	taskCreateCmd.Flags().BoolVar(&taskCreateUseSBA, "use-sba", false, "create task with SBA runner job (job_spec_json); prompt as task context (P2-10)")
	taskListCmd.Flags().IntVarP(&taskListLimit, "limit", "l", 50, "max tasks to return")
	taskListCmd.Flags().IntVar(&taskListOffset, "offset", 0, "pagination offset")
	taskListCmd.Flags().StringVar(&taskListCursor, "cursor", "", "pagination cursor")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "filter by status")
	taskCancelCmd.Flags().BoolVarP(&taskCancelYes, "yes", "y", false, "skip confirmation")
	taskResultCmd.Flags().BoolVarP(&taskResultWait, "wait", "w", false, "poll until terminal status")
	taskResultCmd.Flags().DurationVar(&taskResultWaitInterval, "wait-interval", 2*time.Second, "poll interval when --wait")
}

const (
	maxTaskFileBytes     = 1 << 20
	maxScriptFileBytes   = 256 << 10
	maxCommandsFileBytes = 64 << 10
	maxAttachFileBytes   = 10 << 20
	maxAttachmentCount   = 16
)

func validateRegularReadableFile(path string) (int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, fmt.Errorf("path %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("path %q: symlinks are not allowed", path)
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("path %q: regular file required", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("path %q: not readable: %w", path, err)
	}
	_ = f.Close()
	return info.Size(), nil
}

func readModeFile(path, mode string, maxBytes int64) (string, error) {
	size, err := validateRegularReadableFile(path)
	if err != nil {
		return "", err
	}
	if size > maxBytes {
		return "", fmt.Errorf("%s %q exceeds size limit (%d bytes)", mode, path, maxBytes)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s file %q: %w", mode, path, err)
	}
	return string(b), nil
}

func resolveTaskCreateInput() (prompt string, inputMode string, err error) {
	modeCount := 0
	taskSet := strings.TrimSpace(taskCreateTask) != ""
	promptSet := strings.TrimSpace(taskCreatePrompt) != ""
	if taskSet && promptSet {
		return "", "", fmt.Errorf("exactly one of --task or --prompt is allowed")
	}
	inline := taskSet || promptSet
	if inline {
		modeCount++
	}
	if strings.TrimSpace(taskCreateTaskFile) != "" {
		modeCount++
	}
	if strings.TrimSpace(taskCreateScript) != "" {
		modeCount++
	}
	if len(taskCreateCommands) > 0 {
		modeCount++
	}
	if strings.TrimSpace(taskCreateCommandsFile) != "" {
		modeCount++
	}
	if modeCount != 1 {
		return "", "", fmt.Errorf("exactly one task input mode is required")
	}
	if inline {
		if strings.TrimSpace(taskCreateTask) != "" {
			return taskCreateTask, "prompt", nil
		}
		return taskCreatePrompt, "prompt", nil
	}
	if strings.TrimSpace(taskCreateTaskFile) != "" {
		p, e := readModeFile(taskCreateTaskFile, "task-file", maxTaskFileBytes)
		return p, "prompt", e
	}
	if strings.TrimSpace(taskCreateScript) != "" {
		p, e := readModeFile(taskCreateScript, "script", maxScriptFileBytes)
		return p, "script", e
	}
	if len(taskCreateCommands) > 0 {
		return strings.Join(taskCreateCommands, "\n"), "commands", nil
	}
	p, e := readModeFile(taskCreateCommandsFile, "commands-file", maxCommandsFileBytes)
	return p, "commands", e
}

func validateAttachments(paths []string) error {
	if len(paths) > maxAttachmentCount {
		return fmt.Errorf("too many attachments: max %d", maxAttachmentCount)
	}
	for _, p := range paths {
		size, err := validateRegularReadableFile(p)
		if err != nil {
			return err
		}
		if size > maxAttachFileBytes {
			return fmt.Errorf("attachment %q exceeds size limit (%d bytes)", p, maxAttachFileBytes)
		}
	}
	return nil
}

func runTaskCreate(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	prompt, mode, err := resolveTaskCreateInput()
	if err != nil {
		return exit.Usage(err)
	}
	if err := validateAttachments(taskCreateAttachments); err != nil {
		return exit.Usage(err)
	}
	var projectID *string
	if strings.TrimSpace(taskCreateProjectID) != "" {
		trimmedProjectID := strings.TrimSpace(taskCreateProjectID)
		if _, err := uuid.Parse(trimmedProjectID); err != nil {
			return exit.Usage(fmt.Errorf("invalid --project-id: %w", err))
		}
		projectID = &trimmedProjectID
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	req := userapi.CreateTaskRequest{
		Prompt:       prompt,
		ProjectID:    projectID,
		UseInference: taskCreateUseInference,
		InputMode:    mode,
		UseSBA:       taskCreateUseSBA,
		Attachments:  taskCreateAttachments,
	}
	if taskCreateInputMode != "" && taskCreateInputMode != "prompt" && mode == "prompt" {
		req.InputMode = taskCreateInputMode
	}
	if taskCreateTaskName != "" {
		req.TaskName = &taskCreateTaskName
	}
	task, err := client.CreateTask(&req)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	taskID := task.ResolveTaskID()
	if taskCreateResult {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		for {
			result, err := client.GetTaskResult(taskID)
			if err != nil {
				return exitFromGatewayErr(err)
			}
			if terminalTaskStatuses[result.Status] {
				printTaskResult(result)
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
		}
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(task)
		return nil
	}
	fmt.Println("task_id=" + taskID)
	return nil
}

func runTaskList(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	limit := taskListLimit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	resp, err := client.ListTasks(gateway.ListTasksRequest{
		Limit:  limit,
		Offset: taskListOffset,
		Cursor: taskListCursor,
		Status: taskListStatus,
	})
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(resp)
		return nil
	}
	for i := range resp.Tasks {
		t := &resp.Tasks[i]
		line := fmt.Sprintf("task_id=%s status=%s", t.ResolveTaskID(), t.Status)
		if t.TaskName != nil && *t.TaskName != "" {
			line += " task_name=" + *t.TaskName
		}
		fmt.Println(line)
	}
	return nil
}

func runTaskGet(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	task, err := client.GetTask(args[0])
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(task)
		return nil
	}
	line := fmt.Sprintf("task_id=%s status=%s", task.ResolveTaskID(), task.Status)
	if task.TaskName != nil && *task.TaskName != "" {
		line += " task_name=" + *task.TaskName
	}
	fmt.Println(line)
	return nil
}

func runTaskResult(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	taskID := args[0]
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	if taskResultWait {
		interval := taskResultWaitInterval
		if interval < time.Second {
			interval = time.Second
		}
		for {
			result, err := client.GetTaskResult(taskID)
			if err != nil {
				return exitFromGatewayErr(err)
			}
			if terminalTaskStatuses[result.Status] {
				printTaskResult(result)
				return nil
			}
			time.Sleep(interval)
		}
	}
	result, err := client.GetTaskResult(taskID)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	printTaskResult(result)
	return nil
}

func runTaskCancel(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	taskID := args[0]
	if !taskCancelYes {
		fmt.Fprintf(os.Stderr, "Cancel task %s? [y/N] ", taskID)
		var ch string
		if _, err := fmt.Scanln(&ch); err != nil || (ch != "y" && ch != "Y") {
			return nil
		}
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	resp, err := client.CancelTask(taskID)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(map[string]any{"task_id": resp.TaskID, "canceled": resp.Canceled})
		return nil
	}
	fmt.Printf("task_id=%s canceled=%t\n", resp.TaskID, resp.Canceled)
	return nil
}

func runTaskLogs(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	logs, err := client.GetTaskLogs(args[0], "")
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(logs)
		return nil
	}
	if logs.Stdout != "" {
		fmt.Print(logs.Stdout)
	}
	if logs.Stderr != "" {
		fmt.Fprint(os.Stderr, logs.Stderr)
	}
	return nil
}

func printTaskResult(result *userapi.TaskResultResponse) {
	if outputFmt == outputFormatJSON {
		out := map[string]any{"task_id": result.TaskID, "status": result.Status}
		var stdout, stderr string
		for _, j := range result.Jobs {
			if j.Result != nil {
				stdout += *j.Result
			}
		}
		if stdout != "" {
			out["stdout"] = stdout
		}
		if stderr != "" {
			out["stderr"] = stderr
		}
		_ = jsonOutputEncoder().Encode(out)
		return
	}
	fmt.Printf("task_id=%s status=%s\n", result.TaskID, result.Status)
	for _, j := range result.Jobs {
		fmt.Printf("job %s: %s\n", j.ID, j.Status)
		if j.Result != nil {
			fmt.Println(*j.Result)
		}
	}
}

func runTaskArtifactsList(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	path := "/v1/tasks/" + url.PathEscape(args[0]) + "/artifacts"
	body, err := client.GetBytes(path)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if len(body) == 0 {
		body = []byte("[]")
	}
	printJSONOrRaw(body)
	return nil
}

func runTaskWatch(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	taskID := args[0]
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	useClear := !taskWatchNoClear && term.IsTerminal(int(os.Stdout.Fd()))
	interval := taskWatchInterval
	if interval < time.Second {
		interval = time.Second
	}

	for {
		result, err := client.GetTaskResult(taskID)
		if err != nil {
			return exitFromGatewayErr(err)
		}

		if useClear {
			fmt.Print("\033[H\033[2J")
		}
		fmt.Printf("Every %s: cynork task result %s  %s\n\n", interval.Round(time.Millisecond), taskID, time.Now().Format(time.RFC3339))
		printTaskResult(result)

		if terminalTaskStatuses[result.Status] {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
			// next poll
		}
	}
}
