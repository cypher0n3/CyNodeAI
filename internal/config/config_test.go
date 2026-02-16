package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadOrchestratorConfig_Defaults(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("LISTEN_ADDR")
	os.Unsetenv("JWT_SECRET")

	cfg := LoadOrchestratorConfig()

	if cfg.DatabaseURL == "" {
		t.Error("DatabaseURL should have a default value")
	}

	if cfg.ListenAddr == "" {
		t.Error("ListenAddr should have a default value")
	}

	if cfg.JWTSecret == "" {
		t.Error("JWTSecret should have a default value")
	}

	if cfg.JWTAccessDuration == 0 {
		t.Error("JWTAccessDuration should have a default value")
	}

	if cfg.RateLimitPerMinute == 0 {
		t.Error("RateLimitPerMinute should have a default value")
	}
}

func TestLoadOrchestratorConfig_EnvOverrides(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")
	os.Setenv("LISTEN_ADDR", ":9090")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("LISTEN_ADDR")

	cfg := LoadOrchestratorConfig()

	if cfg.DatabaseURL != "postgres://test:test@localhost:5432/testdb" {
		t.Errorf("DatabaseURL = %v, want postgres://test:test@localhost:5432/testdb", cfg.DatabaseURL)
	}

	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %v, want :9090", cfg.ListenAddr)
	}
}

func TestLoadNodeConfig_Defaults(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("ORCHESTRATOR_URL")
	os.Unsetenv("NODE_SLUG")
	os.Unsetenv("NODE_LISTEN_ADDR")

	cfg := LoadNodeConfig()

	if cfg.OrchestratorURL == "" {
		t.Error("OrchestratorURL should have a default value")
	}

	if cfg.NodeSlug == "" {
		t.Error("NodeSlug should have a default value")
	}

	if cfg.ListenAddr == "" {
		t.Error("ListenAddr should have a default value")
	}

	if cfg.ContainerRuntime == "" {
		t.Error("ContainerRuntime should have a default value")
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	val := getEnv("TEST_VAR", "default")
	if val != "test-value" {
		t.Errorf("getEnv() = %v, want test-value", val)
	}

	val = getEnv("NONEXISTENT_VAR", "default")
	if val != "default" {
		t.Errorf("getEnv() = %v, want default", val)
	}
}

func TestGetIntEnv(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	val := getIntEnv("TEST_INT", 10)
	if val != 42 {
		t.Errorf("getIntEnv() = %v, want 42", val)
	}

	os.Setenv("TEST_INT", "invalid")
	val = getIntEnv("TEST_INT", 10)
	if val != 10 {
		t.Errorf("getIntEnv() with invalid value = %v, want 10", val)
	}

	val = getIntEnv("NONEXISTENT_VAR", 10)
	if val != 10 {
		t.Errorf("getIntEnv() with nonexistent var = %v, want 10", val)
	}
}

func TestGetDurationEnv(t *testing.T) {
	os.Setenv("TEST_DURATION", "5m")
	defer os.Unsetenv("TEST_DURATION")

	val := getDurationEnv("TEST_DURATION", time.Hour)
	if val != 5*time.Minute {
		t.Errorf("getDurationEnv() = %v, want 5m", val)
	}

	os.Setenv("TEST_DURATION", "invalid")
	val = getDurationEnv("TEST_DURATION", time.Hour)
	if val != time.Hour {
		t.Errorf("getDurationEnv() with invalid value = %v, want 1h", val)
	}

	val = getDurationEnv("NONEXISTENT_VAR", time.Hour)
	if val != time.Hour {
		t.Errorf("getDurationEnv() with nonexistent var = %v, want 1h", val)
	}
}
