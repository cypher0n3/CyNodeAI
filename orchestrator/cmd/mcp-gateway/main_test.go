package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestIntArg_IntType(t *testing.T) {
	args := map[string]interface{}{"limit": 5}
	if got := intArg(args, "limit"); got != 5 {
		t.Errorf("intArg int: got %d", got)
	}
}

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_MCP_ENV")
	if getEnv("TEST_MCP_ENV", "def") != "def" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_MCP_ENV", "val")
	defer func() { _ = os.Unsetenv("TEST_MCP_ENV") }()
	if getEnv("TEST_MCP_ENV", "def") != "val" {
		t.Error("from env")
	}
}

func TestRun_CancelledContext(t *testing.T) {
	// Ensure no real DB so run() uses nil store and exits on cancelled ctx without hitting Open.
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.Default()
	err := run(ctx, logger)
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

func TestRun_ListenAndServeFails(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
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
	logger := slog.Default()
	err := run(ctx, logger)
	if err == nil {
		t.Error("expected error when ListenAndServe fails (invalid port)")
	}
}

func TestRunMain_Success(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := runMain(ctx)
	if code != 0 {
		t.Errorf("runMain: got %d", code)
	}
}

func TestShutdownTimeout(t *testing.T) {
	_ = os.Unsetenv("MCP_GATEWAY_SHUTDOWN_SEC")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("default: got %v", shutdownTimeout())
	}
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "5")
	defer func() { _ = os.Unsetenv("MCP_GATEWAY_SHUTDOWN_SEC") }()
	if shutdownTimeout() != 5*time.Second {
		t.Errorf("from env: got %v", shutdownTimeout())
	}
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "x")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("invalid env should use default: got %v", shutdownTimeout())
	}
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "0")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("zero should use default: got %v", shutdownTimeout())
	}
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "-1")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("negative should use default: got %v", shutdownTimeout())
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

func TestToolCallHandler_StoreNil(t *testing.T) {
	handler := toolCallHandler(nil, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(`{"tool_name":"db.preference.get"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("store nil: got status %d", rec.Code)
	}
}

// callToolHandlerWithStore sends a POST with the given store and body, and asserts the response code.
func callToolHandlerWithStore(t *testing.T, store database.Store, body string, wantCode int) {
	t.Helper()
	handler := toolCallHandler(store, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != wantCode {
		t.Errorf("got status %d, want %d", rec.Code, wantCode)
	}
}

// callToolHandlerPOST sends a POST with body and asserts the response code.
func callToolHandlerPOST(t *testing.T, body string, wantCode int) {
	t.Helper()
	handler := toolCallHandler(testutil.NewMockDB(), slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != wantCode {
		t.Errorf("got status %d, want %d", rec.Code, wantCode)
	}
}

func TestToolCallHandler_StoreSet_WritesAudit(t *testing.T) {
	// Use a non-routed tool so gateway returns 501 after writing audit.
	callToolHandlerPOST(t, `{"tool_name":"other.tool"}`, http.StatusNotImplemented)
}

func TestToolCallHandler_InvalidJSON(t *testing.T) {
	callToolHandlerPOST(t, `{`, http.StatusBadRequest)
}

func TestToolCallHandler_EmptyToolName(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":""}`, http.StatusNotImplemented)
}

func TestToolCallHandler_MethodNotPost(t *testing.T) {
	handler := toolCallHandler(testutil.NewMockDB(), slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/call", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d", rec.Code)
	}
}

func TestToolCallHandler_StoreSet_AuditWriteFails(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db error")
	callToolHandlerWithStore(t, mock, `{"tool_name":"x"}`, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceGet_NotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.get","arguments":{"scope_type":"system","key":"missing"}}`, http.StatusNotFound)
}

func TestToolCallHandler_PreferenceGet_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.get"}`, http.StatusBadRequest)
	callToolHandlerPOST(t, `{"tool_name":"db.preference.get","arguments":{"scope_type":"system"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceGet_ScopeIDRequired(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.get","arguments":{"scope_type":"user","key":"k"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceList_ScopeIDRequired(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.list","arguments":{"scope_type":"user"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceList_LimitZero(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.list","arguments":{"scope_type":"system","limit":0}}`, http.StatusOK)
}

func TestToolCallHandler_PreferenceList_Empty(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.list","arguments":{"scope_type":"system"}}`, http.StatusOK)
}

func TestToolCallHandler_PreferenceList_WithLimitAndCursor(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.list","arguments":{"scope_type":"system","limit":5,"cursor":"0"}}`, http.StatusOK)
}

func TestToolCallHandler_PreferenceEffective_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.effective"}`, http.StatusBadRequest)
}

