//nolint:dupl // stub command structure shared with audit, creds
package cmd

import (
	"github.com/spf13/cobra"
)

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Node management (stub until orchestrator supports)",
}

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List nodes",
	RunE:  runNodesList,
}

func init() {
	rootCmd.AddCommand(nodesCmd)
	nodesCmd.AddCommand(nodesListCmd)
}

func runNodesList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/nodes")
}
