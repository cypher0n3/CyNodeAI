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
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

var (
	testJobResultOut        = "out"
	testJobResultDone       = "done"
	testJobResultStdoutJSON = `{"stdout":"s-out","stderr":""}`
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
	p := &tuiAuthProvider{cfg: cfg, saveFn: persistSessionAndConfig}
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
	p := &tuiAuthProvider{cfg: cfg, saveFn: persistSessionAndConfig}
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

func TestTuiAuthProvider_AccessorsAndSetters(t *testing.T) {
	cfg := &config.Config{
		GatewayURL:   "http://gw",
		Token:        "access",
		RefreshToken: "refresh",
		TUI: config.TUIConfig{
			ShowThinkingByDefault:   false,
			ShowToolOutputByDefault: true,
		},
	}
	var saved bool
	p := &tuiAuthProvider{cfg: cfg, saveFn: func() error { saved = true; return nil }}
	if p.Token() != "access" || p.RefreshToken() != "refresh" || p.GatewayURL() != "http://gw" {
		t.Fatalf("getters: token=%q refresh=%q gw=%q", p.Token(), p.RefreshToken(), p.GatewayURL())
	}
	p.SetTokens("a", "b")
	if cfg.Token != "a" || cfg.RefreshToken != "b" {
		t.Fatalf("SetTokens: token=%q refresh=%q", cfg.Token, cfg.RefreshToken)
	}
	p.SetShowThinkingByDefault(true)
	if !p.ShowThinkingByDefault() {
		t.Fatal("ShowThinkingByDefault")
	}
	p.SetShowToolOutputByDefault(false)
	if p.ShowToolOutputByDefault() {
		t.Fatal("ShowToolOutputByDefault")
	}
	oldExplicit := cfgGatewayPersistExplicit
	cfgGatewayPersistExplicit = false
	defer func() { cfgGatewayPersistExplicit = oldExplicit }()
	p.SetGatewayURL("http://new", false)
	if cfg.GatewayURL != "http://new" {
		t.Fatalf("SetGatewayURL: %q", cfg.GatewayURL)
	}
	if err := p.Save(); err != nil || !saved {
		t.Fatalf("Save err=%v saved=%v", err, saved)
	}
}

func TestTuiHealthPollIntervalSec(t *testing.T) {
	if got := tuiHealthPollIntervalSec(nil); got != 5 {
		t.Errorf("nil config: %d", got)
	}
	z := 0
	if got := tuiHealthPollIntervalSec(&config.Config{TUI: config.TUIConfig{HealthPollIntervalSec: &z}}); got != 0 {
		t.Errorf("explicit 0: %d", got)
	}
	seven := 7
	if got := tuiHealthPollIntervalSec(&config.Config{TUI: config.TUIConfig{HealthPollIntervalSec: &seven}}); got != 7 {
		t.Errorf("explicit 7: %d", got)
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
