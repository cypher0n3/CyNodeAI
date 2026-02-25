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
	if err.Error() != "MCP gateway URL not set (SBA_MCP_GATEWAY_URL or MCP_GATEWAY_URL)" {
		t.Errorf("err = %q", err.Error())
	}
}
