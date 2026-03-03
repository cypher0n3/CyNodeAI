package pma

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewMCPClient_EnvPrecedence(t *testing.T) {
	const pmaURL = "http://pma-mcp"
	const genericURL = "http://generic-mcp"
	oldPma := os.Getenv(envPmaMcpGateway)
	oldGeneric := os.Getenv(envMcpGateway)
	defer func() {
		if oldPma != "" {
			_ = os.Setenv(envPmaMcpGateway, oldPma)
		} else {
			_ = os.Unsetenv(envPmaMcpGateway)
		}
		if oldGeneric != "" {
			_ = os.Setenv(envMcpGateway, oldGeneric)
		} else {
			_ = os.Unsetenv(envMcpGateway)
		}
	}()

	_ = os.Unsetenv(envPmaMcpGateway)
	_ = os.Unsetenv(envMcpGateway)
	c := NewMCPClient()
	if c.BaseURL != "" {
		t.Errorf("no env: BaseURL = %q", c.BaseURL)
	}

	_ = os.Setenv(envMcpGateway, genericURL)
	c = NewMCPClient()
	if c.BaseURL != genericURL {
		t.Errorf("MCP_GATEWAY_URL: BaseURL = %q", c.BaseURL)
	}

	_ = os.Setenv(envPmaMcpGateway, pmaURL)
	c = NewMCPClient()
	if c.BaseURL != pmaURL {
		t.Errorf("PMA_MCP_GATEWAY_URL overrides: BaseURL = %q", c.BaseURL)
	}
}

func TestMCPClient_Call_EmptyURL(t *testing.T) {
	c := &MCPClient{BaseURL: ""}
	_, _, err := c.Call(context.Background(), "db.preference.get", nil)
	if err == nil {
		t.Error("expected error for empty BaseURL")
	}
}

func TestMCPClient_Call_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != mcpToolsCallPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req MCPCallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":"ok"}`))
	}))
	defer server.Close()

	c := NewMCPClient()
	c.BaseURL = server.URL
	body, code, err := c.Call(context.Background(), "db.preference.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("code = %d", code)
	}
	if string(body) != `{"value":"ok"}` {
		t.Errorf("body = %s", body)
	}
}

func TestMCPClient_Call_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	}))
	defer server.Close()
	c := NewMCPClient()
	c.BaseURL = server.URL
	body, code, err := c.Call(context.Background(), "db.preference.get", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if code != http.StatusForbidden {
		t.Errorf("code = %d", code)
	}
	if string(body) != "denied" {
		t.Errorf("body = %s", body)
	}
}
