//nolint:dupl // stub command structure shared with audit, nodes
package cmd

import (
	"github.com/spf13/cobra"
)

var credsCmd = &cobra.Command{
	Use:   "creds",
	Short: "Credential management (stub until orchestrator supports)",
}

var credsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List credentials metadata",
	RunE:  runCredsList,
}

func init() {
	rootCmd.AddCommand(credsCmd)
	credsCmd.AddCommand(credsListCmd)
}

func runCredsList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/creds")
}
