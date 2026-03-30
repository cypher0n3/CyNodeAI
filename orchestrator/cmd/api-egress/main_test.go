package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// TestTokenAuth exercises bearer validation on the egress call handler (constant-time compare).
func TestTokenAuth(t *testing.T) {
	h := newCallHandler(slog.Default(), "correct-token", "openai,github")
	body := []byte(`{"provider":"openai","operation":"chat"}`)
	tests := []struct {
		name       string
		authHeader string
		wantCode   int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "Bearer wrong-token", http.StatusUnauthorized},
		{"correct", "Bearer correct-token", http.StatusNotImplemented},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			if tc.authHeader != "" {
				r.Header.Set("Authorization", tc.authHeader)
			}
			h.ServeHTTP(w, r)
			if w.Code != tc.wantCode {
				t.Errorf("code=%d want %d", w.Code, tc.wantCode)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_AE_ENV")
	if getEnv("TEST_AE_ENV", "def") != "def" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_AE_ENV", "val")
	defer func() { _ = os.Unsetenv("TEST_AE_ENV") }()
	if getEnv("TEST_AE_ENV", "def") != "val" {
		t.Error("from env")
	}
}

func TestRun_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.Default()
	err := run(ctx, logger)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("run: %v", err)
	}
}

func runWithEnvAndExpectError(t *testing.T, envKey, envVal, expectErrMsg string) {
	t.Helper()
	old := os.Getenv(envKey)
	_ = os.Setenv(envKey, envVal)
	defer func() {
		if old != "" {
			_ = os.Setenv(envKey, old)
		} else {
			_ = os.Unsetenv(envKey)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := run(ctx, slog.Default())
	if err == nil {
		t.Error(expectErrMsg)
	}
}

func TestRun_WithInvalidDSN_ReturnsError(t *testing.T) {
	runWithEnvAndExpectError(t, "API_EGRESS_DSN", "postgres://invalid/host", "run with invalid DSN: expected error")
}

func TestRun_ListenAndServeFails(t *testing.T) {
	runWithEnvAndExpectError(t, "LISTEN_ADDR", ":99999", "expected error when ListenAndServe fails (invalid port)")
}

func TestRunMain_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := runMain(ctx)
	if code != 0 {
		t.Errorf("runMain: got %d", code)
	}
}

func TestShutdownTimeout(t *testing.T) {
	_ = os.Unsetenv("API_EGRESS_SHUTDOWN_SEC")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("default: got %v", shutdownTimeout())
	}
	_ = os.Setenv("API_EGRESS_SHUTDOWN_SEC", "5")
	defer func() { _ = os.Unsetenv("API_EGRESS_SHUTDOWN_SEC") }()
	if shutdownTimeout() != 5*time.Second {
		t.Errorf("from env: got %v", shutdownTimeout())
	}
	_ = os.Setenv("API_EGRESS_SHUTDOWN_SEC", "x")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("invalid env should use default: got %v", shutdownTimeout())
	}
}

func TestRunMain_RunFails(t *testing.T) {
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	code := runMain(ctx)
	if code != 1 {
		t.Errorf("runMain when run fails: got %d", code)
	}
}

func TestCallHandler_MethodNotAllowed(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/call", http.NoBody)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d", w.Code)
	}
}

func TestCallHandler_Unauthorized(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader([]byte(`{"provider":"openai","operation":"chat"}`)))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no bearer: got %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader([]byte(`{"provider":"openai","operation":"chat"}`)))
	r2.Header.Set("Content-Type", "application/json")
	r2.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(w2, r2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("wrong bearer: got %d", w2.Code)
	}
}

func TestCallHandler_BadRequest(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON: got %d", w.Code)
	}
}

func TestCallHandler_Forbidden(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	body := map[string]string{"provider": "unknown", "operation": "op", "task_id": "t1"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("disallowed provider: got %d", w.Code)
	}
}

func TestCallHandler_AllowedReturns501(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	body := map[string]string{"provider": "openai", "operation": "chat", "task_id": "t2"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("allowed provider: got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["title"] != "Not Implemented" {
		t.Errorf("title: %v", resp["title"])
	}
}

func TestCallHandler_WithStore_AllowReturns501(t *testing.T) {
	mock := testutil.NewMockDB()
	user := &models.User{
		UserBase: models.UserBase{Handle: "ae-user", IsActive: true},
		ID:       uuid.New(),
	}
	mock.AddUser(user)
	task := &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy: &user.ID,
			Status:    "running",
		},
		ID: uuid.New(),
	}
	mock.AddTask(task)
	mock.AccessControlRules = []*models.AccessControlRule{
		{
			AccessControlRuleBase: models.AccessControlRuleBase{
				Effect:          "allow",
				ResourcePattern: "openai/chat",
				Action:          database.ActionApiCall,
				ResourceType:    database.ResourceTypeProviderOperation,
			},
		},
	}
	mock.HasActiveApiCredential = true
	h := newCallHandlerWithStore(slog.Default(), "secret", "openai,github", mock)
	body := map[string]string{"provider": "openai", "operation": "chat", "task_id": task.ID.String()}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("with store allow: got %d", w.Code)
	}
}

