//nolint:dupl // table-driven and similar test patterns are intentional
package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

const testStatusCompleted = "completed"
const chatCompletionsPath = "/v1/chat/completions"

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
	if got != 2 {
		t.Errorf("Execute() = %d, want 2 (usage)", got)
	}
}

func TestRunStatus_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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
	server := mockJSONServer(t, http.StatusOK, userapi.UserResponse{ID: "u1", Handle: "alice"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runAuthWhoami(nil, nil); err != nil {
		t.Errorf("runAuthWhoami: %v", err)
	}
}

func TestRunAuthRefresh_NoRefreshToken(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost"}
	defer func() { cfg = nil }()
	if err := runAuthRefresh(nil, nil); err == nil {
		t.Fatal("expected error when no refresh token")
	}
}

func TestRunAuthRefresh_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	})
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\ntoken: old\nrefresh_token: old-refresh\n")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL, Token: "old", RefreshToken: "old-refresh"}
	defer func() { configPath = ""; cfg = nil }()
	if err := runAuthRefresh(nil, nil); err != nil {
		t.Errorf("runAuthRefresh: %v", err)
	}
	if cfg.Token != "new-access" || cfg.RefreshToken != "new-refresh" {
		t.Errorf("cfg not updated: token=%q refresh_token=%q", cfg.Token, cfg.RefreshToken)
	}
}

func TestRunAuthRefresh_DefaultConfigPath(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{
		AccessToken:  "new-a",
		RefreshToken: "new-r",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	})
	defer server.Close()
	configPath = ""
	cfg = &config.Config{GatewayURL: server.URL, Token: "old", RefreshToken: "old-r"}
	oldGetDefault := getDefaultConfigPath
	getDefaultConfigPath = func() (string, error) { return writeTempConfig(t, "gateway_url: http://x\n"), nil }
	defer func() { configPath = ""; cfg = nil; getDefaultConfigPath = oldGetDefault }()
	if err := runAuthRefresh(nil, nil); err != nil {
		t.Errorf("runAuthRefresh: %v", err)
	}
}

func TestRunModelsList_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runModelsList(nil, nil); err == nil {
		t.Fatal("expected error when no token")
	}
}

