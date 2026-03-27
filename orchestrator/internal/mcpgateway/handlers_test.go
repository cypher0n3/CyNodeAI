package mcpgateway

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestIntArg_IntType(t *testing.T) {
	args := map[string]interface{}{"limit": 5}
	if got := intArg(args, "limit"); got != 5 {
		t.Errorf("intArg int: got %d", got)
	}
}

func TestIntArg_FloatType(t *testing.T) {
	args := map[string]interface{}{"limit": 10.0}
	if got := intArg(args, "limit"); got != 10 {
		t.Errorf("intArg float64: got %d", got)
	}
}

func TestIntArg_NilArgs(t *testing.T) {
	if got := intArg(nil, "limit"); got != 0 {
		t.Errorf("intArg nil args: got %d", got)
	}
}

func TestIntArg_UnsupportedType(t *testing.T) {
	args := map[string]interface{}{"limit": "ten"}
	if got := intArg(args, "limit"); got != 0 {
		t.Errorf("intArg unsupported type: got %d", got)
	}
}

func TestStrArg_NilArgs(t *testing.T) {
	if got := strArg(nil, "key"); got != "" {
		t.Errorf("strArg nil args: got %q", got)
	}
}
func TestToolCallHandler_StoreNil(t *testing.T) {
	handler := ToolCallHandler(nil, slog.Default(), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(`{"tool_name":"preference.get"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("store nil: got status %d", rec.Code)
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

func TestToolCallHandler_StoreSet_AuditWriteFails(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db error")
	callToolHandlerWithStore(t, mock, `{"tool_name":"x"}`, http.StatusInternalServerError)
}
func TestValidateRequiredScopedIds(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
		want     string
	}{
		{"effective missing task_id", "preference.effective", nil, "task_id required"},
		{"effective empty args", "preference.effective", map[string]interface{}{}, "task_id required"},
		{"effective invalid task_id", "preference.effective", map[string]interface{}{"task_id": "not-a-uuid"}, "task_id required"},
		{"effective has task_id", "preference.effective", map[string]interface{}{"task_id": uuid.New().String()}, ""},
		{"get no scoped ids required", "preference.get", map[string]interface{}{"scope_type": "system", "key": "k"}, ""},
		{"list no scoped ids required", "preference.list", map[string]interface{}{"scope_type": "system"}, ""},
		{"task.get missing task_id", "task.get", map[string]interface{}{}, "task_id required"},
		{"task.get has task_id", "task.get", map[string]interface{}{"task_id": uuid.New().String()}, ""},
		{"task.get alias missing task_id", "task.get", map[string]interface{}{}, "task_id required"},
		{"task.list missing user_id", "task.list", map[string]interface{}{}, "user_id required"},
		{"task.list has user_id", "task.list", map[string]interface{}{"user_id": uuid.New().String()}, ""},
		{"help.list no scoped ids", "help.list", map[string]interface{}{}, ""},
		{"help.get no scoped ids", "help.get", map[string]interface{}{}, ""},
		{"project.list missing user_id", "project.list", map[string]interface{}{}, "user_id required"},
		{"project.get missing user_id", "project.get", map[string]interface{}{"project_id": uuid.New().String()}, "user_id required"},
		{"job.get missing job_id", "job.get", map[string]interface{}{}, "job_id required"},
		{"job.get has job_id", "job.get", map[string]interface{}{"job_id": uuid.New().String()}, ""},
		{"artifact.get missing user_id", "artifact.get", map[string]interface{}{"path": "p", "scope": "user"}, "user_id required"},
		{"artifact.get has user_id", "artifact.get", map[string]interface{}{"user_id": uuid.New().String(), "path": "out/x", "scope": "user"}, ""},
		{"skills.create missing user_id", "skills.create", map[string]interface{}{"content": "x"}, "user_id required"},
		{"skills.create has user_id", "skills.create", map[string]interface{}{"user_id": uuid.New().String(), "content": "x"}, ""},
		{"skills.create extraneous task_id ignored", "skills.create", map[string]interface{}{"user_id": uuid.New().String(), "content": "x", "task_id": uuid.New().String()}, ""},
		{"unknown tool", "other.tool", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateRequiredScopedIds(tt.toolName, tt.args)
			if got != tt.want {
				t.Errorf("ValidateRequiredScopedIds() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateRequiredScopedIds_SkillsNeverRequireTaskID(t *testing.T) {
	uid := uuid.New().String()
	for _, name := range []string{
		"skills.create", "skills.list", "skills.get", "skills.update", "skills.delete",
	} {
		got := ValidateRequiredScopedIds(name, map[string]interface{}{"user_id": uid})
		if got == "task_id required" {
			t.Fatalf("%s: unexpected task_id required", name)
		}
	}
}

func TestToolCallHandler_LegacyDbToolName_ReturnsNotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.get","arguments":{"scope_type":"system","key":"k"}}`, http.StatusNotFound)
}

func TestToolCallHandler_LegacyDbTaskTool_ReturnsNotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.task.get","arguments":{"task_id":"00000000-0000-0000-0000-000000000000"}}`, http.StatusNotFound)
}

func TestToolCallHandler_LegacyDbTool_AuditWriteFails(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("audit fail")
	handler := ToolCallHandler(mock, slog.Default(), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(`{"tool_name":"db.x","arguments":{}}`))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestToolCallHandler_RouteAuditWriteFails(t *testing.T) {
	mock, taskID := mockWithTask(t)
	mock.ForceError = errors.New("audit write")
	body := `{"tool_name":"task.get","arguments":{"task_id":"` + taskID.String() + `"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusInternalServerError)
}

func TestToolCallHandler_MethodNotAllowed(t *testing.T) {
	handler := ToolCallHandler(testutil.NewMockDB(), slog.Default(), nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/call", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}
func TestToolCallHandler_BearerHelpListMatrix(t *testing.T) {
	cases := []struct {
		name   string
		bearer string
		want   int
	}{
		{"invalid_token", "Bearer wrong-token", http.StatusUnauthorized},
		{"pm_token", "Bearer pm-secret", http.StatusOK},
		{"sandbox_token", "Bearer sand-secret", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := testutil.NewMockDB()
			auth := &ToolCallAuth{PMToken: "pm-secret", SandboxToken: "sand-secret"}
			handler := ToolCallHandler(mock, slog.Default(), auth)
			req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(jsonMCPBodyHelpList))
			req.Header.Set("Authorization", tc.bearer)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestToolCallHandler_SandboxBearerDenied_AuditWriteFails500(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("audit unavailable")
	auth := &ToolCallAuth{PMToken: "pm-secret", SandboxToken: "sand-secret"}
	handler := ToolCallHandler(mock, slog.Default(), auth)
	body := `{"tool_name":"task.get","arguments":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer sand-secret")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
	}
}

func TestToolCallHandler_SandboxBearerDeniedTaskTool(t *testing.T) {
	mock := testutil.NewMockDB()
	auth := &ToolCallAuth{PMToken: "pm-secret", SandboxToken: "sand-secret"}
	handler := ToolCallHandler(mock, slog.Default(), auth)
	body := `{"tool_name":"task.get","arguments":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer sand-secret")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
	}
}
