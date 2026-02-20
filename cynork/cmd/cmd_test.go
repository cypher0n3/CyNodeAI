package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runWithArgs(t *testing.T, args ...string) int {
	t.Helper()
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = append([]string{"cynork"}, args...)
	return Execute()
}

func mockJSONServer(t *testing.T, status int, v any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}))
}

func TestExecute_Version(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost\n")
	got := runWithArgs(t, "--config", path, "version")
	if got != 0 {
		t.Errorf("Execute() = %d, want 0", got)
	}
}

func TestExecute_StatusOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\n")
	got := runWithArgs(t, "--config", path, "status")
	if got != 0 {
		t.Errorf("Execute() = %d, want 0", got)
	}
}

func TestExecute_LoadConfigFails(t *testing.T) {
	path := t.TempDir()
	got := runWithArgs(t, "--config", path, "version")
	if got != 1 {
		t.Errorf("Execute() = %d, want 1", got)
	}
}

func TestRunStatus_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL}
	defer func() { cfg = nil }()
	if err := runStatus(nil, nil); err != nil {
		t.Errorf("runStatus: %v", err)
	}
}

func TestRunStatus_Error(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://127.0.0.1:0"}
	defer func() { cfg = nil }()
	if err := runStatus(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunAuthWhoami_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runAuthWhoami(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunAuthWhoami_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.UserResponse{ID: "u1", Handle: "alice"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runAuthWhoami(nil, nil); err != nil {
		t.Errorf("runAuthWhoami: %v", err)
	}
}

func TestRunAuthWhoami_GatewayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runAuthWhoami(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskCreate_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskCreate(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskCreate_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusCreated, gateway.TaskResponse{ID: "task-1", Status: "queued"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCreatePrompt = "echo hi"
	defer func() { cfg = nil; taskCreatePrompt = "" }()
	if err := runTaskCreate(nil, nil); err != nil {
		t.Errorf("runTaskCreate: %v", err)
	}
}

func TestRunTaskResult_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskResult(nil, []string{"tid"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskResult_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []gateway.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("out")}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runTaskResult(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskResult: %v", err)
	}
}

func TestRunAuthLogout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("gateway_url: http://localhost\ntoken: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath = path
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "x"}
	defer func() { configPath = ""; cfg = nil }()
	if err := runAuthLogout(nil, nil); err != nil {
		t.Errorf("runAuthLogout: %v", err)
	}
}

func TestRunAuthLogout_DefaultConfigPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "cynork")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("XDG_CONFIG_HOME", dir)
	defer func() { _ = os.Unsetenv("XDG_CONFIG_HOME") }()
	configPath = ""
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "x"}
	defer func() { configPath = ""; cfg = nil }()
	if err := runAuthLogout(nil, nil); err != nil {
		t.Errorf("runAuthLogout: %v", err)
	}
}

func TestRunAuthLogin_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.LoginResponse{AccessToken: "new-tok", TokenType: "Bearer"})
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = "u"
	authLoginPassword = "p"
	defer func() { configPath = ""; cfg = nil; authLoginHandle = ""; authLoginPassword = "" }()
	if err := runAuthLogin(nil, nil); err != nil {
		t.Errorf("runAuthLogin: %v", err)
	}
}

func TestRunAuthLogin_LoginFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	configPath = filepath.Join(t.TempDir(), "config.yaml")
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = "u"
	authLoginPassword = "p"
	defer func() { configPath = ""; cfg = nil; authLoginHandle = ""; authLoginPassword = "" }()
	if err := runAuthLogin(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunAuthLogin_SaveFails(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
	defer server.Close()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath = filepath.Join(blocker, "nested", "config.yaml")
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = "u"
	authLoginPassword = "p"
	defer func() { configPath = ""; cfg = nil; authLoginHandle = ""; authLoginPassword = "" }()
	if err := runAuthLogin(nil, nil); err == nil {
		t.Fatal("expected save error")
	}
}

func TestRunAuthLogin_ConfigPathFails(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
	defer server.Close()
	configPath = ""
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = "u"
	authLoginPassword = "p"
	old := getDefaultConfigPath
	getDefaultConfigPath = func() (string, error) { return "", errors.New("injected") }
	defer func() { configPath = ""; cfg = nil; authLoginHandle = ""; authLoginPassword = ""; getDefaultConfigPath = old }()
	if err := runAuthLogin(nil, nil); err == nil {
		t.Fatal("expected config path error")
	}
}

func TestRunAuthLogout_ConfigPathFails(t *testing.T) {
	configPath = ""
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "x"}
	old := getDefaultConfigPath
	getDefaultConfigPath = func() (string, error) { return "", errors.New("injected") }
	defer func() { configPath = ""; cfg = nil; getDefaultConfigPath = old }()
	if err := runAuthLogout(nil, nil); err == nil {
		t.Fatal("expected config path error")
	}
}

func TestReadPassword(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	_, _ = w.WriteString("secret\n")
	_ = w.Close()
	pass, err := readPassword("")
	if err != nil {
		t.Errorf("readPassword: %v", err)
	}
	if pass != "secret" {
		t.Errorf("password = %q, want secret", pass)
	}
}

func TestReadPassword_ScanFails(t *testing.T) {
	oldStdin := os.Stdin
	r, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	_, err = readPassword("")
	if err == nil {
		t.Fatal("expected error when stdin closed")
	}
}

func TestRunTaskCreate_GatewayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCreatePrompt = "echo hi"
	defer func() { cfg = nil; taskCreatePrompt = "" }()
	if err := runTaskCreate(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskResult_GatewayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runTaskResult(nil, []string{"tid"}); err == nil {
		t.Fatal("expected error")
	}
}

func runAuthLoginWithStdin(t *testing.T, handle, password, stdinInput string) error {
	t.Helper()
	server := mockJSONServer(t, http.StatusOK, gateway.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = handle
	authLoginPassword = password
	oldStdin := os.Stdin
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	defer func() { os.Stdin = oldStdin; configPath = ""; cfg = nil; authLoginHandle = ""; authLoginPassword = "" }()
	os.Stdin = r
	_, _ = w.WriteString(stdinInput)
	_ = w.Close()
	return runAuthLogin(nil, nil)
}

func TestRunAuthLogin_HandleFromStdin(t *testing.T) {
	if err := runAuthLoginWithStdin(t, "", "p", "stdin_user\n"); err != nil {
		t.Errorf("runAuthLogin: %v", err)
	}
}

func TestRunAuthLogin_PasswordFromStdin(t *testing.T) {
	if err := runAuthLoginWithStdin(t, "u", "", "stdin_pass\n"); err != nil {
		t.Errorf("runAuthLogin: %v", err)
	}
}

func TestRunAuthLogin_ConfigPathFromDefault(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
	defer server.Close()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cynork"), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("XDG_CONFIG_HOME", dir)
	defer func() { _ = os.Unsetenv("XDG_CONFIG_HOME") }()
	configPath = ""
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = "u"
	authLoginPassword = "p"
	defer func() { configPath = ""; cfg = nil; authLoginHandle = ""; authLoginPassword = "" }()
	if err := runAuthLogin(nil, nil); err != nil {
		t.Errorf("runAuthLogin: %v", err)
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost\n")
	got := runWithArgs(t, "--config", path, "nonexistent")
	if got != 1 {
		t.Errorf("Execute() = %d, want 1", got)
	}
}

func strPtr(s string) *string { return &s }
