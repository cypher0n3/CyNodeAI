package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check gateway health/readiness",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
	client := gateway.NewClient(cfg.GatewayURL)
	if err := client.Health(); err != nil {
		return exit.Gateway(fmt.Errorf("gateway unreachable: %w", err))
	}
	if outputFmt == outputFormatJSON {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"gateway": "ok"})
		return nil
	}
	fmt.Println("ok")
	return nil
}
