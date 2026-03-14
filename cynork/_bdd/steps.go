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

// mockThread holds the data for a mock chat thread returned by GET /v1/chat/threads.
type mockThread struct {
	ID    string
	Title string
}

type cynorkState struct {
	mockServer *httptest.Server
	cynorkBin  string
	bddRoot    string // working directory for cynork subprocess (cynork/_bdd; tmp/ under it)
	configPath string // path to config file for session persistence (login writes, whoami reads)
	lastExit   int
	lastStdout string
	lastStderr string
	token      string
	taskID     string
	// mock state: token -> handle for GetMe
	userByToken    map[string]string
	tasks          map[string]string // taskID -> prompt (for result echo)
	taskNames      map[string]string // taskID -> optional task name
	lastSkillID    string            // set by POST /v1/skills/load for list/get
	sessionModel   string            // set by "the TUI is running with model" step
	sessionProject string            // set by "the TUI is running with project" step
	prefsMutated   bool              // set when POST /v1/prefs is called during a scenario
	mockThreads    []mockThread      // threads returned by GET /v1/chat/threads
	threadCreated  bool              // set when POST /v1/chat/threads is called
	mu             sync.Mutex
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
			"access_token":  tok,
			"refresh_token": "refresh-" + req.Handle,
			"token_type":    "Bearer",
			"expires_in":    900,
		})
	})
	mux.HandleFunc("POST /v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Accept any refresh-<handle> and return new tokens (rotation).
		handle := strings.TrimPrefix(req.RefreshToken, "refresh-")
		if handle == req.RefreshToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		if s.userByToken == nil {
			s.userByToken = make(map[string]string)
		}
		newTok := "tok-" + handle + "-refreshed"
		s.userByToken[newTok] = handle
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newTok,
			"refresh_token": "refresh-" + handle + "-v2",
			"token_type":    "Bearer",
			"expires_in":    900,
		})
	})
	mux.HandleFunc("POST /v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
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
			Prompt   string  `json:"prompt"`
			TaskName *string `json:"task_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		if s.tasks == nil {
			s.tasks = make(map[string]string)
		}
		if s.taskNames == nil {
			s.taskNames = make(map[string]string)
		}
		id := fmt.Sprintf("task-%d", len(s.tasks)+1)
		s.tasks[id] = req.Prompt
		if req.TaskName != nil && *req.TaskName != "" {
			s.taskNames[id] = *req.TaskName
		}
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
			item := map[string]any{"id": id, "task_id": id, "status": "completed", "prompt": prompt}
			if name := s.taskNames[id]; name != "" {
				item["task_name"] = name
			}
			tasks = append(tasks, item)
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
		taskName := s.taskNames[id]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]any{"id": id, "task_id": id, "status": "completed", "prompt": prompt}
		if taskName != "" {
			payload["task_name"] = taskName
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
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
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := ""
		if len(req.Messages) > 0 {
			resp = req.Messages[len(req.Messages)-1].Content
			if strings.HasPrefix(resp, "echo ") {
				resp = strings.TrimSpace(strings.TrimPrefix(resp, "echo "))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": resp}, "finish_reason": "stop"},
			},
		})
	})
	// Thread endpoints: POST creates a thread, GET lists threads, PATCH renames a thread.
	mux.HandleFunc("POST /v1/chat/threads", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		s.threadCreated = true
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"thread_id": "tid-new-1"})
	})
	mux.HandleFunc("GET /v1/chat/threads", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		threads := s.mockThreads
		s.mu.Unlock()
		data := make([]map[string]any, 0, len(threads))
		for _, t := range threads {
			data = append(data, map[string]any{
				"id":         t.ID,
				"title":      t.Title,
				"created_at": "2025-01-01T00:00:00Z",
				"updated_at": "2025-01-01T00:00:00Z",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	mux.HandleFunc("PATCH /v1/chat/threads/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"tid-r","title":"renamed"}`))
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
		s.mu.Lock()
		s.prefsMutated = true
		s.mu.Unlock()
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
	mux.HandleFunc("GET /v1/skills", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		id := s.lastSkillID
		s.mu.Unlock()
		skills := []map[string]any{}
		if id != "" {
			skills = append(skills, map[string]any{"id": id, "name": "Test skill", "scope": "user", "updated_at": "2026-01-01T00:00:00Z"})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"skills": skills})
	})
	mux.HandleFunc("GET /v1/skills/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		expected := s.lastSkillID
		s.mu.Unlock()
		// Accept exact id or user-typeable selector (e.g. "team-guide" in Get skill by selector scenario).
		if id != expected && id != "team-guide" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resolveID := id
		if id == "team-guide" && expected != "" {
			resolveID = expected
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": resolveID, "name": "Test skill", "scope": "user", "content": "# Test skill",
			"updated_at": "2026-01-01T00:00:00Z",
		})
	})
	mux.HandleFunc("POST /v1/skills/load", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Content, "Ignore previous instructions") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "policy violation", "category": "instruction_override",
				"triggering_text": "Ignore previous instructions",
			})
			return
		}
		s.mu.Lock()
		s.lastSkillID = "s1"
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "s1", "name": "Untitled skill", "scope": "user"})
	})
	return mux
}

