package cmd

import (
	"fmt"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Skills management (stub until orchestrator supports)",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills",
	RunE:  runSkillsList,
}

var skillsGetCmd = &cobra.Command{
	Use:   "get [skill-id]",
	Short: "Get skill content and metadata",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsGet,
}

var skillsLoadCmd = &cobra.Command{
	Use:   "load [file]",
	Short: "Load a skill from a markdown file",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsLoad,
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd, skillsGetCmd, skillsLoadCmd)
}

func runSkillsList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/skills")
}

func runSkillsGet(_ *cobra.Command, args []string) error {
	return runStubFetch("/v1/skills/"+args[0], "{}")
}

func runSkillsLoad(_ *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	_, err := client.PostBytes("/v1/skills/load", []byte("{}"))
	if err != nil {
		return exitFromGatewayErr(err)
	}
	return nil
}
