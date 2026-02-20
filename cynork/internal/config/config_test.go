package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultPathError(t *testing.T) {
	old := defaultConfigPath
	defer func() { defaultConfigPath = old }()
	defaultConfigPath = func() (string, error) { return "", errors.New("injected") }
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("GatewayURL = %q, want default", cfg.GatewayURL)
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

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")
	cfg := &Config{GatewayURL: "http://saved:8080", Token: "t"}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if loaded.GatewayURL != cfg.GatewayURL || loaded.Token != cfg.Token {
		t.Errorf("loaded = %+v, want %+v", loaded, cfg)
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
	if cfg.Token != "t" {
		t.Errorf("Token = %q, want t", cfg.Token)
	}
}

func TestLoad_FileReadError(t *testing.T) {
	path := t.TempDir()
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when path is directory")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("not: [[[ yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
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
	if loaded.Token != "x" {
		t.Errorf("Token = %q, want x", loaded.Token)
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
