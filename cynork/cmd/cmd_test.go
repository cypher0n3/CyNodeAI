//nolint:dupl // table-driven and similar test patterns are intentional
package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

const testUser = "alice"
const testStatusCompleted = "completed"
const (
	chatCompletionsPath = "/v1/chat/completions"
	chatThreadsPath     = "/v1/chat/threads"
	testThreadSelector  = "inbox"
	pathHealthz         = "/healthz"
	testResumeThreadID  = "tid-r"
)
const pathV1SkillsS1 = "/v1/skills/s1"
const pathV1Tasks = "/v1/tasks"
const testPromptEchoHi = "echo hi"
const inputModePrompt = "prompt"
const inputModeCommands = "commands"
const testPassword = "secret"

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

func TestExecute_TUI_StartsWithoutToken(t *testing.T) {
	// Per spec: startup token failure opens in-session login; TUI starts instead of exiting with auth error.
	oldRun := tuiRunProgram
	tuiRunProgram = func(_ *tea.Program) (tea.Model, error) { return nil, nil }
	defer func() { tuiRunProgram = oldRun }()
	path := writeTempConfig(t, "gateway_url: http://localhost\n")
	got := runWithArgs(t, "--config", path, "tui")
	if got != 0 {
		t.Errorf("Execute(tui) without token (in-session login) = %d, want 0", got)
	}
}

func TestExecute_TUI_WithToken(t *testing.T) {
	// Thread ensure runs inside tea (Init), not before Run; gateway need not be reachable here.
	oldRun := tuiRunProgram
	tuiRunProgram = func(_ *tea.Program) (tea.Model, error) { return nil, nil }
	defer func() { tuiRunProgram = oldRun }()
	path := writeTempConfig(t, "gateway_url: http://127.0.0.1:9\ntoken: x\n")
	got := runWithArgs(t, "--config", path, "tui")
	if got != 0 {
		t.Errorf("Execute(tui) with token and mock run = %d, want 0", got)
	}
}

func TestExecute_TUI_RunReturnsError(t *testing.T) {
	oldRun := tuiRunProgram
	tuiRunProgram = func(_ *tea.Program) (tea.Model, error) {
		return nil, errors.New("program error")
	}
	defer func() { tuiRunProgram = oldRun }()
	path := writeTempConfig(t, "gateway_url: http://127.0.0.1:9\ntoken: x\n")
	got := runWithArgs(t, "--config", path, "tui")
	if got != 1 {
		t.Errorf("Execute(tui) when run returns error = %d, want 1", got)
	}
}

func TestExecute_TUI_DefaultNewThread(t *testing.T) {
	oldRun := tuiRunProgram
	tuiRunProgram = func(_ *tea.Program) (tea.Model, error) { return nil, nil }
	defer func() { tuiRunProgram = oldRun }()
	path := writeTempConfig(t, "gateway_url: http://127.0.0.1:9\ntoken: x\n")
	got := runWithArgs(t, "--config", path, "tui")
	if got != 0 {
		t.Errorf("Execute(tui) default new thread = %d, want 0", got)
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
	server := mockJSONServer(t, http.StatusOK, userapi.UserResponse{ID: "u1", Handle: testUser})
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runAuthWhoami(nil, nil); err != nil {
		t.Errorf("runAuthWhoami: %v", err)
	}
}

// captureStdout redirects os.Stdout, runs f, and returns everything written to stdout.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	f()
	_ = w.Close()
	<-done
	os.Stdout = old
	return buf.String()
}

func TestRunAuthWhoami_TableOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.UserResponse{ID: "u1", Handle: testUser})
	defer server.Close()
	oldFmt := outputFmt
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatTable
	defer func() { cfg = nil; outputFmt = oldFmt }()
	out := captureStdout(t, func() {
		if err := runAuthWhoami(nil, nil); err != nil {
			t.Errorf("runAuthWhoami: %v", err)
		}
	})
	if !strings.Contains(out, "id=u1") || !strings.Contains(out, "user=alice") {
		t.Errorf("whoami table output = %q, want id=u1 user=alice", out)
	}
}

