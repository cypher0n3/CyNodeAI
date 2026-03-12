// Package main tests that main() invokes the CLI and exits.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMainInvocation(t *testing.T) {
	if os.Getenv("CYNORK_MAIN") == "1" {
		os.Args = []string{"cynork", "version"}
		main()
		return
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainInvocation$")
	cmd.Env = append(os.Environ(), "CYNORK_MAIN=1", "CYNORK_GATEWAY_URL=http://localhost", "HOME="+dir)
	cmd.Dir = filepath.Dir(os.Args[0])
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() != 0 {
			t.Errorf("main() exit code = %d, want 0", exit.ExitCode())
		} else {
			t.Errorf("run main: %v", err)
		}
	}
}
