package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultGatewayURL_MatchesSpec asserts the literal value of DefaultGatewayURL
// so that drift between the CLI default and the spec port (12080) is caught at test time.
// See docs/tech_specs/ports_and_endpoints.md and cynork_cli.md.
func TestDefaultGatewayURL_MatchesSpec(t *testing.T) {
	const specURL = "http://localhost:12080"
	if DefaultGatewayURL != specURL {
		t.Errorf("DefaultGatewayURL = %q, want %q (see cynork_cli.md)", DefaultGatewayURL, specURL)
	}
}

func TestDefaultConfigPathError_LoadVariants(t *testing.T) {
	old := defaultConfigPath
	defer func() { defaultConfigPath = old }()
	defaultConfigPath = func() (string, error) { return "", errors.New("injected") }
	tests := []struct {
		name string
		load func() (*Config, error)
	}{
		{"Load", func() (*Config, error) { return Load("") }},
		{"LoadFileWithoutEnvOverrides", func() (*Config, error) { return LoadFileWithoutEnvOverrides("") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := tt.load()
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if cfg.GatewayURL != DefaultGatewayURL {
				t.Errorf("GatewayURL = %q, want default", cfg.GatewayURL)
			}
		})
	}
}

func TestLoadFileWithoutEnvOverrides_EmptyPathUsesDefaultConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("gateway_url: http://from-default:1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := defaultConfigPath
	defaultConfigPath = func() (string, error) { return path, nil }
	defer func() { defaultConfigPath = old }()
	cfg, err := LoadFileWithoutEnvOverrides("")
	if err != nil {
		t.Fatalf("LoadFileWithoutEnvOverrides: %v", err)
	}
	if cfg.GatewayURL != "http://from-default:1" {
		t.Errorf("GatewayURL = %q", cfg.GatewayURL)
	}
}

func TestConfigFileHasLegacyTokenKeys(t *testing.T) {
	if !configFileHasLegacyTokenKeys([]byte("token: x\n")) {
		t.Error("expected token key")
	}
	if !configFileHasLegacyTokenKeys([]byte("refresh_token: z\n")) {
		t.Error("expected refresh_token key")
	}
	if configFileHasLegacyTokenKeys([]byte("gateway_url: http://x\n")) {
		t.Error("no legacy keys")
	}
	if configFileHasLegacyTokenKeys([]byte("not valid yaml [[\n")) {
		t.Error("invalid yaml should be false")
	}
}

func TestFinalizeAfterConfigFileRead_EmptyPath(t *testing.T) {
	_ = os.Setenv("CYNORK_GATEWAY_URL", "http://from-env")
	defer func() { _ = os.Unsetenv("CYNORK_GATEWAY_URL") }()
	cfg := &Config{GatewayURL: DefaultGatewayURL}
	if err := finalizeAfterConfigFileRead("", cfg, false); err != nil {
		t.Fatal(err)
	}
	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("without applyEnv, file gateway must stay default; got %q", cfg.GatewayURL)
	}
	if err := finalizeAfterConfigFileRead("", cfg, true); err != nil {
		t.Fatal(err)
	}
	if cfg.GatewayURL != "http://from-env" {
		t.Errorf("with applyEnv, expected env gateway; got %q", cfg.GatewayURL)
	}
}

func TestFinalizeAfterConfigFileRead_WithFileAppliesTokenEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("gateway_url: http://file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("CYNORK_TOKEN", "env-token")
	defer func() { _ = os.Unsetenv("CYNORK_TOKEN") }()
	cfg := &Config{GatewayURL: "http://file"}
	if err := finalizeAfterConfigFileRead(path, cfg, true); err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "env-token" {
		t.Errorf("Token = %q, want env", cfg.Token)
	}
}

func TestLoadFileWithoutEnvOverrides_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	cfg, err := LoadFileWithoutEnvOverrides(path)
	if err != nil {
		t.Fatalf("LoadFileWithoutEnvOverrides: %v", err)
	}
	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("GatewayURL = %q, want default", cfg.GatewayURL)
	}
}

