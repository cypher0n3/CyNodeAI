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

var nodesGetCmd = &cobra.Command{
	Use:   "get [node-id]",
	Short: "Get node details",
	Args:  cobra.ExactArgs(1),
	RunE:  runNodesGet,
}

func init() {
	rootCmd.AddCommand(nodesCmd)
	nodesCmd.AddCommand(nodesListCmd, nodesGetCmd)
}

func runNodesList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/nodes")
}

func runNodesGet(_ *cobra.Command, args []string) error {
	return runStubFetch("/v1/nodes/"+args[0], "{}")
}