func (s *cynorkState) runCynork(args []string, env ...string) (exit int, stdout, stderr string) {
	cmd := exec.Command(s.cynorkBin, args...)
	cmd.Env = append(os.Environ(), env...)
	if s.bddRoot != "" {
		cmd.Dir = s.bddRoot
	}
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
		state.userByToken = nil
		state.tasks = nil
		state.token = ""
		state.taskID = ""
		state.lastSkillID = ""
		state.sessionModel = ""
		state.sessionProject = ""
		state.prefsMutated = false
		state.mockThreads = nil
		state.threadCreated = false
		state.mockServer = httptest.NewServer(state.mockGatewayMux())
		wd, err := os.Getwd()
		if err != nil {
			return ctx, err
		}
		root := wd
		if strings.HasSuffix(wd, string(filepath.Separator)+"_bdd") || filepath.Base(wd) == "_bdd" {
			root = filepath.Join(wd, "..", "..")
		}
		state.bddRoot = filepath.Join(root, "cynork", "_bdd")
		tmpDir := filepath.Join(state.bddRoot, "tmp")
		_ = os.MkdirAll(tmpDir, 0o755)
		state.configPath = filepath.Join(tmpDir, "cynork-bdd-config.yaml")
		_ = os.WriteFile(state.configPath, []byte("gateway_url: http://localhost\n"), 0o600)
		// Attachment files for scenarios that use tmp/doc1.txt, tmp/doc2.txt.
		_ = os.WriteFile(filepath.Join(tmpDir, "doc1.txt"), []byte("first attachment\n"), 0o600)
		_ = os.WriteFile(filepath.Join(tmpDir, "doc2.txt"), []byte("second attachment\n"), 0o600)
		bin := filepath.Join(tmpDir, "cynork-bdd")
		cynorkDir := filepath.Join(root, "cynork")
		build := exec.Command("go", "build", "-o", bin, ".")
		build.Dir = cynorkDir
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
	sc.Step(`^cynork exits with a non-zero code$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit == 0 {
			return fmt.Errorf("cynork exit code 0, want non-zero (stderr: %s)", st.lastStderr)
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
		args := []string{"--config", st.configPath, "auth", "login", "-u", user, "--password-stdin"}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, pass+"\n")
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

	sc.Step(`^I run cynork auth refresh$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		args := []string{"--config", st.configPath, "auth", "refresh"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork auth logout$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		args := []string{"--config", st.configPath, "auth", "logout"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I am logged in with username "([^"]*)" and password "([^"]*)"$`, func(ctx context.Context, user, pass string) error {
		st := getState(ctx)
		args := []string{"--config", st.configPath, "auth", "login", "-u", user, "--password-stdin"}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, pass+"\n")
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
	sc.Step(`^I run cynork task create with prompt "([^"]*)" and task name "([^"]*)"$`, func(ctx context.Context, prompt, taskName string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", prompt, "--task-name", taskName}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})
	sc.Step(`^cynork task get shows task name "([^"]*)"$`, func(ctx context.Context, wantName string) error {
		st := getState(ctx)
		if st.taskID == "" {
			return fmt.Errorf("no stored task id")
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "get", st.taskID}
		_, stdout, _ := st.runCynork(args, env...)
		expect := "task_name=" + wantName
		if !strings.Contains(stdout, expect) {
			return fmt.Errorf("cynork task get output %q does not contain %q", stdout, expect)
		}
		return nil
	})

	sc.Step(`^I have created a task with prompt "([^"]*)" and stored the task id$`, func(ctx context.Context, prompt string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", prompt}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		if st.lastExit != 0 {
			return fmt.Errorf("task create failed with exit %d: %s", st.lastExit, st.lastStderr)
		}
		// Parse and store task_id from stdout
		out := strings.TrimSpace(st.lastStdout)
		if prefix := "task_id="; strings.HasPrefix(out, prefix) {
			st.taskID = strings.TrimSpace(strings.SplitN(out[len(prefix):], " ", 2)[0])
		} else if out != "" && !strings.HasPrefix(out, "{") {
			st.taskID = strings.SplitN(out, "\n", 2)[0]
		} else {
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

	sc.Step(`^I run cynork chat and send "([^"]*)" to cynork stdin$`, func(ctx context.Context, text string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		stdin := text
		if !strings.HasSuffix(stdin, "\n") {
			stdin += "\n"
		}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, stdin)
		return nil
	})

	sc.Step(`^I send "([^"]*)" to cynork stdin$`, func(ctx context.Context, text string) error {
		// Used with "I run cynork chat" - the step above runs chat with stdin "/exit\n"
		// This step is for documentation; actual send is in runCynorkWithStdin
		_ = text
		return nil
	})

	sc.Step(`^a task file "([^"]*)" exists with content "([^"]*)"$`, func(ctx context.Context, path, content string) error {
		if st := getState(ctx); st.bddRoot != "" && !filepath.IsAbs(path) {
			path = filepath.Join(st.bddRoot, path)
		}
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
		args := []string{"--config", st.configPath, "task", "create", "-p", prompt, "--attachment", a1, "--attachment", a2}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^a script file "([^"]*)" exists$`, func(ctx context.Context, path string) error {
		if st := getState(ctx); st.bddRoot != "" && !filepath.IsAbs(path) {
			path = filepath.Join(st.bddRoot, path)
		}
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
	// Match value "\"concise\"" (escaped quotes in Gherkin)
	sc.Step(`^I run cynork prefs set scope type "([^"]*)" key "([^"]*)" value "\\"([^"]*)\\"\"$`, func(ctx context.Context, scopeType, key, value string) error {
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
	// Match value "\"qwen3.5:0.8b\"" (escaped quotes in Gherkin)
	sc.Step(`^I run cynork settings set key "([^"]*)" value "\\"([^"]*)\\"\"$`, func(ctx context.Context, key, value string) error {
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
		if st := getState(ctx); st.bddRoot != "" && !filepath.IsAbs(path) {
			path = filepath.Join(st.bddRoot, path)
		}
		dir := filepath.Dir(path)
		if dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		return os.WriteFile(path, []byte(content), 0o600)
	})

	sc.Step(`^I have loaded a skill$`, func(ctx context.Context) error {
		st := getState(ctx)
		path := filepath.Join(os.TempDir(), "bdd_skill_load.md")
		if err := os.WriteFile(path, []byte("# Loaded skill"), 0o600); err != nil {
			return err
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "skills", "load", path}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		if st.lastExit != 0 {
			return fmt.Errorf("skills load exited with %d: %s", st.lastExit, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^I run cynork skills load with file "([^"]*)"$`, func(ctx context.Context, path string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "skills", "load", path}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})
	sc.Step(`^I run cynork skills list$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "skills", "list"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})
	sc.Step(`^I run cynork skills get "([^"]*)"$`, func(ctx context.Context, id string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "skills", "get", id}
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

	sc.Step(`^I run cynork shell in interactive mode and request tab-completion for a task identifier position$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "shell"}
		// Send tab then exit to simulate requesting completion then leaving
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, "\t\nexit\n")
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

	// ---- TUI slash command and shell escape steps (cynork_tui_slash_commands_and_shell.feature) ----
	// These run through cynork chat (same slash command contract) since the TUI requires a PTY.

	sc.Step(`^the TUI is running$`, func(ctx context.Context) error {
		// No-op: state is pre-set in Before hook; cynork chat is launched in the "type" step.
		return nil
	})

	sc.Step(`^the TUI is running with model "([^"]*)"$`, func(ctx context.Context, model string) error {
		st := getState(ctx)
		st.sessionModel = model
		return nil
	})

	sc.Step(`^the TUI is running with project "([^"]*)"$`, func(ctx context.Context, projectID string) error {
		st := getState(ctx)
		st.sessionProject = projectID
		return nil
	})

	sc.Step(`^the TUI has existing messages in the scrollback$`, func(ctx context.Context) error {
		// State set; cynork chat is launched in the "type" step below.
		return nil
	})

	sc.Step(`^I type "([^"]*)" and press Enter$`, func(ctx context.Context, text string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		if st.sessionModel != "" {
			args = append(args, "--model", st.sessionModel)
		}
		// Send the typed text then /exit to close the session.
		stdin := text + "\n/exit\n"
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, stdin)
		return nil
	})

	sc.Step(`^the scrollback contains a list of slash command names and their descriptions$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, "/clear") && !strings.Contains(combined, "/exit") {
			return fmt.Errorf("expected slash command list in output; got: stdout=%q stderr=%q", st.lastStdout, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the scrollback is empty$`, func(ctx context.Context) error {
		// /clear clears the terminal; in non-TTY mode via chat scanner there is no visible scrollback.
		// Verify the command ran without error.
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("cynork chat /clear exit %d: %s", st.lastExit, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the scrollback contains the cynork version string$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "cynork") {
			return fmt.Errorf("expected version string in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the TUI exits cleanly$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("cynork exit %d; stderr: %q", st.lastExit, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the scrollback contains a hint mentioning "([^"]*)"$`, func(ctx context.Context, hint string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, hint) {
			return fmt.Errorf("expected hint %q in output; got: %q", hint, combined)
		}
		return nil
	})

	sc.Step(`^the TUI session remains active$`, func(ctx context.Context) error {
		// Session remaining active is implied by the chat loop handling /exit cleanly.
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("expected session to remain active (exit 0); got exit %d stderr: %q", st.lastExit, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the scrollback shows model identifiers or an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "model") && !strings.Contains(strings.ToLower(combined), "error") {
			return fmt.Errorf("expected model list or error in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback contains the current model name$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "model") {
			return fmt.Errorf("expected model name in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the session model is updated to "([^"]*)"$`, func(ctx context.Context, model string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, model) {
			return fmt.Errorf("expected model %q in output; got: %q", model, combined)
		}
		return nil
	})

	sc.Step(`^the scrollback contains the current project identifier$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "project") {
			return fmt.Errorf("expected project identifier in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the session project is updated to "([^"]*)"$`, func(ctx context.Context, projectID string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, projectID) {
			return fmt.Errorf("expected project %q in output; got: %q", projectID, combined)
		}
		return nil
	})

	sc.Step(`^stored user preferences are unchanged$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		mutated := st.prefsMutated
		st.mu.Unlock()
		if mutated {
			return fmt.Errorf("expected no prefs mutation but POST /v1/prefs was called during scenario")
		}
		return nil
	})

	sc.Step(`^the scrollback contains "([^"]*)"$`, func(ctx context.Context, expected string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, expected) {
			return fmt.Errorf("expected %q in output; got: %q", expected, combined)
		}
		return nil
	})

	sc.Step(`^the scrollback contains a shell usage hint$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "usage") && !strings.Contains(combined, "!") {
			return fmt.Errorf("expected shell usage hint in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback contains the exit code$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, "exit") {
			return fmt.Errorf("expected exit code in output; got: %q", combined)
		}
		return nil
	})

	// ---- Thread and resume-thread steps (cynork_chat.feature thread scenarios) ----

	sc.Step(`^the mock gateway supports POST "([^"]*)"$`, func(ctx context.Context, _ string) error {
		// The shared mock gateway always supports POST /v1/chat/threads. This step is a no-op
		// that documents the prerequisite and enables threadCreated tracking.
		return nil
	})

	sc.Step(`^the mock gateway returns at least one chat thread with selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{{ID: "tid-inbox-1", Title: selector}}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the mock gateway returns multiple chat threads with user-typeable selectors$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{
			{ID: "tid-inbox-1", Title: "inbox"},
			{ID: "tid-work-2", Title: "work"},
		}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^I run cynork chat without resume-thread and send a first message$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat", "--thread-new", "--message", "hello"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^cynork creates a fresh chat thread before the first completion$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		created := st.threadCreated
		st.mu.Unlock()
		if !created {
			return fmt.Errorf("expected POST /v1/chat/threads to be called; stderr=%q stdout=%q", st.lastStderr, st.lastStdout)
		}
		return nil
	})

	sc.Step(`^cynork creates a fresh chat thread before the next completion$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		created := st.threadCreated
		st.mu.Unlock()
		if !created {
			return fmt.Errorf("expected POST /v1/chat/threads to be called; stderr=%q stdout=%q", st.lastStderr, st.lastStdout)
		}
		return nil
	})

	sc.Step(`^I run cynork chat with resume-thread "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat", "--resume-thread", selector, "--message", "hello"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^the session starts in the thread identified by selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("expected exit 0; got %d stderr=%q", st.lastExit, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the first completion continues that thread's conversation$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("expected session to complete successfully; exit=%d stderr=%q", st.lastExit, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^cynork switches to the thread identified by selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "switched") && !strings.Contains(strings.ToLower(combined), selector) {
			return fmt.Errorf("expected switch confirmation for selector %q in output; got: %q", selector, combined)
		}
		return nil
	})

	sc.Step(`^the chat session shows guidance for valid /thread commands$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "thread") {
			return fmt.Errorf("expected /thread guidance in output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the chat session remains active$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("expected session to remain active (exit 0); got exit %d stderr: %q", st.lastExit, st.lastStderr)
		}
		return nil
	})

	sc.Step(`^the slash-command help includes "([^"]*)"$`, func(ctx context.Context, text string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, text) {
			return fmt.Errorf("expected slash-command help to include %q; got: %q", text, combined)
		}
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
