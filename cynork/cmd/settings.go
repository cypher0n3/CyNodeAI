//nolint:dupl // stub command structure shared with prefs
package cmd

import (
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "System settings (stub until orchestrator supports)",
}

var settingsSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a system setting",
	RunE:  runSettingsSet,
}
var settingsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a system setting",
	RunE:  runSettingsGet,
}

func init() {
	rootCmd.AddCommand(settingsCmd)
	settingsCmd.AddCommand(settingsSetCmd, settingsGetCmd)
}

func runSettingsSet(_ *cobra.Command, _ []string) error {
	return runStubSet("/v1/settings")
}

func runSettingsGet(_ *cobra.Command, _ []string) error {
	return runStubGet("/v1/settings")
}