func TestLoadFileWithoutEnvOverrides_LegacyTokenKeysStripped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("gateway_url: http://legacy:8080\ntoken: secret\nrefresh_token: r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFileWithoutEnvOverrides(path)
	if err != nil {
		t.Fatalf("LoadFileWithoutEnvOverrides: %v", err)
	}
	if cfg.GatewayURL != "http://legacy:8080" {
		t.Errorf("GatewayURL = %q", cfg.GatewayURL)
	}
	if cfg.Token != "" || cfg.RefreshToken != "" {
		t.Errorf("tokens must be cleared: token=%q refresh=%q", cfg.Token, cfg.RefreshToken)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "token:") {
		t.Errorf("legacy token keys should be stripped from file: %s", raw)
	}
}

func TestLoadFileWithoutEnvOverrides_ReadPathNotFile(t *testing.T) {
	_, err := LoadFileWithoutEnvOverrides(t.TempDir())
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestLoad_RemovesLegacySessionFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sessionPath := filepath.Join(dir, legacySessionFileName)
	if err := os.WriteFile(configPath, []byte("gateway_url: http://localhost\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(configPath); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatal("legacy session file should be removed after load")
	}
}

func TestLoad_NoFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("GatewayURL = %q, want %q", cfg.GatewayURL, DefaultGatewayURL)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty", cfg.Token)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	const url = "http://other:9090"
	const token = "secret"
	_ = os.Setenv("CYNORK_GATEWAY_URL", url)
	_ = os.Setenv("CYNORK_TOKEN", token)
	defer func() { _, _ = os.Unsetenv("CYNORK_GATEWAY_URL"), os.Unsetenv("CYNORK_TOKEN") }()

	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GatewayURL != url {
		t.Errorf("GatewayURL = %q, want %q", cfg.GatewayURL, url)
	}
	if cfg.Token != token {
		t.Errorf("Token = %q, want %q", cfg.Token, token)
	}
}

