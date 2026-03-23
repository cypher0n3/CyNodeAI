// Package config provides configuration loading for the cynork CLI.
// See docs/tech_specs/cli_management_app.md.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultGatewayURL is the default user-gateway base URL (localhost:12080).
// See docs/tech_specs/ports_and_endpoints.md and cynork_cli.md.
const DefaultGatewayURL = "http://localhost:12080"

// legacySessionFileName was used by older cynork builds; it is removed on load.
const legacySessionFileName = "session.yaml"

// TUIConfig holds TUI-specific preferences persisted in the config file.
type TUIConfig struct {
	ShowThinkingByDefault   bool `yaml:"show_thinking_by_default" json:"show_thinking_by_default"`
	ShowToolOutputByDefault bool `yaml:"show_tool_output_by_default" json:"show_tool_output_by_default"`
	// HealthPollIntervalSec is seconds between GET /healthz checks for the status indicator.
	// Omitted or nil defaults to 5. Explicit 0 disables polling (static idle glyph).
	HealthPollIntervalSec *int `yaml:"health_poll_interval_seconds,omitempty" json:"health_poll_interval_seconds,omitempty"`
}

// Config holds CLI configuration (file + env overrides).
// Token and RefreshToken are process memory only; they MUST NOT be loaded from
// or written to config.yaml (see Save and finalizeAfterConfigFileRead).
type Config struct {
	GatewayURL   string    `yaml:"gateway_url" json:"gateway_url"`
	Token        string    `yaml:"token" json:"token"`
	RefreshToken string    `yaml:"refresh_token" json:"refresh_token"`
	TUI          TUIConfig `yaml:"tui" json:"tui"`
}

// persistedConfig is written to config.yaml only (no secrets).
type persistedConfig struct {
	GatewayURL string    `yaml:"gateway_url"`
	TUI        TUIConfig `yaml:"tui,omitempty"`
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
// CYNORK_GATEWAY_URL, CYNORK_TOKEN, and CYNORK_REFRESH_TOKEN override file values.
// Secrets parsed from YAML are discarded; legacy token keys are stripped from disk.
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
			if err := finalizeAfterConfigFileRead(configPath, cfg, true); err != nil {
				return nil, err
			}
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
	if err := finalizeAfterConfigFileRead(configPath, cfg, true); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadFileWithoutEnvOverrides reads and parses the config file without applying CYNORK_*
// environment overrides. Used when persisting the CLI config so a session-only
// CYNORK_GATEWAY_URL does not overwrite gateway_url stored on disk.
func LoadFileWithoutEnvOverrides(configPath string) (*Config, error) {
	cfg := &Config{
		GatewayURL: DefaultGatewayURL,
	}
	if configPath == "" {
		var err error
		configPath, err = defaultConfigPath()
		if err != nil {
			return cfg, nil
		}
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := finalizeAfterConfigFileRead(configPath, cfg, false); err != nil {
				return nil, err
			}
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
	if err := finalizeAfterConfigFileRead(configPath, cfg, false); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("CYNORK_GATEWAY_URL"); v != "" {
		cfg.GatewayURL = v
	}
	if v := os.Getenv("CYNORK_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("CYNORK_REFRESH_TOKEN"); v != "" {
		cfg.RefreshToken = v
	}
}

func finalizeAfterConfigFileRead(configPath string, cfg *Config, applyEnv bool) error {
	if configPath == "" {
		if applyEnv {
			applyEnvOverrides(cfg)
		}
		return nil
	}
	if err := stripLegacyTokenKeysFromConfigFile(configPath, cfg); err != nil {
		return err
	}
	// Never use bearer tokens from YAML (REQ-CLIENT-0103).
	cfg.Token = ""
	cfg.RefreshToken = ""
	removeLegacySessionFile(configPath)
	if applyEnv {
		applyEnvOverrides(cfg)
	}
	return nil
}

func configFileHasLegacyTokenKeys(data []byte) bool {
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return false
	}
	_, tok := m["token"]
	_, ref := m["refresh_token"]
	return tok || ref
}

func stripLegacyTokenKeysFromConfigFile(configPath string, cfg *Config) error {
	if configPath == "" {
		return nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	if !configFileHasLegacyTokenKeys(data) {
		return nil
	}
	return Save(configPath, cfg)
}

func removeLegacySessionFile(configPath string) {
	if configPath == "" {
		return
	}
	path := filepath.Join(filepath.Dir(configPath), legacySessionFileName)
	_ = os.Remove(path)
}

// Save writes non-secret preferences to the given path (gateway URL, TUI prefs).
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
	p := persistedConfig{GatewayURL: cfg.GatewayURL, TUI: cfg.TUI}
	data, err := yaml.Marshal(&p)
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
