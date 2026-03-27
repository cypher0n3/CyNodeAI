package mcpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const jsonMCPBodyHelpList = `{"tool_name":"help.list","arguments":{}}`

// mcpHandlerToolCase drives table-driven MCP handler tests (store error -> HTTP 500).
type mcpHandlerToolCase struct {
	name string
	prep func(*testutil.MockDB)
	call func(context.Context, database.Store, map[string]interface{}, *models.McpToolCallAuditLog) (int, []byte, *models.McpToolCallAuditLog)
}

func newTestMockWithTask(t *testing.T) (context.Context, *testutil.MockDB, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(ctx, "mcp-task-"+strings.ReplaceAll(t.Name(), "/", "-"), nil)
	proj, _ := mock.GetOrCreateDefaultProjectForUser(ctx, user.ID)
	uid := user.ID
	task, _ := mock.CreateTask(ctx, &uid, "prompt", nil, &proj.ID)
	return ctx, mock, task.ID
}

func testMCPHandlerSimpleStoreError(t *testing.T, prep func(*testutil.MockDB), call func(context.Context, database.Store, map[string]interface{}, *models.McpToolCallAuditLog) (int, []byte, *models.McpToolCallAuditLog), args map[string]interface{}) {
	t.Helper()
	ctx := context.Background()
	mock := testutil.NewMockDB()
	prep(mock)
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := call(ctx, mock, args, rec)
	if code != http.StatusInternalServerError {
		t.Fatalf("got %d", code)
	}
}

func testMCPHandleTaskWithPrep(t *testing.T, prep func(*testutil.MockDB), call func(context.Context, database.Store, map[string]interface{}, *models.McpToolCallAuditLog) (int, []byte, *models.McpToolCallAuditLog)) {
	t.Helper()
	ctx, mock, taskID := newTestMockWithTask(t)
	prep(mock)
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := call(ctx, mock, map[string]interface{}{"task_id": taskID.String()}, rec)
	if code != http.StatusInternalServerError {
		t.Fatalf("got %d", code)
	}
}

func assertToolCallStoreOKHasKey(t *testing.T, mock *testutil.MockDB, body, jsonKey string) {
	t.Helper()
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d %s", code, respBody)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out[jsonKey] == nil {
		t.Errorf("expected %s in response", jsonKey)
	}
}

// callToolHandlerWithStore sends a POST with the given store and body, and asserts the response code.
func callToolHandlerWithStore(t *testing.T, store database.Store, body string, wantCode int) {
	t.Helper()
	code, _ := callToolHandlerWithStoreAndBody(t, store, body)
	if code != wantCode {
		t.Errorf("got status %d, want %d", code, wantCode)
	}
}

const testPreferenceVal = `"v"`

// mockWithSystemPreference returns a mock DB with one system-scope preference entry for key.
func mockWithSystemPreference(t *testing.T, key string) *testutil.MockDB {
	t.Helper()
	mock := testutil.NewMockDB()
	v := testPreferenceVal
	mock.PreferenceEntries = append(mock.PreferenceEntries, &models.PreferenceEntry{
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "system",
			Key:       key,
			Value:     &v,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
		UpdatedAt: time.Now().UTC(),
	})
	return mock
}

// mockWithTask returns a mock DB with one task; callers set error fields and build tool body.
func mockWithTask(t *testing.T) (*testutil.MockDB, uuid.UUID) {
	t.Helper()
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	return mock, task.ID
}

// callToolHandlerWithStoreAndBody sends a POST and returns status code and response body.
func callToolHandlerWithStoreAndBody(t *testing.T, store database.Store, body string) (statusCode int, bodyBytes []byte) {
	t.Helper()
	handler := ToolCallHandler(store, slog.Default(), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// callToolHandlerPOST sends a POST with body and asserts the response code.
func callToolHandlerPOST(t *testing.T, body string, wantCode int) {
	t.Helper()
	handler := ToolCallHandler(testutil.NewMockDB(), slog.Default(), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != wantCode {
		t.Errorf("got status %d, want %d", rec.Code, wantCode)
	}
}
func mockDBWithUserTaskAndSkill(t *testing.T) (*testutil.MockDB, *models.User, *models.Skill) {
	t.Helper()
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	_, _ = mock.CreateTask(context.Background(), &user.ID, "p", nil)
	skill, _ := mock.CreateSkill(context.Background(), "s1", "# c", "user", &user.ID, false)
	return mock, user, skill
}