func TestRunAuthWhoami_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.UserResponse{ID: "u1", Handle: testUser})
	defer server.Close()
	oldFmt := outputFmt
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { cfg = nil; outputFmt = oldFmt }()
	out := captureStdout(t, func() {
		if err := runAuthWhoami(nil, nil); err != nil {
			t.Errorf("runAuthWhoami: %v", err)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("whoami JSON output not parseable: %v — %q", err, out)
	}
	if got["id"] != "u1" || got["user"] != testUser {
		t.Errorf("whoami JSON = %v, want id=u1 user=alice", got)
	}
}

func TestRunAuthLogout_TableOutput(t *testing.T) {
	path := writeTempConfig(t, "token: tok\n")
	oldFmt := outputFmt
	configPath = path
	cfg = &config.Config{Token: "tok"}
	outputFmt = outputFormatTable
	defer func() { configPath = ""; cfg = nil; outputFmt = oldFmt }()
	out := captureStdout(t, func() {
		if err := runAuthLogout(nil, nil); err != nil {
			t.Errorf("runAuthLogout: %v", err)
		}
	})
	if !strings.Contains(out, "logged_out=true") {
		t.Errorf("logout table output = %q, want logged_out=true", out)
	}
}

func TestRunAuthLogout_JSONOutput(t *testing.T) {
	path := writeTempConfig(t, "token: tok\n")
	oldFmt := outputFmt
	configPath = path
	cfg = &config.Config{Token: "tok"}
	outputFmt = outputFormatJSON
	defer func() { configPath = ""; cfg = nil; outputFmt = oldFmt }()
	out := captureStdout(t, func() {
		if err := runAuthLogout(nil, nil); err != nil {
			t.Errorf("runAuthLogout: %v", err)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("logout JSON output not parseable: %v — %q", err, out)
	}
	if got["logged_out"] != true {
		t.Errorf("logout JSON = %v, want logged_out=true", got)
	}
}

func TestRunAuthLogin_TableOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{
		AccessToken: "tok", TokenType: "Bearer",
	})
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\n")
	oldFmt := outputFmt
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = testUser
	authLoginPasswordStdin = true
	outputFmt = outputFormatTable
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	_, _ = w.WriteString("secret\n")
	_ = w.Close()
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
		outputFmt = oldFmt
	}()
	out := captureStdout(t, func() {
		if err := runAuthLogin(nil, nil); err != nil {
			t.Errorf("runAuthLogin: %v", err)
		}
	})
	if !strings.Contains(out, "logged_in=true") || !strings.Contains(out, "user=alice") {
		t.Errorf("login table output = %q, want logged_in=true user=alice", out)
	}
}