func TestRunModelsList_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.ListModelsResponse{
		Object: "list",
		Data:   []gateway.ListModelEntry{{ID: "cynodeai.pm", Object: "model", Created: 0}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runModelsList(nil, nil); err != nil {
		t.Errorf("runModelsList: %v", err)
	}
}

func TestRunModelsList_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.ListModelsResponse{
		Object: "list",
		Data:   []gateway.ListModelEntry{{ID: "m1", Object: "model", Created: 0}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runModelsList(nil, nil); err != nil {
		t.Errorf("runModelsList: %v", err)
	}
}

func TestRunChat_WithMessageFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == chatCompletionsPath && r.Method == http.MethodPost {
			var req userapi.ChatCompletionsRequest
			if _ = json.NewDecoder(r.Body).Decode(&req); len(req.Messages) > 0 {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{
						{"message": map[string]any{"role": "assistant", "content": "one-shot reply"}},
					},
				})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	chatMessage = "hello"
	chatPlain = true
	defer func() { cfg = nil; chatMessage = ""; chatPlain = false }()
	if err := runChat(nil, nil); err != nil {
		t.Errorf("runChat(--message): %v", err)
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
	server := mockJSONServer(t, http.StatusCreated, userapi.TaskResponse{ID: "task-1", Status: "queued"})
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
	server := mockJSONServer(t, http.StatusOK, userapi.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("out")}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runTaskResult(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskResult: %v", err)
	}
}

func TestRunTaskWatch_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskWatch(nil, []string{"tid"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskWatch_ExitsOnTerminalStatus(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("done")}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskWatchNoClear = true
	defer func() { cfg = nil; taskWatchNoClear = false }()
	if err := runTaskWatch(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskWatch: %v", err)
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
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{AccessToken: "new-tok", TokenType: "Bearer"})
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
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
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
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
	defer server.Close()
	configPath = ""
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = "u"
	authLoginPassword = "p"
	old := getDefaultConfigPath
	getDefaultConfigPath = func() (string, error) { return "", errors.New("injected") }
	defer func() {
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPassword = ""
		getDefaultConfigPath = old
	}()
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
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
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
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
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

func TestRunTaskList_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskList(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskList_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.ListTasksResponse{
		Tasks: []userapi.TaskResponse{{ID: "t1", Status: "completed"}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatTable
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskList(nil, nil); err != nil {
		t.Errorf("runTaskList: %v", err)
	}
}

func TestRunTaskGet_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskGet(nil, []string{"tid"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskGet_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.TaskResponse{ID: "tid", Status: "running"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatTable
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskGet(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskGet: %v", err)
	}
}

func TestRunTaskCancel_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskCancel(nil, []string{"tid"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskCancel_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.CancelTaskResponse{TaskID: "tid", Canceled: true})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCancelYes = true
	defer func() { cfg = nil; taskCancelYes = false }()
	if err := runTaskCancel(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskCancel: %v", err)
	}
}

func TestRunTaskLogs_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskLogs(nil, []string{"tid"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskLogs_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.TaskLogsResponse{TaskID: "tid", Stdout: "out", Stderr: "err"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runTaskLogs(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskLogs: %v", err)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"a b", []string{"a", "b"}},
		{`a "b c" d`, []string{"a", "b c", "d"}},
		{"task list", []string{"task", "list"}},
		{"create --help", []string{"create", "--help"}},
		{"create -p say hello", []string{"create", "-p", "say", "hello"}},
		{`create "hello world"`, []string{"create", "hello world"}},
	}
	for _, tt := range tests {
		got := parseArgs(tt.line)
		if len(got) != len(tt.want) {
			t.Errorf("parseArgs(%q) = %v, want %v", tt.line, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseArgs(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFormatChatResponse_Plain(t *testing.T) {
	got, err := formatChatResponse("hello", true, false)
	if err != nil {
		t.Fatalf("formatChatResponse(plain): %v", err)
	}
	if got != "hello\n" {
		t.Errorf("plain: got %q", got)
	}
}

func TestFormatChatResponse_Formatted(t *testing.T) {
	got, err := formatChatResponse("**bold**", false, true)
	if err != nil {
		t.Fatalf("formatChatResponse(formatted): %v", err)
	}
	if got == "" {
		t.Error("formatted: got empty")
	}
	if !strings.Contains(got, "bold") {
		t.Errorf("formatted output should contain 'bold', got %q", got)
	}
}

func TestFormatChatResponse_FormattedAutoStyle(t *testing.T) {
	got, err := formatChatResponse("text", false, false)
	if err != nil {
		t.Fatalf("formatChatResponse(auto): %v", err)
	}
	if got == "" {
		t.Error("formatted: got empty")
	}
}

func TestRunChat_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runChat(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunCredsList_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runCredsList(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunCredsList_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runCredsList(nil, nil); err != nil {
		t.Errorf("runCredsList: %v", err)
	}
}

func TestRunNodesList_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runNodesList(nil, nil); err != nil {
		t.Errorf("runNodesList: %v", err)
	}
}

func TestRunAuditList_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runAuditList(nil, nil); err != nil {
		t.Errorf("runAuditList: %v", err)
	}
}

func TestRunPrefsSet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runPrefsSet(nil, nil); err != nil {
		t.Errorf("runPrefsSet: %v", err)
	}
}

func TestRunPrefsGet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runPrefsGet(nil, nil); err != nil {
		t.Errorf("runPrefsGet: %v", err)
	}
}

func TestRunSettingsSet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runSettingsSet(nil, nil); err != nil {
		t.Errorf("runSettingsSet: %v", err)
	}
}

func TestRunSettingsGet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runSettingsGet(nil, nil); err != nil {
		t.Errorf("runSettingsGet: %v", err)
	}
}

func TestRunSkillsLoad_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runSkillsLoad(nil, []string{"tmp/skill.md"}); err != nil {
		t.Errorf("runSkillsLoad: %v", err)
	}
}

func TestRunTaskCreate_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusCreated, userapi.TaskResponse{ID: "tid", Status: "queued"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCreatePrompt = "hi"
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; taskCreatePrompt = ""; outputFmt = "" }()
	if err := runTaskCreate(nil, nil); err != nil {
		t.Errorf("runTaskCreate: %v", err)
	}
}

func TestRunTaskResult_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("out")}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskResult(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskResult: %v", err)
	}
}

func TestRunTaskGet_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.TaskResponse{ID: "tid", Status: "running"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskGet(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskGet: %v", err)
	}
}

func TestRunTaskList_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.ListTasksResponse{Tasks: []userapi.TaskResponse{}})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskList(nil, nil); err != nil {
		t.Errorf("runTaskList: %v", err)
	}
}

func TestRunTaskCancel_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.CancelTaskResponse{TaskID: "tid", Canceled: true})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCancelYes = true
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; taskCancelYes = false; outputFmt = "" }()
	if err := runTaskCancel(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskCancel: %v", err)
	}
}

func TestRunStatus_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runStatus(nil, nil); err != nil {
		t.Errorf("runStatus: %v", err)
	}
}

