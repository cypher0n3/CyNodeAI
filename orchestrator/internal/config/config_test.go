package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadOrchestratorConfig_Defaults(t *testing.T) {
	// Clear relevant env vars
	_ = os.Unsetenv("DATABASE_URL")
	_ = os.Unsetenv("LISTEN_ADDR")
	_ = os.Unsetenv("JWT_SECRET")
	_ = os.Unsetenv(envOrchestratorDevMode)

	cfg := LoadOrchestratorConfig()

	if !cfg.DevMode {
		t.Error("DevMode should default to true when ORCHESTRATOR_DEV_MODE is unset")
	}

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

	if cfg.OrchestratorPublicURL == "" {
		t.Error("OrchestratorPublicURL should have a default value")
	}
}

func TestValidateSecrets_nil(t *testing.T) {
	if err := ValidateSecrets(nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestValidateSecrets_devModeAllowsDefaults(t *testing.T) {
	cfg := &OrchestratorConfig{
		DevMode:                true,
		JWTSecret:              DefaultJWTSecret,
		NodeRegistrationPSK:    DefaultNodeRegistrationPSK,
		WorkerAPIBearerToken:   DefaultWorkerAPIBearerToken,
		BootstrapAdminPassword: DefaultBootstrapAdminPassword,
	}
	if err := ValidateSecrets(cfg); err != nil {
		t.Fatalf("DevMode true: %v", err)
	}
}

func TestValidateSecrets_prodRejectsDefaults(t *testing.T) {
	cfg := &OrchestratorConfig{
		DevMode:                false,
		JWTSecret:              DefaultJWTSecret,
		NodeRegistrationPSK:    DefaultNodeRegistrationPSK,
		WorkerAPIBearerToken:   DefaultWorkerAPIBearerToken,
		BootstrapAdminPassword: DefaultBootstrapAdminPassword,
	}
	err := ValidateSecrets(cfg)
	if err == nil {
		t.Fatal("expected error when DevMode is false and defaults are used")
	}
	for _, key := range []string{"JWT_SECRET", "NODE_REGISTRATION_PSK", "WORKER_API_BEARER_TOKEN", "BOOTSTRAP_ADMIN_PASSWORD"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error should mention %q: %v", key, err)
		}
	}
}

func TestValidateSecrets_prodOKWhenCustom(t *testing.T) {
	cfg := &OrchestratorConfig{
		DevMode:                false,
		JWTSecret:              "custom-jwt",
		NodeRegistrationPSK:    "custom-psk",
		WorkerAPIBearerToken:   "custom-worker",
		BootstrapAdminPassword: "custom-password",
	}
	if err := ValidateSecrets(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInsecureDefaults(t *testing.T) {
	// Plan gate: non-dev mode must not accept shipped defaults.
	cfg := LoadOrchestratorConfig()
	cfg.DevMode = false
	if err := ValidateSecrets(cfg); err == nil {
		t.Fatal("LoadOrchestratorConfig values with DevMode false should fail validation")
	}
}

func TestLoadOrchestratorConfig_EnvOverrides(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")
	_ = os.Setenv("LISTEN_ADDR", ":9090")
	defer func() { _ = os.Unsetenv("DATABASE_URL") }()
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()

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
	_ = os.Unsetenv("ORCHESTRATOR_URL")
	_ = os.Unsetenv("NODE_SLUG")
	_ = os.Unsetenv("NODE_LISTEN_ADDR")

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
	_ = os.Setenv("TEST_VAR", "test-value")
	defer func() { _ = os.Unsetenv("TEST_VAR") }()

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
	_ = os.Setenv("TEST_INT", "42")
	defer func() { _ = os.Unsetenv("TEST_INT") }()

	val := getIntEnv("TEST_INT", 10)
	if val != 42 {
		t.Errorf("getIntEnv() = %v, want 42", val)
	}

	_ = os.Setenv("TEST_INT", "invalid")
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
	_ = os.Setenv("TEST_DURATION", "5m")
	defer func() { _ = os.Unsetenv("TEST_DURATION") }()

	val := getDurationEnv("TEST_DURATION", time.Hour)
	if val != 5*time.Minute {
		t.Errorf("getDurationEnv() = %v, want 5m", val)
	}

	_ = os.Setenv("TEST_DURATION", "invalid")
	val = getDurationEnv("TEST_DURATION", time.Hour)
	if val != time.Hour {
		t.Errorf("getDurationEnv() with invalid value = %v, want 1h", val)
	}

	val = getDurationEnv("NONEXISTENT_VAR", time.Hour)
	if val != time.Hour {
		t.Errorf("getDurationEnv() with nonexistent var = %v, want 1h", val)
	}
}

func TestGetBoolEnv(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "on"} {
		_ = os.Setenv("TEST_BOOL", v)
		if !getBoolEnv("TEST_BOOL", false) {
			t.Errorf("getBoolEnv(%q) = false, want true", v)
		}
		_ = os.Unsetenv("TEST_BOOL")
	}
	for _, v := range []string{"0", "false", "no", "off"} {
		_ = os.Setenv("TEST_BOOL", v)
		if getBoolEnv("TEST_BOOL", true) {
			t.Errorf("getBoolEnv(%q) = true, want false", v)
		}
		_ = os.Unsetenv("TEST_BOOL")
	}
	_ = os.Unsetenv("TEST_BOOL")
	if getBoolEnv("TEST_BOOL", true) != true {
		t.Error("getBoolEnv() with unset var = false, want default true")
	}
	if getBoolEnv("TEST_BOOL", false) != false {
		t.Error("getBoolEnv() with unset var = true, want default false")
	}
}
