//nolint:dupl // stub command structure shared with settings
package cmd

import (
	"github.com/spf13/cobra"
)

var prefsCmd = &cobra.Command{
	Use:   "prefs",
	Short: "User preferences (stub until orchestrator supports)",
}

var prefsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List preference entries",
	RunE:  runPrefsList,
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

var prefsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a preference",
	RunE:  runPrefsDelete,
}

var prefsEffectiveCmd = &cobra.Command{
	Use:   "effective",
	Short: "Show effective preferences for context",
	RunE:  runPrefsEffective,
}

func init() {
	rootCmd.AddCommand(prefsCmd)
	prefsCmd.AddCommand(prefsListCmd, prefsSetCmd, prefsGetCmd, prefsDeleteCmd, prefsEffectiveCmd)
}

func runPrefsSet(_ *cobra.Command, _ []string) error {
	return runStubSet("/v1/prefs")
}

func runPrefsGet(_ *cobra.Command, _ []string) error {
	return runStubGet("/v1/prefs/effective")
}

func runPrefsList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/prefs")
}

func runPrefsDelete(_ *cobra.Command, _ []string) error {
	return runStubDelete("/v1/prefs")
}

func runPrefsEffective(_ *cobra.Command, _ []string) error {
	return runStubGet("/v1/prefs/effective")
}