func TestRunAuthLogin_JSONOutput(t *testing.T) {
	server := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{
		AccessToken: "tok", TokenType: "Bearer",
	})
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\n")
	oldFmt := outputFmt
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL}
	authLoginHandle = testUser
	authLoginPasswordStdin = true
	outputFmt = outputFormatJSON
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	_, _ = w.WriteString("secret\n")
	_ = w.Close()
	defer func() {
		os.Stdin = oldStdin
		configPath = ""
		cfg = nil
		authLoginHandle = ""
		authLoginPasswordStdin = false
		outputFmt = oldFmt
	}()
	out := captureStdout(t, func() {
		if err := runAuthLogin(nil, nil); err != nil {
			t.Errorf("runAuthLogin: %v", err)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("login JSON output not parseable: %v — %q", err, out)
	}
	if got["logged_in"] != true || got["user"] != testUser {
		t.Errorf("login JSON = %v, want logged_in=true user=alice", got)
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

func TestTuiAuthProvider_Save_AfterLoginPreservesFileGatewayWithEnvOverride(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost:12080\n")
	t.Setenv("CYNORK_GATEWAY_URL", "http://127.0.0.1:49152")
	var err error
	cfg, err = config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfgGatewayFromEnv = true
	configPath = path
	defer func() {
		configPath = ""
		cfg = nil
		cfgGatewayFromEnv = false
		cfgGatewayPersistExplicit = false
	}()
	p := &tuiAuthProvider{cfg: cfg, saveFn: saveConfig}
	p.SetGatewayURL("http://127.0.0.1:49152", false)
	if err := p.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "12080") {
		t.Fatalf("config file should keep file gateway_url; got:\n%s", s)
	}
	if strings.Contains(s, "49152") {
		t.Fatalf("config file must not persist CYNORK_GATEWAY_URL after login-style SetGatewayURL; got:\n%s", s)
	}
}

func TestTuiAuthProvider_Save_AfterExplicitConnectPersistsNewGateway(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost:12080\n")
	t.Setenv("CYNORK_GATEWAY_URL", "http://127.0.0.1:49152")
	var err error
	cfg, err = config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfgGatewayFromEnv = true
	configPath = path
	defer func() {
		configPath = ""
		cfg = nil
		cfgGatewayFromEnv = false
		cfgGatewayPersistExplicit = false
	}()
	p := &tuiAuthProvider{cfg: cfg, saveFn: saveConfig}
	p.SetGatewayURL("http://127.0.0.1:33333", true)
	if err := p.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "33333") {
		t.Fatalf("explicit /connect gateway should be persisted; got:\n%s", s)
	}
}

func TestSaveConfig_ExistingFilePreservesGatewayWhenMemoryDrifts(t *testing.T) {
	path := writeTempConfig(t, "gateway_url: http://localhost:12080\n")
	var err error
	cfg, err = config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.GatewayURL = "http://127.0.0.1:99999"
	cfgGatewayFromEnv = false
	cfgGatewayPersistExplicit = false
	configPath = path
	defer func() {
		configPath = ""
		cfg = nil
		cfgGatewayPersistExplicit = false
	}()
	if err := saveConfig(); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "12080") {
		t.Fatalf("existing file gateway_url must be preserved; got:\n%s", s)
	}
	if strings.Contains(s, "99999") {
		t.Fatalf("config must not persist drifted in-memory gateway; got:\n%s", s)
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
	taskCreatePrompt = testPromptEchoHi
	defer func() { cfg = nil; taskCreatePrompt = "" }()
	if err := runTaskCreate(nil, nil); err != nil {
		t.Errorf("runTaskCreate: %v", err)
	}
}

func TestRunTaskCreate_RequiresExactlyOneInputMode(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() {
		cfg = nil
		taskCreateTask = ""
		taskCreatePrompt = ""
		taskCreateTaskFile = ""
		taskCreateScript = ""
		taskCreateCommands = nil
		taskCreateCommandsFile = ""
	}()
	err := runTaskCreate(nil, nil)
	if err == nil {
		t.Fatal("expected usage error when no input mode is set")
	}
	if exit.CodeOf(err) != 2 {
		t.Fatalf("exit code = %d, want 2", exit.CodeOf(err))
	}
	taskCreateTask = "a"
	taskCreatePrompt = "b"
	err = runTaskCreate(nil, nil)
	if err == nil {
		t.Fatal("expected usage error when multiple input modes are set")
	}
	if exit.CodeOf(err) != 2 {
		t.Fatalf("exit code = %d, want 2", exit.CodeOf(err))
	}
}

func TestRunTaskCreate_TaskFileModeAndProjectID(t *testing.T) {
	var got userapi.CreateTaskRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathV1Tasks && r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&got)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(userapi.TaskResponse{ID: "task-1", Status: "queued"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	taskFile := filepath.Join(t.TempDir(), "task.md")
	if err := os.WriteFile(taskFile, []byte("file task prompt"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectID := "11111111-2222-3333-4444-555555555555"
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCreateTaskFile = taskFile
	taskCreateProjectID = projectID
	taskCreateTaskName = "from-file"
	defer func() {
		cfg = nil
		taskCreateTaskFile = ""
		taskCreateProjectID = ""
		taskCreateTaskName = ""
	}()
	if err := runTaskCreate(nil, nil); err != nil {
		t.Fatalf("runTaskCreate: %v", err)
	}
	if got.InputMode != inputModePrompt {
		t.Fatalf("input_mode = %q, want %s", got.InputMode, inputModePrompt)
	}
	if strings.TrimSpace(got.Prompt) != "file task prompt" {
		t.Fatalf("prompt = %q", got.Prompt)
	}
	if got.ProjectID == nil || *got.ProjectID != projectID {
		t.Fatalf("project_id = %v, want %s", got.ProjectID, projectID)
	}
}

func TestRunTaskCreate_ResultWait(t *testing.T) {
	first := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == pathV1Tasks && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(userapi.TaskResponse{ID: "task-1", Status: "queued"})
			return
		}
		if r.URL.Path == "/v1/tasks/task-1/result" && r.Method == http.MethodGet {
			if first {
				first = false
				_ = json.NewEncoder(w).Encode(userapi.TaskResultResponse{TaskID: "task-1", Status: "running", Jobs: []userapi.JobResponse{}})
				return
			}
			_ = json.NewEncoder(w).Encode(userapi.TaskResultResponse{TaskID: "task-1", Status: "completed", Jobs: []userapi.JobResponse{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	taskCreatePrompt = testPromptEchoHi
	taskCreateResult = true
	defer func() {
		cfg = nil
		taskCreatePrompt = ""
		taskCreateResult = false
	}()
	if err := runTaskCreate(nil, nil); err != nil {
		t.Fatalf("runTaskCreate --result: %v", err)
	}
}

func TestRunTaskCreate_AttachValidation(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	taskCreatePrompt = testPromptEchoHi
	taskCreateAttachments = []string{t.TempDir()}
	defer func() {
		cfg = nil
		taskCreatePrompt = ""
		taskCreateAttachments = nil
	}()
	err := runTaskCreate(nil, nil)
	if err == nil {
		t.Fatal("expected validation error for non-regular attachment")
	}
	if exit.CodeOf(err) != 2 {
		t.Fatalf("exit code = %d, want 2", exit.CodeOf(err))
	}
}

func resetTaskCreateInputGlobals() {
	taskCreateTask = ""
	taskCreatePrompt = ""
	taskCreateTaskFile = ""
	taskCreateScript = ""
	taskCreateCommands = nil
	taskCreateCommandsFile = ""
}

func TestResolveTaskCreateInput_InlineTask(t *testing.T) {
	defer resetTaskCreateInputGlobals()
	resetTaskCreateInputGlobals()
	taskCreateTask = "inline"
	prompt, mode, err := resolveTaskCreateInput()
	if err != nil || mode != inputModePrompt || prompt != "inline" {
		t.Fatalf("inline task: prompt=%q mode=%q err=%v", prompt, mode, err)
	}
}

func TestResolveTaskCreateInput_InlinePrompt(t *testing.T) {
	defer resetTaskCreateInputGlobals()
	resetTaskCreateInputGlobals()
	taskCreatePrompt = "inline-prompt"
	prompt, mode, err := resolveTaskCreateInput()
	if err != nil || mode != inputModePrompt || prompt != "inline-prompt" {
		t.Fatalf("inline prompt: prompt=%q mode=%q err=%v", prompt, mode, err)
	}
}

func TestResolveTaskCreateInput_Commands(t *testing.T) {
	defer resetTaskCreateInputGlobals()
	resetTaskCreateInputGlobals()
	taskCreateCommands = []string{"echo a", "echo b"}
	prompt, mode, err := resolveTaskCreateInput()
	if err != nil || mode != inputModeCommands || !strings.Contains(prompt, "echo b") {
		t.Fatalf("commands: prompt=%q mode=%q err=%v", prompt, mode, err)
	}
}

func TestResolveTaskCreateInput_TaskAndPromptBothSet(t *testing.T) {
	defer resetTaskCreateInputGlobals()
	resetTaskCreateInputGlobals()
	taskCreateTask = "a"
	taskCreatePrompt = "b"
	_, _, err := resolveTaskCreateInput()
	if err == nil {
		t.Fatal("expected error when both --task and --prompt are set")
	}
}

func TestResolveTaskCreateInput_TaskFile(t *testing.T) {
	defer resetTaskCreateInputGlobals()
	resetTaskCreateInputGlobals()
	taskFile := filepath.Join(t.TempDir(), "task.txt")
	if err := os.WriteFile(taskFile, []byte("from-task-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	taskCreateTaskFile = taskFile
	prompt, mode, err := resolveTaskCreateInput()
	if err != nil || mode != inputModePrompt || prompt != "from-task-file" {
		t.Fatalf("task file: prompt=%q mode=%q err=%v", prompt, mode, err)
	}
}

func TestResolveTaskCreateInput_CommandsFile(t *testing.T) {
	defer resetTaskCreateInputGlobals()
	resetTaskCreateInputGlobals()
	cmdsFile := filepath.Join(t.TempDir(), "commands.txt")
	if err := os.WriteFile(cmdsFile, []byte("echo from file"), 0o600); err != nil {
		t.Fatal(err)
	}
	taskCreateCommandsFile = cmdsFile
	prompt, mode, err := resolveTaskCreateInput()
	if err != nil || mode != inputModeCommands || !strings.Contains(prompt, "from file") {
		t.Fatalf("commands file: prompt=%q mode=%q err=%v", prompt, mode, err)
	}
}

func TestReadModeFile_SizeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("abcdef"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readModeFile(path, "task-file", 3); err == nil {
		t.Fatal("expected size limit error")
	}
	content, err := readModeFile(path, "task-file", 16)
	if err != nil {
		t.Fatalf("readModeFile: %v", err)
	}
	if content != "abcdef" {
		t.Fatalf("content=%q", content)
	}
}

func TestValidateAttachments_CountLimit(t *testing.T) {
	paths := make([]string, maxAttachmentCount+1)
	for i := range paths {
		p := filepath.Join(t.TempDir(), fmt.Sprintf("a-%d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths[i] = p
	}
	if err := validateAttachments(paths); err == nil {
		t.Fatal("expected too many attachments error")
	}
}

func TestValidateRegularReadableFile_SymlinkRejected(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := validateRegularReadableFile(link); err == nil {
		t.Fatal("expected symlink rejection")
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
			Result: strPtr(`{"stdout":"s-out","stderr":""}`),
		}},
	})
	_ = w.Close()
	<-done

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %q (err=%v)", out.String(), err)
	}
	if got, _ := payload["stdout"].(string); got != `{"stdout":"s-out","stderr":""}` {
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
	if err := runStubList("/v1/audit"); err != nil {
		t.Errorf("runStubList /v1/audit: %v", err)
	}
}

func TestRunSkillsDelete_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1SkillsS1 || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runSkillsDelete(nil, []string{"s1"}); err != nil {
		t.Errorf("runSkillsDelete: %v", err)
	}
}

func TestRunSkillsDelete_NoAuth(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost"}
	defer func() { cfg = nil }()
	if err := runSkillsDelete(nil, []string{"s1"}); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestRunSkillsUpdate_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1SkillsS1 || r.Method != http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	tmp := filepath.Join(t.TempDir(), "skill.md")
	if err := os.WriteFile(tmp, []byte("# updated"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	skillsUpdateName, skillsUpdateScope = "", ""
	defer func() { cfg = nil; skillsUpdateName, skillsUpdateScope = "", "" }()
	if err := runSkillsUpdate(nil, []string{"s1", tmp}); err != nil {
		t.Errorf("runSkillsUpdate: %v", err)
	}
}

func TestRunSkillsUpdate_NoAuth(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "skill.md")
	if err := os.WriteFile(tmp, []byte("# x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg = &config.Config{GatewayURL: "http://localhost"}
	defer func() { cfg = nil }()
	if err := runSkillsUpdate(nil, []string{"s1", tmp}); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestRunCredsList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubList("/v1/creds"); err != nil {
		t.Errorf("runStubList /v1/creds: %v", err)
	}
}

func TestRunNodesList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubList("/v1/nodes"); err != nil {
		t.Errorf("runStubList /v1/nodes: %v", err)
	}
}

func TestRunPrefsSet(t *testing.T) {
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

func TestRunPrefsGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubGet("/v1/prefs/effective"); err != nil {
		t.Errorf("runStubGet /v1/prefs/effective: %v", err)
	}
}

func TestRunSettingsSet(t *testing.T) {
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

func TestRunSettingsGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	if err := runStubGet("/v1/settings"); err != nil {
		t.Errorf("runStubGet /v1/settings: %v", err)
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

// TestRunChat_ChatFails asserts that a gateway error on a chat message does not exit the session.
// The error is printed inline and the loop continues until /exit (spec CliChatSubcommandErrors).
func TestRunChat_ChatFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
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
	if err != nil {
		t.Fatalf("gateway error on chat message must not exit session (got err: %v)", err)
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
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	session.SetToken("tok")
	exitSession, err := runSlashCommand(session, "/help")
	if err != nil {
		t.Errorf("runSlashCommand /help: %v", err)
	}
	if exitSession {
		t.Error("runSlashCommand /help should not exit session")
	}
	exitSession, err = runSlashCommand(session, "/version")
	if err != nil {
		t.Errorf("runSlashCommand /version: %v", err)
	}
	if exitSession {
		t.Error("runSlashCommand /version should not exit session")
	}
}

func TestRunSlashCommand_Exit(t *testing.T) {
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	exitSession, _ := runSlashCommand(session, "/exit")
	if !exitSession {
		t.Error("runSlashCommand /exit should set exitSession true")
	}
	exitSession, _ = runSlashCommand(session, "/quit")
	if !exitSession {
		t.Error("runSlashCommand /quit should set exitSession true")
	}
}

func TestRunSlashCommand_Unknown(t *testing.T) {
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	exitSession, err := runSlashCommand(session, "/unknown")
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken(cfg.Token)
	exitSession, err := processChatLine(session, "")
	if err != nil || exitSession {
		t.Errorf("processChatLine empty: exit=%v err=%v", exitSession, err)
	}
	exitSession, err = processChatLine(session, "/help")
	if err != nil || exitSession {
		t.Errorf("processChatLine /help: exit=%v err=%v", exitSession, err)
	}
	exitSession, err = processChatLine(session, "hello")
	if err != nil || exitSession {
		t.Errorf("processChatLine hello: exit=%v err=%v", exitSession, err)
	}
}

func TestProcessChatLine_ShellEscape(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	session.SetToken("tok")
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
	exitSession, err := processChatLine(session, "! echo hi")
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
	_, _ = processChatLine(session, "!")
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
	exitSession, err = processChatLine(session, "! false")
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
	t.Setenv("CYNORK_TOKEN", "tok")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()
	path := writeTempConfig(t, "gateway_url: "+server.URL+"\n")
	configPath = path
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	oldRunner := runCynorkSubcommandForSlash
	runCynorkSubcommandForSlash = runCynorkSubcommandInProcess
	defer func() { runCynorkSubcommandForSlash = oldRunner; configPath = ""; cfg = nil }()
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	var errOut bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errOut, r)
		close(done)
	}()
	exitSession, err := processChatLine(session, "/skills list")
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

// TestProcessChatLine_GatewayErrorDoesNotExit asserts spec CliChatSubcommandErrors: a gateway
// error while sending a plain chat message MUST NOT return an error or exit the session.
// The error MUST be printed to stderr so the user sees it, then the loop continues.
func TestProcessChatLine_GatewayErrorDoesNotExit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("Bad Gateway"))
	}))
	defer server.Close()
	cfg = &config.Config{GatewayURL: server.URL, Token: "tok"}
	defer func() { cfg = nil }()
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	var errOut bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errOut, r)
		close(done)
	}()
	exitSession, err := processChatLine(session, "Hello!")
	_ = w.Close()
	<-done
	os.Stderr = oldStderr
	if err != nil {
		t.Errorf("gateway error on chat message must not return err (session must continue): %v", err)
	}
	if exitSession {
		t.Error("gateway error on chat message must not set exitSession true")
	}
	if !strings.Contains(errOut.String(), "502") && !strings.Contains(errOut.String(), "Bad Gateway") {
		t.Errorf("error should be printed to stderr, got %q", errOut.String())
	}
}

func TestProcessChatLine_AtFileValidation(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	session.SetToken("tok")
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	var errOut bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errOut, r)
		close(done)
	}()
	// @ with no path prints error and continues.
	exitSession, err := processChatLine(session, "@")
	if err != nil || exitSession {
		t.Errorf("@ with no path: exit=%v err=%v", exitSession, err)
	}
	// @/nonexistent_path prints "file not found" and continues.
	_, _ = processChatLine(session, "@/nonexistent_file_bdd_test_xyz.txt")
	_ = w.Close()
	<-done
	os.Stderr = oldStderr
	errStr := errOut.String()
	if !strings.Contains(errStr, "error") {
		t.Errorf("expected error message for missing file; got: %q", errStr)
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
		if r.URL.Path == pathHealthz {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		if r.URL.Path == "/v1/users/me" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "u1", "handle": testUser})
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	_, err := runSlashCommand(session, "/status")
	if err != nil {
		t.Errorf("runSlashCommand /status: %v", err)
	}
	_, err = runSlashCommand(session, "/whoami")
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	_, err := runSlashCommand(session, "/models")
	if err != nil {
		t.Errorf("runSlashCommand /models: %v", err)
	}
}

func stubSlashServeAuth(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/v1/users/me" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "u1", "handle": testUser})
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
	if r.URL.Path == pathV1SkillsS1 && r.Method == http.MethodGet {
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	cmds := []string{
		"/auth whoami", "/nodes list", "/nodes get n1", "/skills list", "/skills get s1",
		"/prefs list", "/prefs get", "/prefs effective", "/prefs delete",
		"/project list", "/project get p1", "/project", "/project set p2",
		"/model", "/model cynodeai.pm", "/task list", "/task get t1",
		"/task create -p say hello", "/task cancel -y t1", "/task result t1", "/task logs t1", "/task artifacts list t1",
		"/clear",
	}
	for _, cmd := range cmds {
		_, err := runSlashCommand(session, cmd)
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
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	session.SetToken("tok")
	// Exercise usage/error paths that don't call gateway.
	for _, cmd := range []string{
		"/task", "/task foo", "/task create", "/task cancel", "/task result", "/task logs",
		"/task artifacts", "/task artifacts list", "/task result --wait",
		"/auth", "/auth foo",
		"/nodes", "/nodes foo x", "/skills", "/skills get", "/prefs", "/project get",
		"/project help", "/project --help",
	} {
		_, err := runSlashCommand(session, cmd)
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
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	_, err := runSlashCommand(session, "/task create --help")
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("old-tok")
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	_, _ = w.WriteString("p\n")
	_ = w.Close()
	defer func() { os.Stdin = oldStdin }()
	if err := runSlashAuthDelegated(session, "login -u u --password-stdin"); err != nil {
		t.Errorf("runSlashAuthDelegated login: %v", err)
	}
	if cfg.Token != "new-tok" {
		t.Errorf("cfg.Token = %q", cfg.Token)
	}

	cfg.Token = "x"
	cfg.RefreshToken = ""
	if err := runSlashAuthDelegated(session, "logout"); err != nil {
		t.Errorf("runSlashAuthDelegated logout: %v", err)
	}

	refreshServer := mockJSONServer(t, http.StatusOK, userapi.LoginResponse{
		AccessToken: "refreshed-tok", RefreshToken: "new-refresh", TokenType: "Bearer", ExpiresIn: 900,
	})
	defer refreshServer.Close()
	t.Setenv("CYNORK_REFRESH_TOKEN", "old-refresh")
	cfg.GatewayURL = refreshServer.URL
	cfg.Token = "old"
	cfg.RefreshToken = "old-refresh"
	path2 := writeTempConfig(t, "gateway_url: "+refreshServer.URL+"\n")
	configPath = path2
	session2 := chat.NewSession(gateway.NewClient(refreshServer.URL))
	session2.SetToken("old")
	if err := runSlashAuthDelegated(session2, "refresh"); err != nil {
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
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	session.SetToken("tok")
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	err := runChatLoopLiner(session)
	if err != nil {
		t.Errorf("runChatLoopLiner with injected reader: %v", err)
	}
}

func TestRunChatLoopLiner_WithLinerGetLine(t *testing.T) {
	chatLinerGetLine = func(prompt string) (string, error) {
		return "/exit", nil
	}
	defer func() { chatLinerGetLine = nil }()
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	session.SetToken("tok")
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	err := runChatLoopLiner(session)
	if err != nil {
		t.Errorf("runChatLoopLiner with chatLinerGetLine: %v", err)
	}
}

func TestRunChatLoopWithReader_EOFImmediate(t *testing.T) {
	chatLineReader = func(prompt string) (string, error) {
		return "", io.EOF
	}
	defer func() { chatLineReader = nil }()
	session := chat.NewSession(gateway.NewClient("http://localhost"))
	cfg = &config.Config{GatewayURL: "http://localhost", Token: "tok"}
	defer func() { cfg = nil }()
	err := runChatLoopLiner(session)
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	if err := sendAndPrintChat(session, "hi"); err != nil {
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	if err := sendAndPrintChat(session, "hi"); err != nil {
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
	session := chat.NewSession(gateway.NewClient(server.URL))
	session.SetToken("tok")
	session.SetModel("gpt-4")
	session.SetProjectID("proj-1")
	if err := sendAndPrintChat(session, "hi"); err != nil {
		t.Fatalf("sendAndPrintChat: %v", err)
	}
	if gotModel != "gpt-4" || gotProject != "proj-1" {
		t.Errorf("session not sent: model=%q project=%q", gotModel, gotProject)
	}
}

func TestStartNewThread_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == chatThreadsPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "test-tid"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	threadID, err := session.NewThread()
	if err != nil {
		t.Fatalf("session.NewThread: %v", err)
	}
	if threadID != "test-tid" {
		t.Errorf("threadID = %q, want test-tid", threadID)
	}
}

func TestStartNewThread_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "unauthorized", "code": "unauthorized"},
		})
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	if _, err := session.NewThread(); err == nil {
		t.Fatal("expected error on non-201 response")
	}
}

func TestRunSlashThread_New(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == chatThreadsPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "new-tid"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, "new"); err != nil {
		t.Fatalf("runSlashThread new: %v", err)
	}
}

func TestRunSlashThread_EmptyRest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "tid"})
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, ""); err != nil {
		t.Fatalf("runSlashThread empty: %v", err)
	}
}

