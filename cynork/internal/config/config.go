// Package config provides configuration loading for the cynork CLI.
// See docs/tech_specs/cli_management_app.md.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultGatewayURL is the default user-gateway base URL (localhost:8080).
const DefaultGatewayURL = "http://localhost:8080"

// Config holds CLI configuration (file + env overrides).
type Config struct {
	GatewayURL string `yaml:"gateway_url" json:"gateway_url"`
	Token      string `yaml:"token" json:"token"`
}

// userHomeDir is overridable in tests.
var userHomeDir = os.UserHomeDir

// ConfigDir returns the default config directory.
// If XDG_CONFIG_HOME is set, uses $XDG_CONFIG_HOME/cynork; otherwise ~/.config/cynork.
// See docs/tech_specs/cli_management_app.md (CliConfigFileLocation).
func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cynork"), nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "cynork"), nil
}

// ConfigPath returns the default config file path (~/.config/cynork/config.yaml).
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// defaultConfigPath is used by Load when configPath is empty; tests may override.
var defaultConfigPath = ConfigPath

// Load reads config from optional file and applies env overrides.
// CYNORK_GATEWAY_URL and CYNORK_TOKEN override file values.
// If no file exists or path is empty, returns config from env/defaults only.
func Load(configPath string) (*Config, error) {
	cfg := &Config{
		GatewayURL: DefaultGatewayURL,
	}
	if configPath == "" {
		var err error
		configPath, err = defaultConfigPath()
		if err != nil {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.GatewayURL == "" {
		cfg.GatewayURL = DefaultGatewayURL
	}
	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("CYNORK_GATEWAY_URL"); v != "" {
		cfg.GatewayURL = v
	}
	if v := os.Getenv("CYNORK_TOKEN"); v != "" {
		cfg.Token = v
	}
}

// Save writes the config to the given path (e.g. after login).
// Creates parent directory if needed. Writes atomically (temp file + rename) so
// a crash or interrupt does not leave a partial file; subsequent CLI runs still
// see the previous config or a complete new one.
func Save(savePath string, cfg *Config) error {
	if savePath == "" {
		var err error
		savePath, err = defaultConfigPath()
		if err != nil {
			return err
		}
	}
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config.yaml.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	// CreateTemp uses 0o600 on Unix; no need to Chmod for spec compliance.
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, savePath); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}
