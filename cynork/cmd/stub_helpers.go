package cmd

import (
	"fmt"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// runStubFetch GETs path and prints body; if empty, prints emptyJSON (e.g. "[]" or "{}").
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
	fmt.Print(string(body))
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
