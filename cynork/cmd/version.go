package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build info",
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Println("cynork", version)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