func TestRunSlashThread_Unknown(t *testing.T) {
	// Unknown subcommand should print to stderr and return nil (not an error).
	session := chat.NewSession(gateway.NewClient("http://localhost:1"))
	if err := runSlashThread(session, "delete"); err != nil {
		t.Fatalf("unknown subcommand should not return error, got: %v", err)
	}
}

func TestRunSlashThread_NilClient(t *testing.T) {
	session := &chat.Session{}
	if err := runSlashThread(session, "new"); err != nil {
		t.Fatalf("nil client should print message and return nil, got: %v", err)
	}
}

func TestSetChatSessionProject_None(t *testing.T) {
	session := &chat.Session{}
	setChatSessionProject(session, "none")
	if session.ProjectID != "" {
		t.Errorf("expected empty project ID after 'none', got %q", session.ProjectID)
	}
}

func TestRunChatLoopWithReader_OtherError(t *testing.T) {
	session := &chat.Session{}
	expectedErr := errors.New("other read error")
	getLine := func(string) (string, error) {
		return "", expectedErr
	}
	err := runChatLoopWithReader(session, "> ", getLine)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected other read error, got %v", err)
	}
}

func TestRunSlashThread_List(t *testing.T) {
	title := testThreadSelector
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == chatThreadsPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "tid-list-1", "title": title, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, "list"); err != nil {
		t.Fatalf("runSlashThread list: %v", err)
	}
}

