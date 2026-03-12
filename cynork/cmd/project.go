package cmd

import (
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Project context (stub until orchestrator supports)",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	RunE:  runProjectList,
}

var projectGetCmd = &cobra.Command{
	Use:   "get [project-id]",
	Short: "Get project details",
	Args:  cobra.ExactArgs(1),
	RunE:  runProjectGet,
}

var projectSetCmd = &cobra.Command{
	Use:   "set [project-id]",
	Short: "Set active project",
	Args:  cobra.ExactArgs(1),
	RunE:  runProjectSet,
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd, projectGetCmd, projectSetCmd)
}

func runProjectList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/projects")
}

func runProjectGet(_ *cobra.Command, args []string) error {
	return runStubFetch("/v1/projects/"+args[0], "{}")
}

func runProjectSet(_ *cobra.Command, _ []string) error {
	return runStubSet("/v1/projects/set")
}
