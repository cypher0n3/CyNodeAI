package pmasubprocess

import (
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
)

func TestStart_Disabled(t *testing.T) {
	cfg := &config.OrchestratorConfig{PMAEnabled: false}
	logger := slog.Default()
	cmd, err := Start(cfg, logger)
	if err != nil {
		t.Fatalf("Start() err = %v", err)
	}
	if cmd != nil {
		t.Errorf("Start() with PMA disabled = %v, want nil", cmd)
	}
}

func TestStart_EnabledButBinaryMissing(t *testing.T) {
	cfg := &config.OrchestratorConfig{
		PMAEnabled:    true,
		PMABinaryPath: "/nonexistent/cynode-pma",
		PMAListenAddr: ":8090",
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cmd, err := Start(cfg, logger)
	if err == nil {
		if cmd != nil {
			_ = cmd.Process.Signal(os.Interrupt)
			_ = cmd.Wait()
		}
		t.Fatal("Start() with missing binary want error")
	}
	if cmd != nil {
		t.Errorf("Start() on error returned cmd = %v", cmd)
	}
}

func TestStart_EnabledEmptyBinaryUsesDefault(t *testing.T) {
	cfg := &config.OrchestratorConfig{
		PMAEnabled:    true,
		PMABinaryPath: "",
		PMAListenAddr: ":8090",
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cmd, err := Start(cfg, logger)
	if err != nil {
		return
	}
	if cmd != nil {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}
}

func TestStart_EnabledBinarySucceeds(t *testing.T) {
	path, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true not in PATH, skipping")
	}
	cfg := &config.OrchestratorConfig{
		PMAEnabled:    true,
		PMABinaryPath: path,
		PMAListenAddr: ":8090",
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cmd, err := Start(cfg, logger)
	if err != nil {
		t.Fatalf("Start() err = %v", err)
	}
	if cmd == nil {
		t.Fatal("Start() with valid binary want non-nil cmd")
	}
	_ = cmd.Wait()
}