func TestRunSlashThread_Switch(t *testing.T) {
	title := testThreadSelector
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == chatThreadsPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "tid-switch-1", "title": title, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, "switch "+testThreadSelector); err != nil {
		t.Fatalf("runSlashThread switch: %v", err)
	}
	if session.CurrentThreadID != "tid-switch-1" {
		t.Errorf("expected CurrentThreadID=tid-switch-1 after switch, got %q", session.CurrentThreadID)
	}
}

func TestRunSlashThread_SwitchNoSelector(t *testing.T) {
	session := chat.NewSession(gateway.NewClient("http://localhost:1"))
	session.SetToken("tok")
	if err := runSlashThread(session, "switch"); err != nil {
		t.Fatalf("runSlashThread switch no selector should not error: %v", err)
	}
}

func TestRunSlashThread_Rename(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, chatThreadsPath) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":testResumeThreadID,"title":"My Title"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	session.CurrentThreadID = testResumeThreadID
	if err := runSlashThread(session, "rename My Title"); err != nil {
		t.Fatalf("runSlashThread rename: %v", err)
	}
}

func TestRunSlashThread_RenameNoTitle(t *testing.T) {
	session := chat.NewSession(gateway.NewClient("http://localhost:1"))
	session.SetToken("tok")
	session.CurrentThreadID = testResumeThreadID
	if err := runSlashThread(session, "rename"); err != nil {
		t.Fatalf("runSlashThread rename no title should not error: %v", err)
	}
}

