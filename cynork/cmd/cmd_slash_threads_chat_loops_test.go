//nolint:dupl // table-driven and similar test patterns are intentional
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/creack/pty"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/config"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

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
	threadID, err := session.NewThread(context.Background())
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
	if _, err := session.NewThread(context.Background()); err == nil {
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
	defer func() {
		cfg = nil
		cfgGatewayPersistExplicit = false
	}()
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
	defer func() {
		cfg = nil
		cfgGatewayPersistExplicit = false
	}()
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
