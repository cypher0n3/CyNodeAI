package sba

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPTool_NameAndDescription(t *testing.T) {
	tool := NewMCPTool(&MCPClient{})
	if tool.Name() != "mcp_call" {
		t.Errorf("Name = %q", tool.Name())
	}
	if tool.Description() == "" || !strings.Contains(tool.Description(), "artifact.put") {
		t.Errorf("Description missing expected content")
	}
}
func TestMCPTool_Description_NoDbPrefix(t *testing.T) {
	tool := NewMCPTool(&MCPClient{BaseURL: "http://localhost"})
	if strings.Contains(tool.Description(), "db.") {
		t.Fatal("MCP tool description must not advertise db.* names")
	}
}

func TestMCPTool_NoURL_ReturnsMessage(t *testing.T) {
	tool := NewMCPTool(&MCPClient{})
	out, err := tool.Call(context.Background(), `{"tool_name": "artifact.put", "arguments": {}}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "MCP gateway not configured (set SBA_MCP_GATEWAY_URL or MCP_GATEWAY_URL)" {
		t.Errorf("out = %q", out)
	}
}

func TestMCPTool_InvalidInput_ReturnsError(t *testing.T) {
	tool := NewMCPTool(&MCPClient{BaseURL: "http://localhost"})
	_, err := tool.Call(context.Background(), "not json")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMCPTool_EmptyToolName_ReturnsError(t *testing.T) {
	tool := NewMCPTool(&MCPClient{BaseURL: "http://localhost"})
	_, err := tool.Call(context.Background(), `{"arguments": {}}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtNetRequiredError(t *testing.T) {
	var e *extNetRequiredError
	if !errors.As(ErrExtNetRequired, &e) {
		t.Error("errors.As failed")
	}
	if ErrExtNetRequired.Error() != "network required but denied" {
		t.Errorf("Error() = %q", ErrExtNetRequired.Error())
	}
}

func TestMCPTool_403_ReturnsErrExtNetRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	client := NewMCPClient()
	client.BaseURL = srv.URL
	tool := NewMCPTool(client)
	_, err := tool.Call(context.Background(), `{"tool_name": "web.fetch", "arguments": {}}`)
	if err != ErrExtNetRequired {
		t.Errorf("err = %v", err)
	}
}

func TestMCPTool_200_ReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": "ok"}`))
	}))
	defer srv.Close()
	client := NewMCPClient()
	client.BaseURL = srv.URL
	tool := NewMCPTool(client)
	out, err := tool.Call(context.Background(), `{"tool_name": "artifact.put", "arguments": {"key": "x"}}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != `{"result": "ok"}` {
		t.Errorf("out = %q", out)
	}
}
