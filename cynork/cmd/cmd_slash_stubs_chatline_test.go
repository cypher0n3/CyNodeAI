//nolint:dupl // table-driven and similar test patterns are intentional
package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

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
				Jobs: []userapi.JobResponse{{Result: &testJobResultDone}},
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
