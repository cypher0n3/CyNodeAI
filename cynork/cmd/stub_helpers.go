package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/spf13/cobra"
)

// registerStubListCmd adds a top-level <use> command with a single "list" subcommand that
// calls GET <path>. This builder eliminates repetition across stub command files.
func registerStubListCmd(use, short, listShort, path string) {
	parent := &cobra.Command{Use: use, Short: short}
	parent.AddCommand(&cobra.Command{
		Use:   "list",
		Short: listShort,
		RunE:  func(_ *cobra.Command, _ []string) error { return runStubList(path) },
	})
	rootCmd.AddCommand(parent)
}

// registerStubListGetCmd is like registerStubListCmd but also adds a "get [id]" subcommand.
func registerStubListGetCmd(use, short, listShort, getShort, path string) {
	parent := &cobra.Command{Use: use, Short: short}
	parent.AddCommand(&cobra.Command{
		Use:   "list",
		Short: listShort,
		RunE:  func(_ *cobra.Command, _ []string) error { return runStubList(path) },
	})
	parent.AddCommand(&cobra.Command{
		Use:   "get [id]",
		Short: getShort,
		Args:  cobra.ExactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return runStubFetch(path+"/"+args[0], "{}") },
	})
	rootCmd.AddCommand(parent)
}

// registerStubSetGetCmd adds a top-level <use> command with "set" and "get" subcommands.
func registerStubSetGetCmd(use, short, setPath, getPath string) {
	parent := &cobra.Command{Use: use, Short: short}
	parent.AddCommand(&cobra.Command{
		Use:   "set",
		Short: "Set a " + use + " entry",
		RunE:  func(_ *cobra.Command, _ []string) error { return runStubSet(setPath) },
	})
	parent.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get a " + use + " entry",
		RunE:  func(_ *cobra.Command, _ []string) error { return runStubGet(getPath) },
	})
	rootCmd.AddCommand(parent)
}

// registerPrefsCmd adds the "prefs" command with list/set/get/delete/effective subcommands.
func registerPrefsCmd() {
	parent := &cobra.Command{Use: "prefs", Short: "User preferences (stub until orchestrator supports)"}
	subCmds := []*cobra.Command{
		{Use: "list", Short: "List preference entries", RunE: func(_ *cobra.Command, _ []string) error {
			return runStubList("/v1/prefs")
		}},
		{Use: "set", Short: "Set a preference", RunE: func(_ *cobra.Command, _ []string) error {
			return runStubSet("/v1/prefs")
		}},
		{Use: "get", Short: "Get a preference", RunE: func(_ *cobra.Command, _ []string) error {
			return runStubGet("/v1/prefs/effective")
		}},
		{Use: "delete", Short: "Delete a preference", RunE: func(_ *cobra.Command, _ []string) error {
			return runStubDelete("/v1/prefs")
		}},
		{Use: "effective", Short: "Show effective preferences for context", RunE: func(_ *cobra.Command, _ []string) error {
			return runStubGet("/v1/prefs/effective")
		}},
	}
	for _, sub := range subCmds {
		parent.AddCommand(sub)
	}
	rootCmd.AddCommand(parent)
}

// printJSONOrRaw writes body to stdout. If body is valid JSON, it is pretty-printed; otherwise printed as-is.
func printJSONOrRaw(body []byte) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return
	}
	var v any
	if err := json.Unmarshal(trimmed, &v); err != nil {
		fmt.Print(string(body))
		return
	}
	_ = jsonOutputEncoder().Encode(v)
}

// runStubFetch GETs path and prints body; if empty, prints emptyJSON (e.g. "[]" or "{}"). JSON is pretty-printed.
func runStubFetch(path, emptyJSON string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	body, err := client.GetBytes(path)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if len(body) == 0 {
		body = []byte(emptyJSON)
	}
	printJSONOrRaw(body)
	return nil
}

func runStubList(path string) error { return runStubFetch(path, "[]") }
func runStubGet(path string) error  { return runStubFetch(path, "{}") }

// runStubSet runs a stub "set" command: auth check, POST path with {}.
func runStubSet(path string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	_, err := client.PostBytes(path, []byte("{}"))
	if err != nil {
		return exitFromGatewayErr(err)
	}
	return nil
}

// runStubDelete runs a stub "delete" command: auth check, DELETE path; prints body if any.
func runStubDelete(path string) error {
	if cfg.Token == "" {
		return exit.Auth(fmt.Errorf("not logged in: run 'cynork auth login'"))
	}
	client := gateway.NewClient(cfg.GatewayURL)
	client.SetToken(cfg.Token)
	body, err := client.DeleteBytes(path)
	if err != nil {
		return exitFromGatewayErr(err)
	}
	if len(body) > 0 {
		printJSONOrRaw(body)
	}
	return nil
}
