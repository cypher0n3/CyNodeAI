package pma

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/agents/internal/mcpclient"
)

// mockMCPResponseValueOK is the JSON body returned by mock MCP success handlers (goconst).
const mockMCPResponseValueOK = `{"value":"ok"}`

const (
	envPmaMcpGateway   = "PMA_MCP_GATEWAY_URL"
	envMcpGateway      = "MCP_GATEWAY_URL"
	envMcpGatewayProxy = "MCP_GATEWAY_PROXY_URL"
)

func TestNewMCPClient_EnvPrecedence(t *testing.T) {
	const pmaURL = "http://pma-mcp"
	const genericURL = "http://generic-mcp"
	const proxyURL = "http://proxy-mcp"
	oldPma := os.Getenv(envPmaMcpGateway)
	oldGeneric := os.Getenv(envMcpGateway)
	oldProxy := os.Getenv(envMcpGatewayProxy)
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
		if oldProxy != "" {
			_ = os.Setenv(envMcpGatewayProxy, oldProxy)
		} else {
			_ = os.Unsetenv(envMcpGatewayProxy)
		}
	}()

	_ = os.Unsetenv(envPmaMcpGateway)
	_ = os.Unsetenv(envMcpGateway)
	_ = os.Unsetenv(envMcpGatewayProxy)
	c := NewMCPClient()
	if c.BaseURL != "" {
		t.Errorf("no env: BaseURL = %q", c.BaseURL)
	}

	_ = os.Setenv(envMcpGatewayProxy, proxyURL)
	c = NewMCPClient()
	if c.BaseURL != proxyURL {
		t.Errorf("MCP_GATEWAY_PROXY_URL: BaseURL = %q", c.BaseURL)
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
		if r.Method != http.MethodPost || r.URL.Path != mcpclient.ToolsCallPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req mcpclient.CallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockMCPResponseValueOK))
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
	if string(body) != mockMCPResponseValueOK {
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

func TestMCPClient_Call_ViaWorkerInternalProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != mcpclient.ProxyCallPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req mcpclient.ManagedServiceProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Path != mcpclient.ToolsCallPath {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":1,"status":200,"body_b64":"eyJ2YWx1ZSI6Im9rIn0="}`))
	}))
	defer server.Close()

	c := NewMCPClient()
	c.BaseURL = server.URL + mcpclient.ProxyCallPath
	body, code, err := c.Call(context.Background(), "db.preference.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Call via proxy: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("code = %d", code)
	}
	if string(body) != mockMCPResponseValueOK {
		t.Errorf("body = %s", body)
	}
}

func TestMCPClient_Call_ViaWorkerInternalProxy_NonOK(t *testing.T) {
	server := newMockServerWithResponse(t, http.StatusBadGateway, "application/problem+json", `{"detail":"upstream failed"}`)
	defer server.Close()

	c := NewMCPClient()
	c.BaseURL = server.URL + mcpclient.ProxyCallPath
	body, code, err := c.Call(context.Background(), "db.preference.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Call via proxy: %v", err)
	}
	if code != http.StatusBadGateway {
		t.Errorf("code = %d", code)
	}
	if string(body) != `{"detail":"upstream failed"}` {
		t.Errorf("body = %s", body)
	}
}

// newMockServerWithResponse starts an httptest.Server that responds with the given status, Content-Type, and body.
func newMockServerWithResponse(t *testing.T, status int, contentType, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestMCPClient_Call_ViaWorkerInternalProxy_HTTPUnixSuccess(t *testing.T) {
	tmp := t.TempDir()
	socketPath := filepath.Join(tmp, "proxy.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer func() { _ = listener.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+mcpclient.ProxyCallPath, func(w http.ResponseWriter, r *http.Request) {
		var req mcpclient.ManagedServiceProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Path != mcpclient.ToolsCallPath {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":1,"status":200,"body_b64":"eyJ2YWx1ZSI6Im9rIn0="}`))
	})
	server := &http.Server{Handler: mux}
	defer func() { _ = server.Close() }()
	go func() { _ = server.Serve(listener) }()

	c := NewMCPClient()
	c.BaseURL = "http+unix://" + url.PathEscape(socketPath) + mcpclient.ProxyCallPath
	c.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	body, code, err := c.Call(context.Background(), "db.preference.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Call via http+unix proxy: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("code = %d", code)
	}
	if string(body) != mockMCPResponseValueOK {
		t.Errorf("body = %s", body)
	}
}

func TestParseHTTPUnixEndpoint(t *testing.T) {
	socketPath := "/tmp/cynode/proxy.sock"
	raw := "http+unix://" + url.PathEscape(socketPath) + mcpclient.ProxyCallPath
	gotSocket, gotPath, ok := mcpclient.ParseHTTPUnixEndpoint(raw)
	if !ok {
		t.Fatal("expected parse success")
	}
	if gotSocket != socketPath {
		t.Fatalf("socket = %q", gotSocket)
	}
	if gotPath != mcpclient.ProxyCallPath {
		t.Fatalf("path = %q", gotPath)
	}
	if _, _, ok := mcpclient.ParseHTTPUnixEndpoint("http://127.0.0.1:12090" + mcpclient.ProxyCallPath); ok {
		t.Fatal("expected parse failure for non-http+unix URL")
	}
	if _, _, ok := mcpclient.ParseHTTPUnixEndpoint("http+unix://only-socket-no-path"); ok {
		t.Fatal("expected parse failure for missing endpoint path")
	}
	if _, _, ok := mcpclient.ParseHTTPUnixEndpoint("http+unix://%zz" + mcpclient.ProxyCallPath); ok {
		t.Fatal("expected parse failure for invalid escaped socket path")
	}
}

func TestMCPClient_Call_ViaWorkerInternalProxy_InvalidProxyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":1,"status":200,"body_b64":"%%%not-b64%%%"}`))
	}))
	defer server.Close()
	c := NewMCPClient()
	c.BaseURL = server.URL + mcpclient.ProxyCallPath
	_, _, err := c.Call(context.Background(), "db.preference.get", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected base64 decode error")
	}
}

func TestMCPClient_Call_ViaWorkerInternalProxy_InvalidHTTPUnixURL(t *testing.T) {
	c := NewMCPClient()
	c.BaseURL = "http+unix://bad" + mcpclient.ProxyCallPath
	_, _, err := c.Call(context.Background(), "db.preference.get", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected invalid http+unix URL error")
	}
}