func TestChatResumeThread_Flag(t *testing.T) {
	title := testThreadSelector
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == chatThreadsPath {
			called = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "tid-resume-1", "title": title, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg = &config.Config{GatewayURL: srv.URL, Token: "tok"}
	defer func() { cfg = nil }()

	oldResume := chatResumeThread
	chatResumeThread = testThreadSelector
	defer func() { chatResumeThread = oldResume }()

	chatLinerGetLine = func(_ string) (string, error) { return "", io.EOF }
	defer func() { chatLinerGetLine = nil }()

	if err := runChat(nil, nil); err != nil {
		t.Fatalf("runChat with --resume-thread: %v", err)
	}
	if !called {
		t.Error("expected GET /v1/chat/threads to be called when --resume-thread is set")
	}
}

func TestChatResumeThread_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg = &config.Config{GatewayURL: srv.URL, Token: "tok"}
	defer func() { cfg = nil }()

	oldResume := chatResumeThread
	chatResumeThread = testThreadSelector
	defer func() { chatResumeThread = oldResume }()

	if err := runChat(nil, nil); err == nil {
		t.Fatal("expected error when --resume-thread resolution fails")
	}
}

func TestRunSlashConnect_ShowURL(t *testing.T) {
	session := chat.NewSession(gateway.NewClient("http://gw-test:1234"))
	session.SetToken("tok")
	if err := runSlashConnect(session, ""); err != nil {
		t.Fatalf("runSlashConnect show: %v", err)
	}
}

