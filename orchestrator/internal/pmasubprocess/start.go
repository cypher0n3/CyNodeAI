// Package pmasubprocess starts the cynode-pma agent binary as a subprocess.
// See docs/tech_specs/cynode_pma.md and docs/mvp_plan.md Phase 1.7.
package pmasubprocess

import (
	"log/slog"
	"os"
	"os/exec"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
)

// Start builds and starts the cynode-pma process. The caller must stop it (e.g. cmd.Process.Signal + cmd.Wait).
// Returns (nil, nil) if cfg.PMAEnabled is false.
func Start(cfg *config.OrchestratorConfig, logger *slog.Logger) (*exec.Cmd, error) {
	if !cfg.PMAEnabled {
		return nil, nil
	}
	binary := cfg.PMABinaryPath
	if binary == "" {
		binary = "cynode-pma"
	}
	args := []string{"--role=project_manager", "--listen=" + cfg.PMAListenAddr}
	if cfg.PMAInstructionsRoot != "" {
		args = append(args, "--instructions-root="+cfg.PMAInstructionsRoot)
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	logger.Info("cynode-pma started", "pid", cmd.Process.Pid, "addr", cfg.PMAListenAddr)
	return cmd, nil
}
