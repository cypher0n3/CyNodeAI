//nolint:dupl // stub command structure shared with settings
package cmd

import (
	"github.com/spf13/cobra"
)

var prefsCmd = &cobra.Command{
	Use:   "prefs",
	Short: "User preferences (stub until orchestrator supports)",
}

var prefsSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a preference",
	RunE:  runPrefsSet,
}
var prefsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a preference",
	RunE:  runPrefsGet,
}

func init() {
	rootCmd.AddCommand(prefsCmd)
	prefsCmd.AddCommand(prefsSetCmd, prefsGetCmd)
}

func runPrefsSet(_ *cobra.Command, _ []string) error {
	return runStubSet("/v1/prefs")
}

func runPrefsGet(_ *cobra.Command, _ []string) error {
	return runStubGet("/v1/prefs/effective")
}
