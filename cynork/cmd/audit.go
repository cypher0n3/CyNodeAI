//nolint:dupl // stub command structure shared with creds, nodes
package cmd

import (
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit logs (stub until orchestrator supports)",
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit events",
	RunE:  runAuditList,
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditListCmd)
}

func runAuditList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/audit")
}
