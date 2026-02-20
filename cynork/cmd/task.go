package cmd

import (
	"fmt"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var taskCreatePrompt string
var taskCreateUseInference bool

// taskCmd represents the task command group.
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task operations (create, result)",
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

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskCreateCmd, taskResultCmd)
	taskCreateCmd.Flags().StringVarP(&taskCreatePrompt, "prompt", "p", "", "task prompt (command to run)")
	taskCreateCmd.Flags().BoolVar(&taskCreateUseInference, "use-inference", false, "run job in a pod with inference proxy (OLLAMA_BASE_URL in sandbox)")
	_ = taskCreateCmd.MarkFlagRequired("prompt")
}

func runTaskCreate(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return fmt.Errorf("not logged in: run 'cynork auth login'")
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	task, err := client.CreateTask(gateway.CreateTaskRequest{Prompt: taskCreatePrompt, UseInference: taskCreateUseInference})
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
	fmt.Printf("task_id=%s status=%s\n", result.TaskID, result.Status)
	for _, j := range result.Jobs {
		fmt.Printf("job %s: %s\n", j.ID, j.Status)
		if j.Result != nil {
			fmt.Println(*j.Result)
		}
	}
	return nil
}
