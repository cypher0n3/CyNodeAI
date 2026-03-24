package mcpgateway

import (
	"testing"
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
	a := &ToolCallAuth{PMToken: "pm", SandboxToken: "sand"}
	role, ok := a.resolveRole("pm")
	if !ok || role != AgentRolePM {
		t.Fatalf("pm: ok=%v role=%v", ok, role)
	}
	role, ok = a.resolveRole("sand")
	if !ok || role != AgentRoleSandbox {
		t.Fatalf("sand: ok=%v role=%v", ok, role)
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
