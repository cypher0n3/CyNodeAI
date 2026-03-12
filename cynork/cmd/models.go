package cmd

import (
	"fmt"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available models (GET /v1/models)",
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List models from the gateway",
	RunE:  runModelsList,
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(modelsListCmd)
}

func runModelsList(_ *cobra.Command, _ []string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	resp, err := client.ListModels()
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if outputFmt == outputFormatJSON {
		_ = jsonOutputEncoder().Encode(resp)
		return nil
	}
	for _, m := range resp.Data {
		fmt.Println(m.ID)
	}
	return nil
}
