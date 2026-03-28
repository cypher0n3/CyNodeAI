// Package bdd – HTTP ServeMux factory for cynork BDD mock gateway (auth, tasks, chat, models, etc.).
package bdd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

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
		s.mu.Lock()
		forceErr := s.authErrorNextReq
		if forceErr {
			s.authErrorNextReq = false
		}
		s.mu.Unlock()
		if forceErr {
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
		s.mu.Lock()
		force404 := s.task404Mode
		s.mu.Unlock()
		if force404 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		prompt, ok := s.tasks[id]
		taskName := s.taskNames[id]
		statusOverride := s.taskStatuses[id]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		status := "completed"
		if statusOverride != "" {
			status = statusOverride
		}
		payload := map[string]any{"id": id, "task_id": id, "status": status, "prompt": prompt}
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
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.chatCompleted = true
		if req.Model != "" {
			s.lastChatModel = req.Model
		}
		s.lastChatProjectHeader = r.Header.Get("OpenAI-Project")
		s.lastChatStream = req.Stream
		s.mu.Unlock()
		resp := ""
		if len(req.Messages) > 0 {
			resp = req.Messages[len(req.Messages)-1].Content
			if strings.HasPrefix(resp, "echo ") {
				resp = strings.TrimSpace(strings.TrimPrefix(resp, "echo "))
			}
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fl, _ := w.(http.Flusher)
			parts := []string{"to", "ken"}
			if s.bddStreamDegraded {
				parts = []string{"full answer"}
			}
			for _, p := range parts {
				bddWriteChatCompletionDelta(w, fl, p, nil)
			}
			stop := "stop"
			bddWriteChatCompletionDelta(w, fl, "", &stop)
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if fl != nil {
				fl.Flush()
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": resp}, "finish_reason": "stop"},
			},
		})
	})
	mux.HandleFunc("POST /v1/responses", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fl, _ := w.(http.Flusher)
			bddWriteSSELine(w, fl, "", []byte(`{"response_id":"bdd-resp-1"}`))
			delta, _ := json.Marshal(map[string]string{"delta": "resp"})
			bddWriteSSELine(w, fl, userapi.SSEEventResponseOutputTextDelta, delta)
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if fl != nil {
				fl.Flush()
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "bdd-resp-json",
			"object":  "response",
			"created": 1,
			"output": []map[string]string{
				{"type": "text", "text": "responses non-stream ok"},
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
		s.mu.Lock()
		errMode := s.prefsErrorMode
		if errMode {
			s.prefsErrorMode = false
		}
		s.mu.Unlock()
		if errMode {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"prefs unavailable"}`))
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
		multi := s.multipleSkillsMode
		setup := s.skillSelectorSetup
		s.mu.Unlock()
		skills := []map[string]any{}
		if multi {
			skills = append(skills,
				map[string]any{"id": "skill-foo-1", "name": "foo-alpha", "scope": "user", "updated_at": "2026-01-01T00:00:00Z"},
				map[string]any{"id": "skill-foo-2", "name": "foo-beta", "scope": "user", "updated_at": "2026-01-01T00:00:00Z"},
			)
		} else if id != "" {
			skills = append(skills, map[string]any{"id": id, "name": "Test skill", "scope": "user", "updated_at": "2026-01-01T00:00:00Z"})
		} else if setup != "" {
			skills = append(skills, map[string]any{"id": "skill-setup-1", "name": setup, "scope": "user", "updated_at": "2026-01-01T00:00:00Z"})
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
		setup := s.skillSelectorSetup
		s.mu.Unlock()
		// Accept exact id, "team-guide" (legacy test helper), or the setup selector.
		if id != expected && id != "team-guide" && (setup == "" || id != setup) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resolveID := id
		if id == "team-guide" && expected != "" {
			resolveID = expected
		}
		name := "Test skill"
		if setup != "" && id == setup {
			name = setup
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": resolveID, "name": name, "scope": "user", "content": "# Test skill",
			"updated_at": "2026-01-01T00:00:00Z",
		})
	})
	mux.HandleFunc("DELETE /v1/skills/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})
	mux.HandleFunc("PUT /v1/skills/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": "updated", "scope": "user"})
	})
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		errMode := s.modelsErrorMode
		ids := s.modelIDs
		s.mu.Unlock()
		if errMode {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"models unavailable"}`))
			return
		}
		models := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			models = append(models, map[string]any{"id": id, "object": "model"})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": models})
	})
	mux.HandleFunc("GET /v1/prefs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		errMode := s.prefsErrorMode
		if errMode {
			s.prefsErrorMode = false
		}
		s.mu.Unlock()
		if errMode {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"prefs unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})
	mux.HandleFunc("DELETE /v1/prefs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})
	mux.HandleFunc("GET /v1/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		projects := s.projectsByID
		s.mu.Unlock()
		list := make([]map[string]any, 0, len(projects))
		for id, p := range projects {
			item := map[string]any{"id": id}
			for k, v := range p {
				item[k] = v
			}
			list = append(list, item)
		}
		if len(list) == 0 {
			list = []map[string]any{{"id": "proj-default", "name": "Default Project"}}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": list})
	})
	mux.HandleFunc("GET /v1/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		proj, ok := s.projectsByID[id]
		s.mu.Unlock()
		if !ok {
			proj = map[string]any{"id": id, "name": "Project " + id}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(proj)
	})
	mux.HandleFunc("GET /v1/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		node, ok := s.nodesByID[id]
		s.mu.Unlock()
		if !ok {
			node = map[string]any{"id": id, "status": "online"}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(node)
	})
	mux.HandleFunc("GET /v1/tasks/{id}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		s.mu.Lock()
		artifacts := s.taskArtifactsByID[id]
		s.mu.Unlock()
		list := make([]map[string]any, 0, len(artifacts))
		for _, a := range artifacts {
			list = append(list, map[string]any{"name": a})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": list})
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

func bddWriteSSELine(w http.ResponseWriter, fl http.Flusher, event string, data []byte) {
	if event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", event)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	if fl != nil {
		fl.Flush()
	}
}

func bddWriteChatCompletionDelta(w http.ResponseWriter, fl http.Flusher, content string, finishReason *string) {
	chunk := userapi.ChatCompletionChunk{
		ID:      "bdd-chunk",
		Object:  "chat.completion.chunk",
		Created: 1,
		Model:   "bdd-model",
		Choices: []userapi.ChatCompletionChunkChoice{
			{Index: 0, Delta: userapi.ChatCompletionChunkDelta{Content: content}, FinishReason: finishReason},
		},
	}
	b, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	bddWriteSSELine(w, fl, "", b)
}
