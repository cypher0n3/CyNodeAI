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
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"
	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

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
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: &testJobResultOut}},
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
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: &testJobResultDone}},
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
	authLoginPasswordStdin = true
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
	}()
	os.Stdin = r
	_, _ = w.WriteString("p\n")
	_ = w.Close()
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
	authLoginPasswordStdin = true
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
	}()
	os.Stdin = r
	_, _ = w.WriteString("p\n")
	_ = w.Close()
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
	authLoginPasswordStdin = true
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
	}()
	os.Stdin = r
	_, _ = w.WriteString("p\n")
	_ = w.Close()
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
	authLoginPasswordStdin = true
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	_, _ = w.WriteString("p\n")
	_ = w.Close()
	old := getDefaultConfigPath
	getDefaultConfigPath = func() (string, error) { return "", errors.New("injected") }
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
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
	_, _ = w.WriteString(testPassword + "\n")
	_ = w.Close()
	pass, err := readPassword("")
	if err != nil {
		t.Errorf("readPassword: %v", err)
	}
	if pass != testPassword {
		t.Errorf("password = %q, want %s", pass, testPassword)
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

func TestReadPasswordFromStdin_TrimsSingleNewline(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	_, _ = w.WriteString(testPassword + "\r\n")
	_ = w.Close()
	pass, err := readPasswordFromStdin()
	if err != nil {
		t.Fatalf("readPasswordFromStdin: %v", err)
	}
	if pass != testPassword {
		t.Fatalf("password = %q, want %s", pass, testPassword)
	}
}

func TestReadPasswordFromStdin_NoTrailingNewline(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	_, _ = w.WriteString(testPassword)
	_ = w.Close()
	pass, err := readPasswordFromStdin()
	if err != nil {
		t.Fatalf("readPasswordFromStdin: %v", err)
	}
	if pass != testPassword {
		t.Fatalf("password = %q, want %s", pass, testPassword)
	}
}

func TestReadPromptLine_TrimsWhitespace(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	_, _ = w.WriteString("  user1  \n")
	_ = w.Close()
	handle, err := readPromptLine("Handle: ")
	if err != nil {
		t.Fatalf("readPromptLine: %v", err)
	}
	if handle != "user1" {
		t.Fatalf("handle = %q, want user1", handle)
	}
}

func TestReadPassword_TerminalInput(t *testing.T) {
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		t.Skipf("pty.Open not available: %v", err)
	}
	defer func() {
		_ = ptyMaster.Close()
		_ = ptySlave.Close()
	}()
	oldStdin := os.Stdin
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = ptySlave
	os.Stderr = w
	defer func() {
		os.Stdin = oldStdin
		os.Stderr = oldStderr
		_ = r.Close()
		_ = w.Close()
	}()

	type result struct {
		pass string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		pass, pErr := readPassword("Password: ")
		done <- result{pass: pass, err: pErr}
	}()
	_, _ = ptyMaster.WriteString(testPassword + "\n")
	got := <-done
	if got.err != nil {
		t.Fatalf("readPassword (terminal): %v", got.err)
	}
	if got.pass != testPassword {
		t.Fatalf("password = %q, want %s", got.pass, testPassword)
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

func runAuthLoginWithStdin(t *testing.T, handle string, passwordStdin bool, stdinInput string) error {
	t.Helper()
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{AccessToken: "tok", TokenType: "Bearer"})
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = handle
	authLoginPasswordStdin = passwordStdin
	oldStdin := os.Stdin
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
	}()
	os.Stdin = r
	_, _ = w.WriteString(stdinInput)
	_ = w.Close()
	return runAuthLogin(nil, nil)
}

func TestRunAuthLogin_HandleFromStdin(t *testing.T) {
	if err := runAuthLoginWithStdin(t, "", false, "stdin_user\nstdin_pass\n"); err != nil {
		t.Errorf("runAuthLogin: %v", err)
	}
}

func TestRunAuthLogin_PasswordFromStdin(t *testing.T) {
	if err := runAuthLoginWithStdin(t, "u", true, "stdin_pass\n"); err != nil {
		t.Errorf("runAuthLogin: %v", err)
	}
}

