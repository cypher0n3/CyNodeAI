// Package cmd provides the cynork CLI commands.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

const (
	outputFormatJSON  = "json"
	outputFormatTable = "table"
)

var (
	configPath string
	outputFmt  string
	noColor    bool
	cfg        *config.Config
	// getDefaultConfigPath resolves the default config file path when --config is not set.
	// Tests may override to inject failures.
	getDefaultConfigPath = config.ConfigPath
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "cynork",
	Short: "CyNodeAI CLI management client",
	Long:  "Operates against the User API Gateway for auth, tasks, and admin operations.",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return exit.Usage(fmt.Errorf("load config: %w", err))
		}
		if outputFmt != "" && outputFmt != outputFormatTable && outputFmt != outputFormatJSON {
			return exit.Usage(fmt.Errorf("output must be table or json"))
		}
		return nil
	},
}

// jsonOutputEncoder returns an encoder for stdout with indentation and no HTML escaping.
func jsonOutputEncoder() *json.Encoder {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc
}

// Execute runs the root command.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exit.CodeOf(err)
	}
	return 0
}

// exitFromGatewayErr maps gateway HTTP errors to spec exit codes (3 auth, 4 not found, etc.).
func exitFromGatewayErr(err error) error {
	if err == nil {
		return nil
	}
	var he *gateway.HTTPError
	if !errors.As(err, &he) {
		return exit.Gateway(err)
	}
	switch he.Status {
	case 401, 403:
		return exit.Auth(he.Err)
	case 404:
		return exit.NotFound(he.Err)
	case 409:
		return exit.Conflict(he.Err)
	case 400, 422:
		return exit.Validation(he.Err)
	default:
		return exit.Gateway(he.Err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default ~/.config/cynork/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", outputFormatTable, "output format: table | json")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
}
