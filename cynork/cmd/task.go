package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var taskCreatePrompt string
var taskCreateUseInference bool
var taskCreateInputMode string
var taskWatchInterval time.Duration
var taskWatchNoClear bool

// taskCmd represents the task command group.
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task operations (create, result, watch)",
}

var taskCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a task via POST /v1/tasks",
	RunE:  runTaskCreate,
}

var taskResultCmd = &cobra.Command{
	Use:   "result [task-id]",
	Short: "Get task result via GET /v1/tasks/{id}/result",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskResult,
}

var taskWatchCmd = &cobra.Command{
	Use:   "watch [task-id]",
	Short: "Poll task status and display results (like watch(1))",
	Long:  "Repeatedly fetches task result at the given interval, clears the screen and redraws. Exits when the task reaches a terminal status (completed, failed, canceled) or on interrupt.",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskWatch,
}

var terminalTaskStatuses = map[string]bool{"completed": true, "failed": true, "canceled": true}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskCreateCmd, taskResultCmd, taskWatchCmd)
	taskWatchCmd.Flags().DurationVarP(&taskWatchInterval, "interval", "n", 2*time.Second, "poll interval")
	taskWatchCmd.Flags().BoolVar(&taskWatchNoClear, "no-clear", false, "do not clear screen between polls")
	taskCreateCmd.Flags().StringVarP(&taskCreatePrompt, "prompt", "p", "", "task prompt (natural language or command)")
	taskCreateCmd.Flags().BoolVar(&taskCreateUseInference, "use-inference", false, "run job in a pod with inference proxy (OLLAMA_BASE_URL in sandbox)")
	taskCreateCmd.Flags().StringVar(&taskCreateInputMode, "input-mode", "prompt", "input mode: prompt (default, use inference), script, or commands (literal shell)")
	_ = taskCreateCmd.MarkFlagRequired("prompt")
}

func runTaskCreate(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return fmt.Errorf("not logged in: run 'cynork auth login'")
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	inputMode := taskCreateInputMode
	if inputMode == "" {
		inputMode = "prompt"
	}
	task, err := client.CreateTask(gateway.CreateTaskRequest{Prompt: taskCreatePrompt, UseInference: taskCreateUseInference, InputMode: inputMode})
	if err != nil {
		return err
	}
	fmt.Println(task.ID)
	return nil
}

func runTaskResult(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return fmt.Errorf("not logged in: run 'cynork auth login'")
	}
	taskID := args[0]
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	result, err := client.GetTaskResult(taskID)
	if err != nil {
		return err
	}
	printTaskResult(result)
	return nil
}

func printTaskResult(result *gateway.TaskResultResponse) {
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
		return fmt.Errorf("not logged in: run 'cynork auth login'")
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
			return err
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