func TestRunTaskGet_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"forbidden","status":403}`))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	err := runTaskGet(nil, []string{"tid"})
	if err == nil {
		t.Fatal("expected error")
	}
	if exit.CodeOf(err) != 3 {
		t.Errorf("exit code = %d, want 3 (auth)", exit.CodeOf(err))
	}
}

func TestRunTaskResult_WaitTerminal(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("done")}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskResultWait = true
	defer func() { cfg = nil; taskResultWait = false }()
	if err := runTaskResult(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskResult --wait: %v", err)
	}
}

func TestExecute_ShellC(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost\n")
	got := runWithArgs(t, "--config", path, "shell", "-c", "version")
	if got != 0 {
		t.Errorf("Execute(shell -c version) = %d, want 0", got)
	}
}

func TestRunTaskList_WithStatus(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.ListTasksResponse{Tasks: []userapi.TaskResponse{}})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskListStatus = testStatusCompleted
	defer func() { cfg = nil; taskListStatus = "" }()
	if err := runTaskList(nil, nil); err != nil {
		t.Errorf("runTaskList: %v", err)
	}
}

func TestExitFromGatewayErr_Validation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"invalid","status":422}`))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCreatePrompt = "test"
	defer func() { cfg = nil; taskCreatePrompt = "" }()
	err := runTaskCreate(nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if exit.CodeOf(err) != 6 {
		t.Errorf("exit code = %d, want 6 (validation)", exit.CodeOf(err))
	}
}

func strPtr(s string) *string { return &s }

func TestRunStubList_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runStubList("/v1/audit"); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestRunStubList_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"a1"}]`))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubList("/v1/audit"); err != nil {
		t.Errorf("runStubList: %v", err)
	}
}

func TestRunStubList_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// empty body -> runStubFetch uses default "[]"
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubList("/v1/audit"); err != nil {
		t.Errorf("runStubList: %v", err)
	}
}

func TestRunStubList_GatewayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubList("/v1/audit"); err == nil {
		t.Fatal("expected gateway error")
	}
}

func TestRunStubGet_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runStubGet("/v1/prefs/effective"); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestRunStubGet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"val"}`))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubGet("/v1/prefs/effective"); err != nil {
		t.Errorf("runStubGet: %v", err)
	}
}

func TestRunStubSet_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runStubSet("/v1/prefs"); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestRunStubSet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubSet("/v1/prefs"); err != nil {
		t.Errorf("runStubSet: %v", err)
	}
}

func TestRunAuditList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runAuditList(nil, nil); err != nil {
		t.Errorf("runAuditList: %v", err)
	}
}

func TestRunCredsList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runCredsList(nil, nil); err != nil {
		t.Errorf("runCredsList: %v", err)
	}
}

func TestRunNodesList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runNodesList(nil, nil); err != nil {
		t.Errorf("runNodesList: %v", err)
	}
}

func TestRunPrefsSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runPrefsSet(nil, nil); err != nil {
		t.Errorf("runPrefsSet: %v", err)
	}
}

func TestRunPrefsGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runPrefsGet(nil, nil); err != nil {
		t.Errorf("runPrefsGet: %v", err)
	}
}

func TestRunSettingsSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runSettingsSet(nil, nil); err != nil {
		t.Errorf("runSettingsSet: %v", err)
	}
}

func TestRunSettingsGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runSettingsGet(nil, nil); err != nil {
		t.Errorf("runSettingsGet: %v", err)
	}
}

func TestRunChat_OneMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == chatCompletionsPath && r.Method == http.MethodPost {
			var req userapi.ChatCompletionsRequest
			if _ = json.NewDecoder(r.Body).Decode(&req); len(req.Messages) > 0 && req.Messages[0].Content == "hello" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{
						{"message": map[string]any{"role": "assistant", "content": "reply"}},
					},
				})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	_, _ = w.WriteString("hello\n/exit\n")
	_ = w.Close()
	if err := runChat(nil, nil); err != nil {
		t.Errorf("runChat: %v", err)
	}
}

func TestRunChat_ChatFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	_, _ = w.WriteString("hi\n/exit\n")
	_ = w.Close()
	err = runChat(nil, nil)
	if err == nil {
		t.Fatal("expected error from chat")
	}
}

func TestRunSkillsLoad_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runSkillsLoad(nil, []string{"file.md"}); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestRunShell_SingleCommand(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost\n")
	shellCommand = "--config " + path + " version"
	defer func() { shellCommand = "" }()
	args := parseArgs(shellCommand)
	oldArgs := os.Args
	os.Args = append([]string{"cynork"}, args...)
	defer func() { os.Args = oldArgs }()
	if err := runShell(nil, nil); err != nil {
		t.Errorf("runShell: %v", err)
	}
}

