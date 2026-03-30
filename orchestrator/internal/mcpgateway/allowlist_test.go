package mcpgateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestParseBearerToken(t *testing.T) {
	t.Parallel()
	if got := parseBearerToken("Bearer abc"); got != "abc" {
		t.Errorf("got %q", got)
	}
	if got := parseBearerToken("bearer abc"); got != "abc" {
		t.Errorf("case-insensitive: got %q", got)
	}
	if got := parseBearerToken(""); got != "" {
		t.Errorf("empty: got %q", got)
	}
	if got := parseBearerToken("Basic x"); got != "" {
		t.Errorf("wrong scheme: got %q", got)
	}
}

func TestToolCallAuth_resolveRole(t *testing.T) {
	t.Parallel()
	a := &ToolCallAuth{PMToken: "pm", SandboxToken: "sand", PAToken: "pa"}
	role, ok := a.resolveRole("pm")
	if !ok || role != AgentRolePM {
		t.Fatalf("pm: ok=%v role=%v", ok, role)
	}
	role, ok = a.resolveRole("sand")
	if !ok || role != AgentRoleSandbox {
		t.Fatalf("sand: ok=%v role=%v", ok, role)
	}
	role, ok = a.resolveRole("pa")
	if !ok || role != AgentRolePA {
		t.Fatalf("pa: ok=%v role=%v", ok, role)
	}
	_, ok = a.resolveRole("nope")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestSandboxAllowsTool(t *testing.T) {
	t.Parallel()
	if !sandboxAllowsTool("help.list") {
		t.Fatal("help.list")
	}
	if sandboxAllowsTool("task.get") {
		t.Fatal("task.get should be denied for sandbox")
	}
}

func TestToolCallAuth_anyTokenConfigured(t *testing.T) {
	t.Parallel()
	if (*ToolCallAuth)(nil).anyTokenConfigured() {
		t.Fatal("nil auth")
	}
	if (&ToolCallAuth{}).anyTokenConfigured() {
		t.Fatal("empty tokens")
	}
	if !(&ToolCallAuth{PMToken: "x"}).anyTokenConfigured() {
		t.Fatal("pm token")
	}
	if !(&ToolCallAuth{SandboxToken: "y"}).anyTokenConfigured() {
		t.Fatal("sandbox token")
	}
	if !(&ToolCallAuth{PAToken: "z"}).anyTokenConfigured() {
		t.Fatal("pa token")
	}
}

func TestToolCallAuth_resolveRole_nilReceiver(t *testing.T) {
	t.Parallel()
	var a *ToolCallAuth
	role, ok := a.resolveRole("x")
	if ok || role != AgentRoleNone {
		t.Fatalf("nil auth resolveRole: ok=%v role=%v", ok, role)
	}
}

func TestPmAllowsTool(t *testing.T) {
	t.Parallel()
	if !pmAllowsTool("task.get") || !pmAllowsTool("node.list") {
		t.Fatal("pm should allow task and node tools")
	}
	if pmAllowsTool("zzz.unknown") {
		t.Fatal("unknown namespace")
	}
}

func TestPaAllowsTool(t *testing.T) {
	t.Parallel()
	if !paAllowsTool("task.get") || !paAllowsTool("skills.list") {
		t.Fatal("pa should allow task and skills read")
	}
	if paAllowsTool("node.list") || paAllowsTool("system_setting.get") {
		t.Fatal("pa must not allow node or system_setting tools")
	}
	if paAllowsTool("task.cancel") {
		t.Fatal("task.cancel denied for PA")
	}
}

func TestAllowlist(t *testing.T) {
	mock := testutil.NewMockDB()
	auth := &ToolCallAuth{PMToken: "pm", SandboxToken: "sand", PAToken: "pa"}
	h := ToolCallHandler(mock, slog.Default(), auth)
	t.Run("no_bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(`{"tool_name":"help.list","arguments":{}}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code %d", rr.Code)
		}
	})
	t.Run("pm_denied_sandbox_only_tool_not_in_pm", func(t *testing.T) {
		// Not a routed tool name; PM allowlist rejects before 501.
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(`{"tool_name":"zzz.unknown","arguments":{}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer pm")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
		}
	})
	t.Run("pa_denied_node", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(`{"tool_name":"node.list","arguments":{}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer pa")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("code %d", rr.Code)
		}
	})
	t.Run("pa_ok_help_list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(`{"tool_name":"help.list","arguments":{}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer pa")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnauthorized {
			t.Fatalf("code %d", rr.Code)
		}
	})
}