func TestLoad_CYNORK_REFRESH_TOKEN(t *testing.T) {
	_ = os.Setenv("CYNORK_REFRESH_TOKEN", "r-secret")
	defer func() { _ = os.Unsetenv("CYNORK_REFRESH_TOKEN") }()
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RefreshToken != "r-secret" {
		t.Errorf("RefreshToken = %q, want r-secret", cfg.RefreshToken)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	const url = "http://file:8080"
	if err := os.WriteFile(path, []byte("gateway_url: "+url+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GatewayURL != url {
		t.Errorf("GatewayURL = %q, want %q", cfg.GatewayURL, url)
	}
}

func TestLoadFileWithoutEnvOverrides_IgnoresEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	const fileURL = "http://file:13080"
	if err := os.WriteFile(path, []byte("gateway_url: "+fileURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("CYNORK_GATEWAY_URL", "http://localhost:12080")
	defer func() { _ = os.Unsetenv("CYNORK_GATEWAY_URL") }()

	cfg, err := LoadFileWithoutEnvOverrides(path)
	if err != nil {
		t.Fatalf("LoadFileWithoutEnvOverrides: %v", err)
	}
	if cfg.GatewayURL != fileURL {
		t.Errorf("GatewayURL = %q, want %q (must not apply env)", cfg.GatewayURL, fileURL)
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")
	cfg := &Config{GatewayURL: "http://saved:8080", Token: "t"}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "token:") || strings.Contains(string(raw), "refresh_token:") {
		t.Fatalf("config.yaml must not contain secrets: %s", raw)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if loaded.GatewayURL != cfg.GatewayURL {
		t.Errorf("GatewayURL = %q, want %q", loaded.GatewayURL, cfg.GatewayURL)
	}
	if loaded.Token != "" {
		t.Errorf("Token from file load must be empty without CYNORK_TOKEN, got %q", loaded.Token)
	}
}

func TestConfigDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Setenv("HOME", dir)
	defer func() { _, _ = os.Unsetenv("HOME"), os.Unsetenv("XDG_CONFIG_HOME") }()
	got, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	want := filepath.Join(dir, ".config", "cynork")
	if got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestConfigDir_XDGConfigHome(t *testing.T) {
	xdgDir := t.TempDir()
	_ = os.Setenv("XDG_CONFIG_HOME", xdgDir)
	defer func() { _ = os.Unsetenv("XDG_CONFIG_HOME") }()
	got, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	want := filepath.Join(xdgDir, "cynork")
	if got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestConfigPath(t *testing.T) {
	dir := t.TempDir()
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Setenv("HOME", dir)
	defer func() { _, _ = os.Unsetenv("HOME"), os.Unsetenv("XDG_CONFIG_HOME") }()
	got, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	want := filepath.Join(dir, ".config", "cynork", "config.yaml")
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestConfigPath_XDGConfigHome(t *testing.T) {
	xdgDir := t.TempDir()
	_ = os.Setenv("XDG_CONFIG_HOME", xdgDir)
	defer func() { _ = os.Unsetenv("XDG_CONFIG_HOME") }()
	got, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	want := filepath.Join(xdgDir, "cynork", "config.yaml")
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestLoad_EmptyGatewayInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("gateway_url: \"\"\ntoken: t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("GatewayURL = %q, want default %q", cfg.GatewayURL, DefaultGatewayURL)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty (never loaded from YAML)", cfg.Token)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "token:") {
		t.Errorf("legacy token key should be stripped from config.yaml after load: %s", raw)
	}
}

func TestLoad_FileReadError(t *testing.T) {
	path := t.TempDir()
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when path is directory")
	}
}

func TestLoad_InvalidYAML_Variants(t *testing.T) {
	cases := []struct {
		name string
		body string
		load func(string) (*Config, error)
	}{
		{"LoadFileWithoutEnvOverrides", "gateway_url: [\n", LoadFileWithoutEnvOverrides},
		{"Load", "not: [[[ yaml", Load},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := tc.load(path)
			if err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestSave_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Setenv("HOME", dir)
	defer func() { _, _ = os.Unsetenv("HOME"), os.Unsetenv("XDG_CONFIG_HOME") }()
	if err := os.MkdirAll(filepath.Join(dir, ".config", "cynork"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{GatewayURL: "http://localhost", Token: "x"}
	if err := Save("", cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load("")
	if err != nil {
		t.Fatalf("Load after Save empty path: %v", err)
	}
	if loaded.Token != "" {
		t.Errorf("Token = %q, want empty without CYNORK_TOKEN", loaded.Token)
	}
}

func TestSave_MkdirFails(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "nested", "config.yaml")
	err := Save(path, &Config{GatewayURL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error when parent is a file")
	}
}

// TestSave_CreateTempFails ensures Save returns an error when the config dir is not writable
// (e.g. CreateTemp fails), so the caller gets a clear failure instead of a partial write.
func TestSave_CreateTempFails(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(sub, 0o700) }()
	path := filepath.Join(sub, "config.yaml")
	err := Save(path, &Config{GatewayURL: "http://localhost", Token: "t"})
	if err == nil {
		t.Fatal("expected error when config dir is not writable")
	}
}

func TestSave_WriteFails(t *testing.T) {
	path := t.TempDir()
	err := Save(path, &Config{GatewayURL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestSave_DefaultPathError(t *testing.T) {
	old := defaultConfigPath
	defer func() { defaultConfigPath = old }()
	defaultConfigPath = func() (string, error) { return "", errors.New("injected") }
	err := Save("", &Config{GatewayURL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error when defaultConfigPath fails")
	}
}

func TestConfigDirAndPath_UserHomeDirFails(t *testing.T) {
	old := userHomeDir
	defer func() { userHomeDir = old }()
	userHomeDir = func() (string, error) { return "", errors.New("injected") }
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	defer func() { _ = os.Unsetenv("XDG_CONFIG_HOME") }()
	if _, err := ConfigDir(); err == nil {
		t.Fatal("ConfigDir: expected error when UserHomeDir fails")
	}
	if _, err := ConfigPath(); err == nil {
		t.Fatal("ConfigPath: expected error when ConfigDir fails")
	}
}

func TestStripLegacyTokenKeysFromConfigFile_ReadDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("token: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(path, 0o600) }()
	err := stripLegacyTokenKeysFromConfigFile(path, &Config{GatewayURL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error when config file is not readable")
	}
}

func TestSave_RenameFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	err := Save(path, &Config{GatewayURL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error when save path is a directory")
	}
}

func TestRemoveLegacySessionFile_Explicit(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sess := filepath.Join(dir, legacySessionFileName)
	if err := os.WriteFile(sess, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	removeLegacySessionFile(configPath)
	if _, err := os.Stat(sess); !os.IsNotExist(err) {
		t.Fatal("expected legacy session file removed")
	}
}

func TestRemoveLegacySessionFile_EmptyPath(t *testing.T) {
	removeLegacySessionFile("")
}