func TestCallHandler_WithStore_NoTaskID_403(t *testing.T) {
	mock := testutil.NewMockDB()
	h := newCallHandlerWithStore(slog.Default(), "secret", "openai,github", mock)
	body := map[string]string{"provider": "openai", "operation": "chat"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("no task_id: got %d", w.Code)
	}
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["detail"] != "task_id required" {
		t.Errorf("detail: %v", resp["detail"])
	}
}

func TestCallHandler_WithStore_InvalidTaskID_403(t *testing.T) {
	mock := testutil.NewMockDB()
	h := newCallHandlerWithStore(slog.Default(), "secret", "openai,github", mock)
	body := map[string]string{"provider": "openai", "operation": "chat", "task_id": "not-a-uuid"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("invalid task_id: got %d", w.Code)
	}
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["detail"] != "invalid task_id" {
		t.Errorf("detail: %v", resp["detail"])
	}
}

func setupMockWithUserAndTask(mock *testutil.MockDB, handle string) *models.Task {
	user := &models.User{
		UserBase: models.UserBase{Handle: handle, IsActive: true},
		ID:       uuid.New(),
	}
	mock.AddUser(user)
	task := &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy: &user.ID,
			Status:    "running",
		},
		ID: uuid.New(),
	}
	mock.AddTask(task)
	return task
}

func TestCallHandler_WithStore_PolicyOrCredentialDeny_403(t *testing.T) {
	for name, tc := range map[string]struct {
		effect     string
		hasCred    bool
		wantDetail string
	}{
		"no_credential": {"allow", false, "no active credential for provider"},
		"policy_deny":   {"deny", true, "policy denies provider/operation"},
	} {
		t.Run(name, func(t *testing.T) {
			mock := testutil.NewMockDB()
			task := setupMockWithUserAndTask(mock, "ae-user-"+name)
			mock.AccessControlRules = []*models.AccessControlRule{
				{
					AccessControlRuleBase: models.AccessControlRuleBase{
						Effect:          tc.effect,
						ResourcePattern: "openai/chat",
						Action:          database.ActionApiCall,
						ResourceType:    database.ResourceTypeProviderOperation,
					},
				},
			}
			mock.HasActiveApiCredential = tc.hasCred
			code, detail := callWithStoreAndAssert403(t, mock, task.ID.String())
			if code != http.StatusForbidden || detail != tc.wantDetail {
				t.Errorf("code=%d detail=%q", code, detail)
			}
		})
	}
}

func TestCallHandler_WithStore_TaskNotFound_403(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.GetTaskByIDErr = database.ErrNotFound
	h := newCallHandlerWithStore(slog.Default(), "secret", "openai,github", mock)
	body := map[string]string{"provider": "openai", "operation": "chat", "task_id": uuid.New().String()}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("task not found: got %d", w.Code)
	}
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["detail"] != "task not found" {
		t.Errorf("detail: %v", resp["detail"])
	}
}

func callWithStoreAndAssert403(t *testing.T, mock *testutil.MockDB, taskID string) (code int, detail string) {
	t.Helper()
	h := newCallHandlerWithStore(slog.Default(), "secret", "openai,github", mock)
	body := map[string]string{"provider": "openai", "operation": "chat", "task_id": taskID}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	detail, _ = resp["detail"].(string)
	code = w.Code
	return code, detail
}

func TestCallHandler_WithStore_NoUserContext_403(t *testing.T) {
	mock := testutil.NewMockDB()
	task := &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy: nil,
			Status:    "running",
		},
		ID: uuid.New(),
	}
	mock.AddTask(task)
	h := newCallHandlerWithStore(slog.Default(), "secret", "openai,github", mock)
	body := map[string]string{"provider": "openai", "operation": "chat", "task_id": task.ID.String()}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("no user context: got %d", w.Code)
	}
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["detail"] != "task has no user context" {
		t.Errorf("detail: %v", resp["detail"])
	}
}

func TestCallHandler_OversizeJSONBody(t *testing.T) {
	h := newCallHandler(slog.Default(), "", "openai")
	pad := strings.Repeat("x", int(httplimits.DefaultMaxAPIRequestBodyBytes)+1024)
	body := []byte(`{"provider":"openai","operation":"chat","task_id":"` + pad + `"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code %d want 413", w.Code)
	}
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