func TestRunSlashConnect_UpdateURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathHealthz {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	cfg = &config.Config{GatewayURL: "http://old:1", Token: "tok"}
	defer func() { cfg = nil }()
	session := chat.NewSession(gateway.NewClient("http://old:1"))
	session.SetToken("tok")
	if err := runSlashConnect(session, srv.URL); err != nil {
		t.Fatalf("runSlashConnect update: %v", err)
	}
	if session.Client.BaseURL != srv.URL {
		t.Errorf("session.Client.BaseURL not updated, got %q", session.Client.BaseURL)
	}
}

func TestRunSlashConnect_HealthFail(t *testing.T) {
	cfg = &config.Config{GatewayURL: "http://old:1", Token: "tok"}
	defer func() { cfg = nil }()
	session := chat.NewSession(gateway.NewClient("http://old:1"))
	session.SetToken("tok")
	if err := runSlashConnect(session, "http://localhost:9"); err != nil {
		t.Fatalf("runSlashConnect health fail should not error: %v", err)
	}
}

func TestRunSlashSetThinking_Show(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runSlashSetThinking(nil, true); err != nil {
		t.Fatalf("runSlashSetThinking show: %v", err)
	}
	if !cfg.TUI.ShowThinkingByDefault {
		t.Error("expected ShowThinkingByDefault=true after show-thinking")
	}
}

