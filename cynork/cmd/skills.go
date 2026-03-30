package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Skills management (list, get, load, update, delete)",
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
var skillsLoadName, skillsLoadScope string

var skillsUpdateCmd = &cobra.Command{
	Use:   "update [skill-id] [file]",
	Short: "Update a skill by id (content from file)",
	Args:  cobra.ExactArgs(2),
	RunE:  runSkillsUpdate,
}
var skillsUpdateName, skillsUpdateScope string

var skillsDeleteCmd = &cobra.Command{
	Use:   "delete [skill-id]",
	Short: "Delete a skill by id",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsDelete,
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd, skillsGetCmd, skillsLoadCmd, skillsUpdateCmd, skillsDeleteCmd)
	skillsLoadCmd.Flags().StringVar(&skillsLoadName, "name", "", "Skill name")
	skillsLoadCmd.Flags().StringVar(&skillsLoadScope, "scope", "user", "Scope: user, group, project, global")
	skillsUpdateCmd.Flags().StringVar(&skillsUpdateName, "name", "", "Skill name (optional)")
	skillsUpdateCmd.Flags().StringVar(&skillsUpdateScope, "scope", "", "Scope (optional)")
}

func runSkillsList(_ *cobra.Command, _ []string) error {
	return runStubList("/v1/skills")
}

func runSkillsGet(_ *cobra.Command, args []string) error {
	return runStubFetch("/v1/skills/"+args[0], "{}")
}

func runSkillsLoad(cmd *cobra.Command, args []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	payload := map[string]string{"content": string(content)}
	if skillsLoadName != "" {
		payload["name"] = skillsLoadName
	}
	if skillsLoadScope != "" {
		payload["scope"] = skillsLoadScope
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	resp, err := client.PostBytes(cmdContext(cmd), "/v1/skills/load", body)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if len(resp) > 0 {
		printJSONOrRaw(resp)
	}
	return nil
}

func runSkillsUpdate(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[1])
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	payload := map[string]string{"content": string(content)}
	if skillsUpdateName != "" {
		payload["name"] = skillsUpdateName
	}
	if skillsUpdateScope != "" {
		payload["scope"] = skillsUpdateScope
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	resp, err := client.PutBytes(cmdContext(cmd), "/v1/skills/"+args[0], body)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if len(resp) > 0 {
		printJSONOrRaw(resp)
	}
	return nil
}

func runSkillsDelete(_ *cobra.Command, args []string) error {
	return runStubDelete("/v1/skills/" + args[0])
}
