//nolint:dupl // table-driven and similar test patterns are intentional
package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

const testStatusCompleted = "completed"

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

func TestRunTaskWatch_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskWatch(nil, []string{"tid"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskWatch_ExitsOnTerminalStatus(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []gateway.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("done")}},
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

func TestRunTaskList_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runTaskList(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTaskList_OK(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.ListTasksResponse{
		Tasks: []gateway.TaskResponse{{ID: "t1", Status: "completed"}},
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
	server := mockJSONServer(t, http.StatusOK, gateway.TaskResponse{ID: "tid", Status: "running"})
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
	server := mockJSONServer(t, http.StatusOK, gateway.CancelTaskResponse{TaskID: "tid", Canceled: true})
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
	server := mockJSONServer(t, http.StatusOK, gateway.TaskLogsResponse{TaskID: "tid", Stdout: "out", Stderr: "err"})
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
	server := mockJSONServer(t, http.StatusCreated, gateway.TaskResponse{ID: "tid", Status: "queued"})
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
	server := mockJSONServer(t, http.StatusOK, gateway.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []gateway.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("out")}},
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
	server := mockJSONServer(t, http.StatusOK, gateway.TaskResponse{ID: "tid", Status: "running"})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskGet(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskGet: %v", err)
	}
}

func TestRunTaskList_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.ListTasksResponse{Tasks: []gateway.TaskResponse{}})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskList(nil, nil); err != nil {
		t.Errorf("runTaskList: %v", err)
	}
}

func TestRunTaskCancel_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, gateway.CancelTaskResponse{TaskID: "tid", Canceled: true})
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
	server := mockJSONServer(t, http.StatusOK, gateway.TaskResultResponse{
		TaskID: "tid", Status: "completed",
		Jobs: []gateway.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("done")}},
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
	server := mockJSONServer(t, http.StatusOK, gateway.ListTasksResponse{Tasks: []gateway.TaskResponse{}})
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
		if r.URL.Path == "/v1/chat" && r.Method == http.MethodPost {
			var req gateway.ChatRequest
			if _ = json.NewDecoder(r.Body).Decode(&req); req.Message == "hello" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(gateway.ChatResponse{Response: "reply"})
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
				_ = json.NewEncoder(w).Encode(gateway.TaskResultResponse{TaskID: "tid", Status: "running", Jobs: []gateway.JobResponse{}})
				return
			}
			_ = json.NewEncoder(w).Encode(gateway.TaskResultResponse{
				TaskID: "tid", Status: "completed",
				Jobs: []gateway.JobResponse{{Result: strPtr("done")}},
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