func TestRunSlashSetThinking_Hide(t *testing.T) {
	cfg = &config.Config{TUI: config.TUIConfig{ShowThinkingByDefault: true}}
	defer func() { cfg = nil }()
	if err := runSlashSetThinking(nil, false); err != nil {
		t.Fatalf("runSlashSetThinking hide: %v", err)
	}
	if cfg.TUI.ShowThinkingByDefault {
		t.Error("expected ShowThinkingByDefault=false after hide-thinking")
	}
}

func TestRunSlashSetToolOutput_Show(t *testing.T) {
	cfg = &config.Config{}
	defer func() { cfg = nil }()
	if err := runSlashSetToolOutput(nil, true); err != nil {
		t.Fatalf("runSlashSetToolOutput show: %v", err)
	}
	if !cfg.TUI.ShowToolOutputByDefault {
		t.Error("expected ShowToolOutputByDefault=true after show-tool-output")
	}
}

func TestRunSlashSetToolOutput_Hide(t *testing.T) {
	cfg = &config.Config{TUI: config.TUIConfig{ShowToolOutputByDefault: true}}
	defer func() { cfg = nil }()
	if err := runSlashSetToolOutput(nil, false); err != nil {
		t.Fatalf("runSlashSetToolOutput hide: %v", err)
	}
	if cfg.TUI.ShowToolOutputByDefault {
		t.Error("expected ShowToolOutputByDefault=false after hide-tool-output")
	}
}

func TestRunSlashCommand_ConnectAndThinking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathHealthz {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	cfg = &config.Config{GatewayURL: srv.URL, Token: "tok"}
	defer func() { cfg = nil }()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if _, err := runSlashCommand(session, "/connect"); err != nil {
		t.Errorf("runSlashCommand /connect: %v", err)
	}
	if _, err := runSlashCommand(session, "/show-thinking"); err != nil {
		t.Errorf("runSlashCommand /show-thinking: %v", err)
	}
	if _, err := runSlashCommand(session, "/hide-thinking"); err != nil {
		t.Errorf("runSlashCommand /hide-thinking: %v", err)
	}
	if _, err := runSlashCommand(session, "/show-tool-output"); err != nil {
		t.Errorf("runSlashCommand /show-tool-output: %v", err)
	}
	if _, err := runSlashCommand(session, "/hide-tool-output"); err != nil {
		t.Errorf("runSlashCommand /hide-tool-output: %v", err)
	}
}

func TestRunSlashThread_NewError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, "new"); err != nil {
		t.Fatalf("runSlashThread new error path should not return error: %v", err)
	}
}

func TestRunSlashThread_ListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, subCmdList); err != nil {
		t.Fatalf("runSlashThread list error path should not return error: %v", err)
	}
}

func TestRunSlashThread_ListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == chatThreadsPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, subCmdList); err != nil {
		t.Fatalf("runSlashThread list empty should not error: %v", err)
	}
}

func TestRunSlashThread_SwitchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	if err := runSlashThread(session, "switch "+testThreadSelector); err != nil {
		t.Fatalf("runSlashThread switch error path should not error: %v", err)
	}
}

func TestRunSlashThread_RenameError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	session := chat.NewSession(gateway.NewClient(srv.URL))
	session.SetToken("tok")
	session.CurrentThreadID = testResumeThreadID
	if err := runSlashThread(session, "rename My Title"); err != nil {
		t.Fatalf("runSlashThread rename error path should not error: %v", err)
	}
}

func TestRunCynorkSubcommand_ExeError(t *testing.T) {
	old := getCynorkExeForSubcommand
	getCynorkExeForSubcommand = func() (string, error) {
		return "", errors.New("no executable found")
	}
	defer func() { getCynorkExeForSubcommand = old }()
	err := runCynorkSubcommand("test", "")
	if err == nil {
		t.Error("expected error when getCynorkExeForSubcommand fails")
	}
}
