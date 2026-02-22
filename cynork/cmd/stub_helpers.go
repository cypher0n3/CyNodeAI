package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

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
