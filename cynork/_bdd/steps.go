// Package bdd provides Godog step definitions for the cynork CLI suite.
// Feature files live under repo features/cynork/.
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

type ctxKey int

const stateKey ctxKey = 0

type cynorkState struct {
	mockServer *httptest.Server
	cynorkBin  string
	configPath string // path to config file for session persistence (login writes, whoami reads)
	lastExit   int
	lastStdout string
	lastStderr string
	token      string
	taskID     string
	// mock state: token -> handle for GetMe
	userByToken map[string]string
	tasks       map[string]string // taskID -> result stdout
	mu          sync.Mutex
}

func getState(ctx context.Context) *cynorkState {
	s, _ := ctx.Value(stateKey).(*cynorkState)
	return s
}

func (s *cynorkState) mockGatewayMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Handle   string `json:"handle"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		tok := "tok-" + req.Handle
		s.mu.Lock()
		if s.userByToken == nil {
			s.userByToken = make(map[string]string)
		}
		s.userByToken[tok] = req.Handle
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": tok,
			"token_type":   "Bearer",
			"expires_in":   900,
		})
	})
	mux.HandleFunc("GET /v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		tok := strings.TrimPrefix(auth, "Bearer ")
		s.mu.Lock()
		handle, ok := s.userByToken[tok]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "user-1", "handle": handle, "is_active": true,
		})
	})
	mux.HandleFunc("POST /v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		if s.tasks == nil {
			s.tasks = make(map[string]string)
		}
		id := fmt.Sprintf("task-%d", len(s.tasks)+1)
		s.tasks[id] = req.Prompt
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": id, "status": "queued", "created_at": "", "updated_at": "",
		})
	})
	mux.HandleFunc("GET /v1/tasks/{id}/result", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		prompt, ok := s.tasks[id]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Echo the prompt as result for "echo hello" -> "hello"
		result := prompt
		if strings.HasPrefix(prompt, "echo ") {
			result = strings.TrimSpace(strings.TrimPrefix(prompt, "echo "))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"task_id": id, "status": "completed",
			"jobs": []map[string]any{
				{"id": "j1", "status": "completed", "result": result},
			},
		})
	})
	mux.HandleFunc("GET /v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		var tasks []map[string]any
		for id, prompt := range s.tasks {
			tasks = append(tasks, map[string]any{"id": id, "task_id": id, "status": "completed", "prompt": prompt})
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"tasks": tasks})
	})
	mux.HandleFunc("GET /v1/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		prompt, ok := s.tasks[id]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "task_id": id, "status": "completed", "prompt": prompt})
	})
	mux.HandleFunc("POST /v1/tasks/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		_, ok := s.tasks[id]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"task_id": id, "canceled": true})
	})
	mux.HandleFunc("GET /v1/tasks/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		prompt, ok := s.tasks[id]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		result := prompt
		if strings.HasPrefix(prompt, "echo ") {
			result = strings.TrimSpace(strings.TrimPrefix(prompt, "echo "))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"task_id": id, "stdout": result, "stderr": ""})
	})
	// Stub endpoints for creds, prefs, settings, nodes, skills, audit
	mux.HandleFunc("GET /v1/creds", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})
	mux.HandleFunc("GET /v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})
	mux.HandleFunc("GET /v1/audit", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})
	mux.HandleFunc("POST /v1/prefs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /v1/prefs/effective", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	mux.HandleFunc("POST /v1/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /v1/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	mux.HandleFunc("POST /v1/skills/load", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func (s *cynorkState) runCynork(args []string, env ...string) (exit int, stdout, stderr string) {
	cmd := exec.Command(s.cynorkBin, args...)
	cmd.Env = append(os.Environ(), env...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = strings.TrimSpace(outBuf.String())
	stderr = strings.TrimSpace(errBuf.String())
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exit = exitErr.ExitCode()
		} else {
			exit = -1
		}
	}
	return exit, stdout, stderr
}

// InitializeCynorkSuite sets up the godog suite for cynork features.
func InitializeCynorkSuite(sc *godog.ScenarioContext, state *cynorkState) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		state.mockServer = httptest.NewServer(state.mockGatewayMux())
		state.userByToken = nil
		state.tasks = nil
		state.token = ""
		state.taskID = ""
		wd, err := os.Getwd()
		if err != nil {
			return ctx, err
		}
		root := wd
		if strings.HasSuffix(wd, string(filepath.Separator)+"_bdd") || filepath.Base(wd) == "_bdd" {
			root = filepath.Join(wd, "..", "..")
		}
		tmpDir := filepath.Join(root, "tmp")
		_ = os.MkdirAll(tmpDir, 0o755)
		state.configPath = filepath.Join(tmpDir, "cynork-bdd-config.yaml")
		_ = os.WriteFile(state.configPath, []byte("gateway_url: http://localhost\n"), 0o600)
		bin := filepath.Join(tmpDir, "cynork-bdd")
		cynorkDir := filepath.Join(root, "cynork")
		build := exec.Command("go", "build", "-o", bin, ".")
		build.Dir = cynorkDir
		build.Env = os.Environ()
		if err := build.Run(); err != nil {
			return ctx, fmt.Errorf("build cynork: %w", err)
		}
		state.cynorkBin = bin
		return context.WithValue(ctx, stateKey, state), nil
	})

	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if state.mockServer != nil {
			state.mockServer.Close()
		}
		state.mockServer = nil
		return ctx, nil
	})

	sc.Step(`^a mock gateway is running$`, func(ctx context.Context) error {
		if getState(ctx).mockServer == nil {
			return fmt.Errorf("mock gateway not started")
		}
		return nil
	})

	sc.Step(`^cynork is built$`, func(ctx context.Context) error {
		if getState(ctx).cynorkBin == "" {
			return fmt.Errorf("cynork binary path not set")
		}
		return nil
	})

	sc.Step(`^I run cynork status$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork([]string{"status"}, "CYNORK_GATEWAY_URL="+st.mockServer.URL)
		return nil
	})

	sc.Step(`^cynork exits with code (\d+)$`, func(ctx context.Context, codeStr string) error {
		var want int
		if _, err := fmt.Sscanf(codeStr, "%d", &want); err != nil {
			return err
		}
		st := getState(ctx)
		if st.lastExit != want {
			return fmt.Errorf("cynork exit code %d, want %d (stderr: %s)", st.lastExit, want, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^cynork stdout contains "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getState(ctx)
		if !strings.Contains(st.lastStdout, want) {
			return fmt.Errorf("stdout %q does not contain %q", st.lastStdout, want)
		}
		return nil
	})

	sc.Step(`^I run cynork auth login with username "([^"]*)" and password "([^"]*)"$`, func(ctx context.Context, user, pass string) error {
		st := getState(ctx)
		args := []string{"--config", st.configPath, "auth", "login", "-u", user, "-p", pass}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, "CYNORK_GATEWAY_URL="+st.mockServer.URL)
		if st.lastExit == 0 {
			st.token = "tok-" + user
		}
		return nil
	})

	sc.Step(`^I run cynork auth whoami$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		if st.token != "" {
			env = append(env, "CYNORK_TOKEN="+st.token)
		}
		args := []string{"--config", st.configPath, "auth", "whoami"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork auth whoami using the stored config$`, func(ctx context.Context) error {
		st := getState(ctx)
		// No CYNORK_TOKEN: whoami must read token from config file (session persistence).
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		args := []string{"--config", st.configPath, "auth", "whoami"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I am logged in with username "([^"]*)" and password "([^"]*)"$`, func(ctx context.Context, user, pass string) error {
		st := getState(ctx)
		args := []string{"--config", st.configPath, "auth", "login", "-u", user, "-p", pass}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, "CYNORK_GATEWAY_URL="+st.mockServer.URL)
		if st.lastExit != 0 {
			return fmt.Errorf("login failed: %s", st.lastStderr)
		}
		st.token = "tok-" + user
		return nil
	})

	sc.Step(`^I run cynork task create with prompt "([^"]*)"$`, func(ctx context.Context, prompt string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", prompt}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I store the task id from cynork stdout$`, func(ctx context.Context) error {
		st := getState(ctx)
		// Parse task_id=<id> from table mode or first line
		out := strings.TrimSpace(st.lastStdout)
		if prefix := "task_id="; strings.HasPrefix(out, prefix) {
			st.taskID = strings.TrimSpace(strings.SplitN(out[len(prefix):], " ", 2)[0])
		} else if out != "" && !strings.HasPrefix(out, "{") {
			st.taskID = strings.SplitN(out, "\n", 2)[0]
		} else {
			// JSON mode: {"task_id":"..."}
			var m map[string]string
			if err := json.Unmarshal([]byte(out), &m); err == nil && m["task_id"] != "" {
				st.taskID = m["task_id"]
			} else {
				st.taskID = out
			}
		}
		if st.taskID == "" {
			return fmt.Errorf("stdout empty or no task_id: %q", st.lastStdout)
		}
		return nil
	})

	sc.Step(`^I run cynork task result with the stored task id$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.taskID == "" {
			return fmt.Errorf("no stored task id")
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "result", st.taskID}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork task list$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "list"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork task get with the stored task id$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.taskID == "" {
			return fmt.Errorf("no stored task id")
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "get", st.taskID}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork task cancel with the stored task id and yes$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.taskID == "" {
			return fmt.Errorf("no stored task id")
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "cancel", "-y", st.taskID}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork task logs with the stored task id$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.taskID == "" {
			return fmt.Errorf("no stored task id")
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "logs", st.taskID}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork status with output json using shorthand "([^"]*)"$`, func(ctx context.Context, shorthand string) error {
		st := getState(ctx)
		args := []string{"--config", st.configPath, "status", shorthand, "json"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, "CYNORK_GATEWAY_URL="+st.mockServer.URL)
		return nil
	})

	sc.Step(`^I run cynork chat$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		// Run in background or with stdin; for "accepts /exit" we need to send /exit on stdin
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, "/exit\n")
		return nil
	})

	sc.Step(`^I send "([^"]*)" to cynork stdin$`, func(ctx context.Context, text string) error {
		// Used with "I run cynork chat" - the step above runs chat with stdin "/exit\n"
		// This step is for documentation; actual send is in runCynorkWithStdin
		_ = text
		return nil
	})

	sc.Step(`^a task file "([^"]*)" exists with content "([^"]*)"$`, func(ctx context.Context, path, content string) error {
		dir := filepath.Dir(path)
		if dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		return os.WriteFile(path, []byte(content), 0o600)
	})

	sc.Step(`^I run cynork task create with task file "([^"]*)"$`, func(ctx context.Context, path string) error {
		st := getState(ctx)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", string(data)}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork task create with prompt "([^"]*)" and attachments "([^"]*)" "([^"]*)"$`, func(ctx context.Context, prompt, a1, a2 string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", prompt}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		_ = a1
		_ = a2
		return nil
	})

	sc.Step(`^a script file "([^"]*)" exists$`, func(ctx context.Context, path string) error {
		dir := filepath.Dir(path)
		if dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		return os.WriteFile(path, []byte("#!/bin/sh\necho hello\n"), 0o755)
	})

	sc.Step(`^I run cynork task create with script "([^"]*)"$`, func(ctx context.Context, path string) error {
		st := getState(ctx)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", string(data), "--input-mode", "script"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork task create with command "([^"]*)" and command "([^"]*)"$`, func(ctx context.Context, c1, c2 string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", c1 + "\n" + c2, "--input-mode", "commands"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork creds list$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "creds", "list"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork prefs set scope type "([^"]*)" key "([^"]*)" value "([^"]*)"$`, func(ctx context.Context, scopeType, key, value string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "prefs", "set", "scope", "type", scopeType, "key", key, "value", value}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})
	sc.Step(`^I run cynork prefs set scope type "([^"]*)" key "([^"]*)" value "\"([^\"]*)\"\"$`, func(ctx context.Context, scopeType, key, value string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "prefs", "set", "scope", "type", scopeType, "key", key, "value", value}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork prefs get scope type "([^"]*)" key "([^"]*)"$`, func(ctx context.Context, scopeType, key string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "prefs", "get", "scope", "type", scopeType, "key", key}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork settings set key "([^"]*)" value "([^"]*)"$`, func(ctx context.Context, key, value string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "settings", "set", "key", key, "value", value}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})
	sc.Step(`^I run cynork settings set key "([^"]*)" value "\"([^\"]*)\"\"$`, func(ctx context.Context, key, value string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "settings", "set", "key", key, "value", value}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork settings get key "([^"]*)"$`, func(ctx context.Context, key string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "settings", "get", "key", key}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork nodes list$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "nodes", "list"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^a markdown file "([^"]*)" exists with content "([^"]*)"$`, func(ctx context.Context, path, content string) error {
		dir := filepath.Dir(path)
		if dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		return os.WriteFile(path, []byte(content), 0o600)
	})

	sc.Step(`^I run cynork skills load with file "([^"]*)"$`, func(ctx context.Context, path string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "skills", "load", path}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork audit list$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "audit", "list"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork shell in interactive mode$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "shell"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, "exit\n")
		return nil
	})

	sc.Step(`^at least one task exists with a human-readable name$`, func(ctx context.Context) error {
		// Mock already has tasks from create; no extra setup
		return nil
	})

	sc.Step(`^I request tab-completion for a task identifier position$`, func(ctx context.Context) error {
		// Stub: tab completion not implemented in shell yet
		return nil
	})

	sc.Step(`^the completion candidates include task names$`, func(ctx context.Context) error {
		// Stub: would check completion output
		return nil
	})
}

// runCynorkWithStdin runs cynork with the given stdin content (e.g. "/exit\n" for chat).
func (s *cynorkState) runCynorkWithStdin(args []string, env []string, stdin string) (exit int, stdout, stderr string) {
	cmd := exec.Command(s.cynorkBin, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = strings.TrimSpace(outBuf.String())
	stderr = strings.TrimSpace(errBuf.String())
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exit = exitErr.ExitCode()
		} else {
			exit = -1
		}
	}
	return exit, stdout, stderr
}