func TestRunShell_InteractiveOneLine(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost\n")
	shellCommand = ""
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin; shellCommand = "" }()
	os.Stdin = r
	_, _ = w.WriteString("--config " + path + " version\n")
	_ = w.Close()
	if err := runShell(nil, nil); err != nil {
		t.Errorf("runShell: %v", err)
	}
}

func TestExitFromGatewayErr_StatusCodes(t *testing.T) {
	// 404 -> NotFound
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"detail":"not found","status":404}`))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runTaskGet(nil, []string{"missing"}); err == nil {
		t.Fatal("expected error")
	}
	// 409 -> Conflict
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"detail":"conflict","status":409}`))
	}))
	defer server2.Close()
	cfg = &config.Config{GatewayURL: server2.URL, Token: "tok"}
	taskCreatePrompt = "test"
	defer func() { taskCreatePrompt = "" }()
	if err := runTaskCreate(nil, nil); err == nil {
		t.Fatal("expected conflict error")
	}
	// 422 -> Validation
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"validation","status":422}`))
	}))
	defer server3.Close()
	cfg = &config.Config{GatewayURL: server3.URL, Token: "tok"}
	if err := runTaskList(nil, nil); err == nil {
		t.Fatal("expected validation error")
	}
	// 500 -> Gateway (default)
	server4 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server4.Close()
	cfg = &config.Config{GatewayURL: server4.URL, Token: "tok"}
	if err := runTaskList(nil, nil); err == nil {
		t.Fatal("expected gateway error")
	}
	cfg = nil
}

func TestRunTaskCancel_UserDeclines(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	taskCancelYes = false
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = w.WriteString("n\n")
	_ = w.Close()
	defer func() { os.Stdin = oldStdin; cfg = nil; taskCancelYes = false }()
	if err := runTaskCancel(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskCancel (decline): %v", err)
	}
}

func TestRunTaskResult_Wait(t *testing.T) {
	first := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/tasks/tid/result" {
			if first {
				first = false
				_ = json.NewEncoder(w).Encode(userapi.TaskResultResponse{TaskID: "tid", Status: "running", Jobs: []userapi.JobResponse{}})
				return
			}
			_ = json.NewEncoder(w).Encode(userapi.TaskResultResponse{
				TaskID: "tid", Status: "completed",
				Jobs: []userapi.JobResponse{{Result: strPtr("done")}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskResultWait = true
	taskResultWaitInterval = 1 * time.Millisecond
	defer func() { cfg = nil; taskResultWait = false; taskResultWaitInterval = 2 * time.Second }()
	if err := runTaskResult(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskResult --wait: %v", err)
	}
}

func TestParseSlash(t *testing.T) {
	tests := []struct {
		line     string
		wantCmd  string
		wantRest string
		wantOK   bool
	}{
		{"", "", "", false},
		{"hello", "", "", false},
		{"/", "", "", true},
		{"/help", "help", "", true},
		{"/Help", "help", "", true},
		{"/task list", "task", "list", true},
		{"/task list --limit 5", "task", "list --limit 5", true},
	}
	for _, tt := range tests {
		cmd, rest, ok := parseSlash(tt.line)
		if ok != tt.wantOK || cmd != tt.wantCmd || rest != tt.wantRest {
			t.Errorf("parseSlash(%q) = %q, %q, %v; want %q, %q, %v", tt.line, cmd, rest, ok, tt.wantCmd, tt.wantRest, tt.wantOK)
		}
	}
}

func TestRunSlashCommand_HelpAndVersion(t *testing.T) {
	client := gateway.NewClient("http://localhost")
	client.SetToken("tok")
	exitSession, err := runSlashCommand(client, "/help")
	if err != nil {
		t.Errorf("runSlashCommand /help: %v", err)
	}
	if exitSession {
		t.Error("runSlashCommand /help should not exit session")
	}
	exitSession, err = runSlashCommand(client, "/version")
	if err != nil {
		t.Errorf("runSlashCommand /version: %v", err)
	}
	if exitSession {
		t.Error("runSlashCommand /version should not exit session")
	}
}

func TestRunSlashCommand_Exit(t *testing.T) {
	client := gateway.NewClient("http://localhost")
	exitSession, _ := runSlashCommand(client, "/exit")
	if !exitSession {
		t.Error("runSlashCommand /exit should set exitSession true")
	}
	exitSession, _ = runSlashCommand(client, "/quit")
	if !exitSession {
		t.Error("runSlashCommand /quit should set exitSession true")
	}
}

func TestRunSlashCommand_Unknown(t *testing.T) {
	client := gateway.NewClient("http://localhost")
	exitSession, err := runSlashCommand(client, "/unknown")
	if err != nil {
		t.Errorf("runSlashCommand /unknown: %v", err)
	}
	if exitSession {
		t.Error("runSlashCommand /unknown should not exit session")
	}
}

func TestProcessChatLine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == chatCompletionsPath && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"role": "assistant", "content": "hi"}},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	client := gateway.NewClient(server.URL)
	client.SetToken(cfg.Token)
	exitSession, err := processChatLine(client, "")
	if err != nil || exitSession {
		t.Errorf("processChatLine empty: exit=%v err=%v", exitSession, err)
	}
	exitSession, err = processChatLine(client, "/help")
	if err != nil || exitSession {
		t.Errorf("processChatLine /help: exit=%v err=%v", exitSession, err)
	}
	exitSession, err = processChatLine(client, "hello")
	if err != nil || exitSession {
		t.Errorf("processChatLine hello: exit=%v err=%v", exitSession, err)
	}
}

func TestProcessChatLine_ShellEscape(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	client := gateway.NewClient("http://localhost")
	client.SetToken("tok")
	// ! echo hi -> output to stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()
	done := make(chan struct{})
	var out bytes.Buffer
	go func() {
		_, _ = io.Copy(&out, r)
		close(done)
	}()
	exitSession, err := processChatLine(client, "! echo hi")
	_ = w.Close()
	<-done
	if err != nil || exitSession {
		t.Errorf("processChatLine ! echo hi: exit=%v err=%v", exitSession, err)
	}
	if !strings.Contains(out.String(), "hi") {
		t.Errorf("expected stdout to contain hi, got %q", out.String())
	}
	// ! with empty command -> usage to stderr
	r2, w2, _ := os.Pipe()
	oldStderr := os.Stderr
	os.Stderr = w2
	var errOut bytes.Buffer
	done2 := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errOut, r2)
		close(done2)
	}()
	_, _ = processChatLine(client, "!")
	_ = w2.Close()
	<-done2
	os.Stderr = oldStderr
	if !strings.Contains(errOut.String(), "usage") {
		t.Errorf("expected stderr to contain usage for empty !, got %q", errOut.String())
	}
	// ! false -> exit status 1 to stderr, session continues
	r3, w3, _ := os.Pipe()
	os.Stderr = w3
	var errOut2 bytes.Buffer
	done3 := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errOut2, r3)
		close(done3)
	}()
	exitSession, err = processChatLine(client, "! false")
	_ = w3.Close()
	<-done3
	os.Stderr = oldStderr
	if err != nil || exitSession {
		t.Errorf("! false must not exit session: exit=%v err=%v", exitSession, err)
	}
	if !strings.Contains(errOut2.String(), "exit status 1") {
		t.Errorf("expected stderr to contain exit status 1, got %q", errOut2.String())
	}
}

func TestProcessChatLine_SlashErrorDoesNotExit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\ntoken: tok\n")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	oldRunner := runCynorkSubcommandForSlash
	runCynorkSubcommandForSlash = runCynorkSubcommandInProcess
	defer func() { runCynorkSubcommandForSlash = oldRunner; configPath = ""; cfg = nil }()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	var errOut bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errOut, r)
		close(done)
	}()
	exitSession, err := processChatLine(client, "/skills list")
	_ = w.Close()
	<-done
	os.Stderr = oldStderr
	if err != nil {
		t.Errorf("slash error must not return err (spec: session continues): %v", err)
	}
	if exitSession {
		t.Error("slash error must not set exitSession true")
	}
	if !strings.Contains(errOut.String(), "404") && !strings.Contains(errOut.String(), "Not Found") {
		t.Errorf("error should be printed to stderr, got %q", errOut.String())
	}
}

func TestAllSlashCommands(t *testing.T) {
	cmds := AllSlashCommands()
	if len(cmds) == 0 {
		t.Fatal("AllSlashCommands() should not be empty")
	}
	found := false
	for _, c := range cmds {
		if c.Name == "/help" && c.Description != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AllSlashCommands() should include /help with description")
	}
}

func TestSlashCompleter(t *testing.T) {
	out := slashCompleter("/")
	if len(out) == 0 {
		t.Fatal("slashCompleter(\"/\") should return commands")
	}
	out = slashCompleter("/ta")
	if len(out) != 1 || out[0] != "/task" {
		t.Errorf("slashCompleter(\"/ta\") = %v", out)
	}
	out = slashCompleter("hello")
	if len(out) != 0 {
		t.Errorf("slashCompleter(\"hello\") = %v, want nil", out)
	}
}

func TestRunTaskArtifactsList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/artifacts") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"artifact_id":"a1","name":"out.txt"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runTaskArtifactsList(nil, []string{"task-1"}); err != nil {
		t.Errorf("runTaskArtifactsList: %v", err)
	}
}

func TestRunSlashCommand_StatusWhoami(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		if r.URL.Path == "/v1/users/me" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "u1", "handle": "alice"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\ntoken: tok\n")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	oldRunner := runCynorkSubcommandForSlash
	runCynorkSubcommandForSlash = runCynorkSubcommandInProcess
	defer func() { runCynorkSubcommandForSlash = oldRunner; configPath = ""; cfg = nil }()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	_, err := runSlashCommand(client, "/status")
	if err != nil {
		t.Errorf("runSlashCommand /status: %v", err)
	}
	_, err = runSlashCommand(client, "/whoami")
	if err != nil {
		t.Errorf("runSlashCommand /whoami: %v", err)
	}
}

func TestRunSlashCommand_Models(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "gpt-4"}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\ntoken: tok\n")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	oldRunner := runCynorkSubcommandForSlash
	runCynorkSubcommandForSlash = runCynorkSubcommandInProcess
	defer func() { runCynorkSubcommandForSlash = oldRunner; configPath = ""; cfg = nil }()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	_, err := runSlashCommand(client, "/models")
	if err != nil {
		t.Errorf("runSlashCommand /models: %v", err)
	}
}

func stubSlashServeAuth(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/v1/users/me" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "u1", "handle": "alice"})
		return true
	}
	return false
}

func stubSlashServerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if stubSlashServeAuth(w, r) || stubSlashServeNodes(w, r) || stubSlashServePrefs(w, r) || stubSlashServeProjects(w, r) || stubSlashServeTasks(w, r) || stubSlashServeSkills(w, r) {
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func stubSlashServeNodes(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/v1/nodes" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
		return true
	}
	if r.URL.Path == "/v1/nodes/n1" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return true
	}
	return false
}

func stubSlashServeSkills(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/v1/skills" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
		return true
	}
	if r.URL.Path == "/v1/skills/s1" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return true
	}
	return false
}

func stubSlashServePrefs(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/v1/prefs" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
		return true
	}
	if r.URL.Path == "/v1/prefs/effective" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return true
	}
	if r.URL.Path == "/v1/prefs" && r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return true
	}
	return false
}

func stubSlashServeProjects(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/v1/projects" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
		return true
	}
	if r.URL.Path == "/v1/projects/p1" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return true
	}
	if r.URL.Path == "/v1/projects/set" && r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return true
	}
	return false
}

func stubSlashServeTasks(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/v1/tasks" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"tasks": []any{}})
		return true
	}
	if r.URL.Path == "/v1/tasks" && r.Method == http.MethodPost {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "t-new", "id": "t-new", "status": "queued"})
		return true
	}
	if r.URL.Path == "/v1/tasks/t1" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "t1", "id": "t1", "status": "completed"})
		return true
	}
	if r.URL.Path == "/v1/tasks/t1/result" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "t1", "status": "completed", "jobs": []any{}})
		return true
	}
	if r.URL.Path == "/v1/tasks/t1/cancel" && r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "t1", "canceled": true})
		return true
	}
	if r.URL.Path == "/v1/tasks/t1/logs" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"stdout": "", "stderr": ""})
		return true
	}
	if r.URL.Path == "/v1/tasks/t1/artifacts" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
		return true
	}
	return false
}

func TestRunSlashCommand_StubEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(stubSlashServerHandler))
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\ntoken: tok\n")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	oldRunner := runCynorkSubcommandForSlash
	runCynorkSubcommandForSlash = runCynorkSubcommandInProcess
	defer func() { runCynorkSubcommandForSlash = oldRunner; configPath = ""; cfg = nil }()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	cmds := []string{
		"/auth whoami", "/nodes list", "/nodes get n1", "/skills list", "/skills get s1",
		"/prefs list", "/prefs get", "/prefs effective", "/prefs delete",
		"/project list", "/project get p1", "/project", "/project set p2",
		"/model", "/model m1", "/task list", "/task get t1",
		"/task create -p say hello", "/task cancel -y t1", "/task result t1", "/task logs t1", "/task artifacts list t1",
		"/clear",
	}
	for _, cmd := range cmds {
		_, err := runSlashCommand(client, cmd)
		if err != nil {
			t.Errorf("runSlashCommand %q: %v", cmd, err)
		}
	}
}

func TestRunSlashCommand_UsagePaths(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost\ntoken: tok\n")
	configPath = path
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	oldRunner := runCynorkSubcommandForSlash
	runCynorkSubcommandForSlash = runCynorkSubcommandInProcess
	defer func() { runCynorkSubcommandForSlash = oldRunner; configPath = ""; cfg = nil }()
	client := gateway.NewClient("http://localhost")
	client.SetToken("tok")
	// Exercise usage/error paths that don't call gateway.
	for _, cmd := range []string{
		"/task", "/task foo", "/task create", "/task cancel", "/task result", "/task logs",
		"/task artifacts", "/task artifacts list", "/task result --wait",
		"/auth", "/auth foo",
		"/nodes", "/nodes foo x", "/skills", "/skills get", "/prefs", "/project get",
		"/project help", "/project --help",
	} {
		_, err := runSlashCommand(client, cmd)
		if err != nil {
			t.Errorf("runSlashCommand %q: %v", cmd, err)
		}
	}
}

func TestRunCynorkSubcommand_ExecPath(t *testing.T) {
	// Cover runCynorkSubcommand (exec path) by using "true" as the executable so the child exits 0.
	path := writeTempConfig(t, "gateway_url: http://localhost\ntoken: tok\n")
	configPath = path
	defer func() { configPath = "" }()
	oldExe := getCynorkExeForSubcommand
	oldRunner := runCynorkSubcommandForSlash
	getCynorkExeForSubcommand = func() (string, error) {
		p, err := exec.LookPath("true")
		if err != nil {
			t.Skip("true not in PATH:", err)
		}
		return p, nil
	}
	runCynorkSubcommandForSlash = runCynorkSubcommand
	defer func() { getCynorkExeForSubcommand = oldExe; runCynorkSubcommandForSlash = oldRunner }()
	_, err := runSlashCommand(nil, "/task create --help")
	if err != nil {
		t.Errorf("runSlashCommand /task create --help (exec path): %v", err)
	}
}

func TestRunSlashAuth_LoginRefreshLogoutUpdateClient(t *testing.T) {
	// Delegated auth: run subcommand in process then sync client token.
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{
		AccessToken: "new-tok", TokenType: "Bearer", ExpiresIn: 900,
	})
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\n")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL}
	oldRunner := runCynorkSubcommandForSlash
	runCynorkSubcommandForSlash = runCynorkSubcommandInProcess
	defer func() { runCynorkSubcommandForSlash = oldRunner; configPath = ""; cfg = nil }()
	client := gateway.NewClient(server.URL)
	client.SetToken("old-tok")
	if err := runSlashAuthDelegated(client, "login -u u -p p"); err != nil {
		t.Errorf("runSlashAuthDelegated login: %v", err)
	}
	if cfg.Token != "new-tok" {
		t.Errorf("cfg.Token = %q", cfg.Token)
	}

	cfg.Token = "x"
	cfg.RefreshToken = ""
	if err := runSlashAuthDelegated(client, "logout"); err != nil {
		t.Errorf("runSlashAuthDelegated logout: %v", err)
	}

	refreshServer := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{
		AccessToken: "refreshed-tok", RefreshToken: "new-refresh", TokenType: "Bearer", ExpiresIn: 900,
	})
	defer refreshServer.Close()
	cfg.GatewayURL = refreshServer.URL
	cfg.Token = "old"
	cfg.RefreshToken = "old-refresh"
	path2 := writeTempConfig(t, "gateway_url: "+refreshServer.URL+"\nrefresh_token: old-refresh\n")
	configPath = path2
	client2 := gateway.NewClient(refreshServer.URL)
	client2.SetToken("old")
	if err := runSlashAuthDelegated(client2, "refresh"); err != nil {
		t.Errorf("runSlashAuthDelegated refresh: %v", err)
	}
	if cfg.Token != "refreshed-tok" {
		t.Errorf("cfg.Token after refresh = %q", cfg.Token)
	}
}

func TestPrintJSONOrRaw(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	done := make(chan struct{})
	var out bytes.Buffer
	go func() {
		_, _ = io.Copy(&out, r)
		close(done)
	}()
	printJSONOrRaw(nil)
	printJSONOrRaw([]byte("   "))
	_ = w.Close()
	<-done
	if out.Len() != 0 {
		t.Errorf("empty input should produce no output, got %q", out.String())
	}
	// Invalid JSON: printed as-is
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	out.Reset()
	done2 := make(chan struct{})
	go func() {
		_, _ = io.Copy(&out, r2)
		close(done2)
	}()
	printJSONOrRaw([]byte("not json"))
	_ = w2.Close()
	<-done2
	if out.String() != "not json" {
		t.Errorf("invalid JSON should print raw, got %q", out.String())
	}
	// Valid JSON
	r3, w3, _ := os.Pipe()
	os.Stdout = w3
	out.Reset()
	done3 := make(chan struct{})
	go func() {
		_, _ = io.Copy(&out, r3)
		close(done3)
	}()
	printJSONOrRaw([]byte(`{"a":1}`))
	_ = w3.Close()
	<-done3
	if out.Len() == 0 || !strings.Contains(out.String(), "a") {
		t.Errorf("valid JSON should be printed, got %q", out.String())
	}
}

func TestExitFromGatewayErr(t *testing.T) {
	if exitFromGatewayErr(nil) != nil {
		t.Error("nil should return nil")
	}
	plain := errors.New("plain")
	if err := exitFromGatewayErr(plain); err == nil {
		t.Error("non-HTTPError should return non-nil")
	}
	for _, status := range []int{401, 403, 404, 409, 400, 422, 500} {
		he := &gateway.HTTPError{Status: status, Err: errors.New("e")}
		if err := exitFromGatewayErr(he); err == nil {
			t.Errorf("status %d should return non-nil", status)
		}
	}
}

func TestRunProjectSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/projects/set" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runProjectSet(nil, []string{"p1"}); err != nil {
		t.Errorf("runProjectSet: %v", err)
	}
}

func TestRunChatLoopWithReader(t *testing.T) {
	lines := []string{"/help", "/exit"}
	i := 0
	chatLineReader = func(prompt string) (string, error) {
		if i >= len(lines) {
			return "", io.EOF
		}
		s := lines[i]
		i++
		return s, nil
	}
	defer func() { chatLineReader = nil }()
	client := gateway.NewClient("http://localhost")
	client.SetToken("tok")
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	err := runChatLoopLiner(client)
	if err != nil {
		t.Errorf("runChatLoopLiner with injected reader: %v", err)
	}
}

func TestRunChatLoopLiner_WithLinerGetLine(t *testing.T) {
	chatLinerGetLine = func(prompt string) (string, error) {
		return "/exit", nil
	}
	defer func() { chatLinerGetLine = nil }()
	client := gateway.NewClient("http://localhost")
	client.SetToken("tok")
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	err := runChatLoopLiner(client)
	if err != nil {
		t.Errorf("runChatLoopLiner with chatLinerGetLine: %v", err)
	}
}

func TestRunChatLoopWithReader_EOFImmediate(t *testing.T) {
	chatLineReader = func(prompt string) (string, error) {
		return "", io.EOF
	}
	defer func() { chatLineReader = nil }()
	client := gateway.NewClient("http://localhost")
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	err := runChatLoopLiner(client)
	if err != nil {
		t.Errorf("runChatLoopLiner EOF: %v", err)
	}
}

func TestRunChat_ScannerEOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	if err := runChat(nil, nil); err != nil {
		t.Errorf("runChat with EOF stdin: %v", err)
	}
}

func TestRunChat_LinerPathWithPTY(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		t.Skipf("pty.Open: %v (skip on non-Unix or no PTY)", err)
	}
	defer func() { _ = ptyMaster.Close(); _ = ptySlave.Close() }()
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	os.Stdin = ptySlave
	os.Stdout = ptySlave
	defer func() { os.Stdin = oldStdin; os.Stdout = oldStdout }()
	var chatErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		chatErr = runChat(nil, nil)
	}()
	_, _ = ptyMaster.WriteString("/exit\n")
	wg.Wait()
	if chatErr != nil {
		t.Errorf("runChat with PTY: %v", chatErr)
	}
}

func TestSendAndPrintChat_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == chatCompletionsPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"role": "assistant", "content": ""}},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	if err := sendAndPrintChat(client, "hi"); err != nil {
		t.Errorf("sendAndPrintChat empty response: %v", err)
	}
}

func TestSendAndPrintChat_FormatErrorFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == chatCompletionsPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"role": "assistant", "content": "fallback"}},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	old := formatChatResponseFn
	formatChatResponseFn = func(string, bool, bool) (string, error) { return "", errors.New("injected") }
	defer func() { formatChatResponseFn = old }()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	if err := sendAndPrintChat(client, "hi"); err != nil {
		t.Errorf("sendAndPrintChat format error fallback: %v", err)
	}
}

func TestRunSlashCommand_SendAndPrintUsesSessionModel(t *testing.T) {
	var gotModel, gotProject string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == chatCompletionsPath {
			gotProject = r.Header.Get("OpenAI-Project")
			var req userapi.ChatCompletionsRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			gotModel = req.Model
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"role": "assistant", "content": "ok"}},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	chatSessionModel = "gpt-4"
	chatSessionProjectID = "proj-1"
	defer func() { chatSessionModel = ""; chatSessionProjectID = "" }()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	if err := sendAndPrintChat(client, "hi"); err != nil {
		t.Fatalf("sendAndPrintChat: %v", err)
	}
	if gotModel != "gpt-4" || gotProject != "proj-1" {
		t.Errorf("session not sent: model=%q project=%q", gotModel, gotProject)
	}
}
