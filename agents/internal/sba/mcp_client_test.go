package sba

import (
	"context"
	"testing"
)

func TestMCPClient_Call_NoURL_ReturnsError(t *testing.T) {
	c := &MCPClient{}
	_, _, err := c.Call(context.Background(), "artifact.put", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "MCP tools base URL not set (PMA: PMA_MCP_GATEWAY_URL / MCP_GATEWAY_URL / MCP_GATEWAY_PROXY_URL / ORCHESTRATOR_MCP_TOOLS_BASE_URL / ORCHESTRATOR_MCP_GATEWAY_BASE_URL / ORCHESTRATOR_URL; SBA: SBA_MCP_GATEWAY_URL / MCP_GATEWAY_URL / ORCHESTRATOR_MCP_TOOLS_BASE_URL / ORCHESTRATOR_MCP_GATEWAY_BASE_URL / ORCHESTRATOR_URL)"
	if err.Error() != want {
		t.Errorf("err = %q", err.Error())
	}
}
