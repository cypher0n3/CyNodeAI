// Package cmd provides the cynork CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/spf13/cobra"
)

var (
	configPath string
	cfg        *config.Config
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "cynork",
	Short: "CyNodeAI CLI management client",
	Long:  "Operates against the User API Gateway for auth, tasks, and admin operations.",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default ~/.config/cynork/config.yaml)")
}
