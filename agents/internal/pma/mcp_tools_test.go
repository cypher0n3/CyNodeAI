package pma

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPTool_NameAndDescription(t *testing.T) {
	tool := NewMCPTool(&MCPClient{})
	if tool.Name() != "mcp_call" {
		t.Errorf("Name() = %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() empty")
	}
}
func TestMCPTool_Description_NoDbPrefix(t *testing.T) {
	tool := NewMCPTool(&MCPClient{BaseURL: "http://localhost"})
	if strings.Contains(tool.Description(), "db.") {
		t.Fatal("MCP tool description must not advertise db.* names")
	}
}

func TestMCPTool_Call_NoClient(t *testing.T) {
	tool := NewMCPTool(nil)
	out, err := tool.Call(context.Background(), `{"tool_name":"preference.get","arguments":{}}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out == "" || out != MsgMCPGatewayNotConfigured {
		t.Errorf("Call() = %q", out)
	}
}

func TestMCPTool_Call_EmptyBaseURL(t *testing.T) {
	tool := NewMCPTool(&MCPClient{BaseURL: ""})
	out, err := tool.Call(context.Background(), `{"tool_name":"preference.get","arguments":{}}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != MsgMCPGatewayNotConfigured {
		t.Errorf("Call() = %q", out)
	}
}

func TestMCPTool_Call_InvalidJSON(t *testing.T) {
	tool := NewMCPTool(&MCPClient{BaseURL: "http://localhost"})
	_, err := tool.Call(context.Background(), "not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMCPTool_Call_Non200ReturnsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"missing task_id"}`))
	}))
	defer server.Close()
	client := NewMCPClient()
	client.BaseURL = server.URL
	tool := NewMCPTool(client)
	out, err := tool.Call(context.Background(), `{"tool_name":"task.get","arguments":{}}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != `{"error":"missing task_id"}` {
		t.Errorf("Call() = %q", out)
	}
}

func TestMCPTool_Call_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"preference":"value"}`))
	}))
	defer server.Close()

	client := NewMCPClient()
	client.BaseURL = server.URL
	tool := NewMCPTool(client)
	out, err := tool.Call(context.Background(), `{"tool_name":"preference.get","arguments":{"key":"x"}}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != `{"preference":"value"}` {
		t.Errorf("Call() = %q", out)
	}
}