func TestRunAuthLogin_PasswordStdinRequiresHandle(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost"}
	authLoginPasswordStdin = true
	defer func() {
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
	}()
	err := runAuthLogin(nil, nil)
	if err == nil {
		t.Fatal("expected usage error")
	}
	if exit.CodeOf(err) != 2 {
		t.Fatalf("exit code = %d, want 2", exit.CodeOf(err))
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
	authLoginPasswordStdin = true
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
	}()
	os.Stdin = r
	_, _ = w.WriteString("p\n")
	_ = w.Close()
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

func TestRunChat_ThreadNewError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == chatThreadsPath && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	chatThreadNew = true
	defer func() { chatThreadNew = false }()
	if err := runChat(nil, nil); err == nil {
		t.Fatal("expected error when NewThread fails")
	}
}

func TestRunCredsList_NoToken(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runStubList("/v1/creds"); err == nil {
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
	if err := runStubList("/v1/creds"); err != nil {
		t.Errorf("runStubList /v1/creds: %v", err)
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
	if err := runStubList("/v1/nodes"); err != nil {
		t.Errorf("runStubList /v1/nodes: %v", err)
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
	if err := runStubList("/v1/audit"); err != nil {
		t.Errorf("runStubList /v1/audit: %v", err)
	}
}

func TestRunPrefsSet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubSet("/v1/prefs"); err != nil {
		t.Errorf("runStubSet /v1/prefs: %v", err)
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
	if err := runStubGet("/v1/prefs/effective"); err != nil {
		t.Errorf("runStubGet /v1/prefs/effective: %v", err)
	}
}

func TestRunSettingsSet_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubSet("/v1/settings"); err != nil {
		t.Errorf("runStubSet /v1/settings: %v", err)
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
	if err := runStubGet("/v1/settings"); err != nil {
		t.Errorf("runStubGet /v1/settings: %v", err)
	}
}

func TestRunSkillsLoad_OK(t *testing.T) {
	f, err := os.CreateTemp("", "skill-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString("# Test skill\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/skills/load" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"s1","name":"Test skill"}`))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runSkillsLoad(nil, []string{f.Name()}); err != nil {
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
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: &testJobResultOut}},
	})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = "" }()
	if err := runTaskResult(nil, []string{"tid"}); err != nil {
		t.Errorf("runTaskResult: %v", err)
	}
}

func TestParseTaskJobResultStderr(t *testing.T) {
	stderr, ok := parseTaskJobResultStderr(`{"stdout":"out","stderr":"err"}`)
	if !ok {
		t.Fatal("expected parseTaskJobResultStderr to parse stderr from JSON")
	}
	if stderr != "err" {
		t.Fatalf("unexpected parsed stderr: %q", stderr)
	}
	if _, ok := parseTaskJobResultStderr(`{"message":"ok"}`); ok {
		t.Fatal("expected parseTaskJobResultStderr to reject JSON without stderr key")
	}
	if _, ok := parseTaskJobResultStderr("not-json"); ok {
		t.Fatal("expected parseTaskJobResultStderr to reject non-JSON input")
	}
}

func TestPrintTaskResult_JSONOutput_TerminalIncludesStderr(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	oldFmt := outputFmt
	os.Stdout = w
	outputFmt = outputFormatJSON
	defer func() {
		os.Stdout = oldStdout
		outputFmt = oldFmt
	}()

	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&out, r)
		close(done)
	}()

	printTaskResult(&userapi.TaskResultResponse{
		TaskID: "tid",
		Status: "completed",
		Jobs: []userapi.JobResponse{{
			ID:     "j1",
			Status: "completed",
			Result: &testJobResultStdoutJSON,
		}},
	})
	_ = w.Close()
	<-done

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q (err=%v)", out.String(), err)
	}
	if got, _ := payload["stdout"].(string); got != testJobResultStdoutJSON {
		t.Fatalf("stdout mismatch: got %q payload=%v", got, payload)
	}
	stderr, ok := payload["stderr"]
	if !ok {
		t.Fatalf("expected stderr key in terminal task result payload: %v", payload)
	}
	if got, _ := stderr.(string); got != "" {
		t.Fatalf("stderr mismatch: got %q payload=%v", got, payload)
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
		Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: &testJobResultDone}},
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
