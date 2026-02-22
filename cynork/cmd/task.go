package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var taskCreatePrompt string
var taskCreateUseInference bool
var taskCreateInputMode string
var taskWatchInterval time.Duration
var taskWatchNoClear bool
var taskListLimit int
var taskListStatus string
var taskListOffset int
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

var terminalTaskStatuses = map[string]bool{"completed": true, "failed": true, "canceled": true, "cancelled": true}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskCreateCmd, taskListCmd, taskGetCmd, taskResultCmd, taskCancelCmd, taskLogsCmd, taskWatchCmd)
	taskWatchCmd.Flags().DurationVarP(&taskWatchInterval, "interval", "n", 2*time.Second, "poll interval")
	taskWatchCmd.Flags().BoolVar(&taskWatchNoClear, "no-clear", false, "do not clear screen between polls")
	taskCreateCmd.Flags().StringVarP(&taskCreatePrompt, "prompt", "p", "", "task prompt (natural language or command)")
	taskCreateCmd.Flags().BoolVar(&taskCreateUseInference, "use-inference", false, "run job in a pod with inference proxy (OLLAMA_BASE_URL in sandbox)")
	taskCreateCmd.Flags().StringVar(&taskCreateInputMode, "input-mode", "prompt", "input mode: prompt (default, use inference), script, or commands (literal shell)")
	_ = taskCreateCmd.MarkFlagRequired("prompt")
	taskListCmd.Flags().IntVarP(&taskListLimit, "limit", "l", 50, "max tasks to return")
	taskListCmd.Flags().IntVar(&taskListOffset, "offset", 0, "pagination offset")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "filter by status")
	taskCancelCmd.Flags().BoolVarP(&taskCancelYes, "yes", "y", false, "skip confirmation")
	taskResultCmd.Flags().BoolVarP(&taskResultWait, "wait", "w", false, "poll until terminal status")
	taskResultCmd.Flags().DurationVar(&taskResultWaitInterval, "wait-interval", 2*time.Second, "poll interval when --wait")
}

func runTaskCreate(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	inputMode := taskCreateInputMode
	if inputMode == "" {
		inputMode = "prompt"
	}
	task, err := client.CreateTask(gateway.CreateTaskRequest{Prompt: taskCreatePrompt, UseInference: taskCreateUseInference, InputMode: inputMode})
	if err != nil {
		return exitFromGatewayErr(err)
	}
	taskID := task.ResolveTaskID()
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(map[string]string{"task_id": taskID})
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
	resp, err := client.ListTasks(gateway.ListTasksRequest{Limit: limit, Offset: taskListOffset, Status: taskListStatus})
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(resp)
		return nil
	}
	for _, t := range resp.Tasks {
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
		out := map[string]string{"task_id": task.ResolveTaskID(), "status": task.Status}
		if task.TaskName != nil {
			out["task_name"] = *task.TaskName
		}
		_ = jsonOutputEncoder().Encode(out)
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

func printTaskResult(result *gateway.TaskResultResponse) {
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