func TestValidateRequiredScopedIds(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
		want     string
	}{
		{"effective missing task_id", "db.preference.effective", nil, "task_id required"},
		{"effective empty args", "db.preference.effective", map[string]interface{}{}, "task_id required"},
		{"effective invalid task_id", "db.preference.effective", map[string]interface{}{"task_id": "not-a-uuid"}, "task_id required"},
		{"effective has task_id", "db.preference.effective", map[string]interface{}{"task_id": uuid.New().String()}, ""},
		{"get no scoped ids required", "db.preference.get", map[string]interface{}{"scope_type": "system", "key": "k"}, ""},
		{"list no scoped ids required", "db.preference.list", map[string]interface{}{"scope_type": "system"}, ""},
		{"unknown tool", "other.tool", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateRequiredScopedIds(tt.toolName, tt.args)
			if got != tt.want {
				t.Errorf("validateRequiredScopedIds() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolCallHandler_PreferenceList_UserScope(t *testing.T) {
	mock := testutil.NewMockDB()
	uid := uuid.New()
	val := `"v"`
	mock.PreferenceEntries = append(mock.PreferenceEntries, &models.PreferenceEntry{
		ID: uuid.New(), ScopeType: "user", ScopeID: &uid, Key: "k", Value: &val, ValueType: "string", Version: 1, UpdatedAt: time.Now().UTC(),
	})
	handler := toolCallHandler(mock, slog.Default())
	body := `{"tool_name":"db.preference.list","arguments":{"scope_type":"user","scope_id":"` + uid.String() + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}

func TestToolCallHandler_PreferenceGet_Found(t *testing.T) {
	mock := testutil.NewMockDB()
	v := `"val"`
	mock.PreferenceEntries = append(mock.PreferenceEntries, &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		ScopeID:   nil,
		Key:       "a.key",
		Value:     &v,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	})
	handler := toolCallHandler(mock, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(`{"tool_name":"db.preference.get","arguments":{"scope_type":"system","key":"a.key"}}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}

func TestToolCallHandler_PreferenceEffective_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p")
	val := `"v"`
	mock.PreferenceEntries = append(mock.PreferenceEntries, &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		Key:       "x",
		Value:     &val,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	})
	handler := toolCallHandler(mock, slog.Default())
	body := `{"tool_name":"db.preference.effective","arguments":{"task_id":"` + task.ID.String() + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}

func TestToolCallHandler_PreferenceGet_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.GetPreferenceErr = errors.New("db error")
	callToolHandlerWithStore(t, mock, `{"tool_name":"db.preference.get","arguments":{"scope_type":"system","key":"k"}}`, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceList_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ListPreferencesErr = errors.New("db error")
	callToolHandlerWithStore(t, mock, `{"tool_name":"db.preference.list","arguments":{"scope_type":"system"}}`, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceEffective_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p")
	mock.GetEffectivePreferencesForTaskErr = errors.New("db error")
	body := `{"tool_name":"db.preference.effective","arguments":{"task_id":"` + task.ID.String() + `"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusInternalServerError)
}

// TestRun_DatabaseOpenFails covers run() when DATABASE_URL is set but Open fails.
func TestRun_DatabaseOpenFails(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", "postgres://invalid/invalid")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := run(ctx, slog.Default())
	if err == nil {
		t.Error("expected error when DATABASE_URL is invalid")
	}
}

// TestRun_TestDatabaseOpenReturnsError covers run() when testDatabaseOpen is set but returns an error.
func TestRun_TestDatabaseOpenReturnsError(t *testing.T) {
	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return nil, errors.New("open failed")
	}
	defer func() { testDatabaseOpen = nil }()
	_ = os.Setenv("DATABASE_URL", "postgres://local/test")
	defer func() { _ = os.Unsetenv("DATABASE_URL") }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := run(ctx, slog.Default())
	if err == nil {
		t.Error("expected error when testDatabaseOpen fails")
	}
}

// TestRun_WithTestDatabaseOpen covers the path where testDatabaseOpen is set (store from hook, no real DB).
func TestRun_WithTestDatabaseOpen(t *testing.T) {
	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return testutil.NewMockDB(), nil
	}
	defer func() { testDatabaseOpen = nil }()
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", "postgres://local/test")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.Default()) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}

// TestRun_WithTestStore starts run() with testStore set and POSTs to the tool-call endpoint to cover the store path.
func TestRun_WithTestStore(t *testing.T) {
	testStore = testutil.NewMockDB()
	defer func() { testStore = nil }()
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", "127.0.0.1:19083")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.Default()) }()

	time.Sleep(50 * time.Millisecond)
	// Use a non-routed tool so gateway returns 501.
	resp, err := http.Post("http://127.0.0.1:19083/v1/mcp/tools/call", "application/json", bytes.NewReader([]byte(`{"tool_name":"other.tool"}`)))
	if err != nil {
		cancel()
		<-done
		t.Skipf("POST failed (server may not be up): %v", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("POST: got status %d", resp.StatusCode)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}
