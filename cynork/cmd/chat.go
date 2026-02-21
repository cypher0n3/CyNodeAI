package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat with the Project Manager (create task + poll result per message)",
	Long:  "Reads lines from stdin; /exit or /quit or EOF exits. Each message is sent as a task and the result is printed when terminal. No token yields exit 3.",
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
		result, err := sendMessageAndPoll(client, line)
		if err != nil {
			return err
		}
		printJobResults(result)
	}
}

func sendMessageAndPoll(client *gateway.Client, message string) (*gateway.TaskResultResponse, error) {
	task, err := client.CreateTask(gateway.CreateTaskRequest{Prompt: message})
	if err != nil {
		return nil, exitFromGatewayErr(err)
	}
	for {
		result, err := client.GetTaskResult(task.ResolveTaskID())
		if err != nil {
			return nil, exitFromGatewayErr(err)
		}
		if terminalTaskStatuses[result.Status] {
			return result, nil
		}
		time.Sleep(2 * time.Second)
	}
}

func printJobResults(result *gateway.TaskResultResponse) {
	for _, j := range result.Jobs {
		if j.Result != nil {
			fmt.Println(*j.Result)
		}
	}
}
