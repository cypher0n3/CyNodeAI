package mcpclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestClient_Call_DirectHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ToolsCallPath {
			t.Errorf("path %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var body CallRequest
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatal(err)
		}
		if body.ToolName != "task.get" {
			t.Errorf("tool %s", body.ToolName)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	body, code, err := c.Call(context.Background(), "task.get", map[string]interface{}{"task_id": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("body %s", body)
	}
}

func TestNewPMClient_EmptyURL(t *testing.T) {
	t.Setenv(envPmaMcpGateway, "")
	t.Setenv(envMcpGateway, "")
	t.Setenv(envMcpGatewayProxy, "")
	c := NewPMClient()
	if c.BaseURL != "" {
		t.Errorf("expected empty base URL")
	}
}

func TestNewPMClient_Precedence(t *testing.T) {
	t.Setenv(envPmaMcpGateway, "http://pma")
	t.Setenv(envMcpGateway, "http://gw")
	t.Setenv(envMcpGatewayProxy, "http://proxy")
	c := NewPMClient()
	if c.BaseURL != "http://pma" {
		t.Errorf("want PMA first: %q", c.BaseURL)
	}
}

func TestNewSBAClient_PrefersSBAEnv(t *testing.T) {
	t.Setenv(envSbaMcpGateway, "http://sba")
	t.Setenv(envMcpGateway, "http://gw")
	c := NewSBAClient()
	if c.BaseURL != "http://sba" {
		t.Errorf("want SBA first: %q", c.BaseURL)
	}
}

func TestClient_Call_ViaProxyUnixSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "mcp.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != ProxyCallPath {
				t.Errorf("path %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			var env ManagedServiceProxyRequest
			if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
				t.Fatal(err)
			}
			resp := struct {
				Version int    `json:"version"`
				Status  int    `json:"status"`
				BodyB64 string `json:"body_b64,omitempty"`
			}{Version: 1, Status: http.StatusOK, BodyB64: base64.StdEncoding.EncodeToString([]byte(`{"unix":true}`))}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = ln.Close()
	})

	rawURL := "http+unix://" + url.PathEscape(sockPath) + ProxyCallPath
	c := &Client{BaseURL: rawURL, HTTPClient: http.DefaultClient}
	body, code, err := c.Call(context.Background(), "task.get", map[string]interface{}{"task_id": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK || string(body) != `{"unix":true}` {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestClient_Call_ViaProxyHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ProxyCallPath {
			t.Errorf("path %s", r.URL.Path)
		}
		var env ManagedServiceProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		if env.Path != ToolsCallPath || env.Method != http.MethodPost {
			t.Errorf("env %+v", env)
		}
		raw, _ := base64.StdEncoding.DecodeString(env.BodyB64)
		var inner CallRequest
		if err := json.Unmarshal(raw, &inner); err != nil || inner.ToolName != "task.get" {
			t.Fatalf("inner %s err=%v", raw, err)
		}
		resp := struct {
			Version int    `json:"version"`
			Status  int    `json:"status"`
			BodyB64 string `json:"body_b64,omitempty"`
		}{Version: 1, Status: http.StatusOK, BodyB64: base64.StdEncoding.EncodeToString([]byte(`{"ok":1}`))}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL + ProxyCallPath, HTTPClient: srv.Client()}
	body, code, err := c.Call(context.Background(), "task.get", map[string]interface{}{"task_id": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	if string(body) != `{"ok":1}` {
		t.Fatalf("body %s", body)
	}
}

func TestClient_Call_ProxyBadJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL + ProxyCallPath, HTTPClient: srv.Client()}
	_, _, err := c.Call(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Call_ProxyBadBase64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Version int    `json:"version"`
			Status  int    `json:"status"`
			BodyB64 string `json:"body_b64,omitempty"`
		}{Version: 1, Status: http.StatusOK, BodyB64: "!!!"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL + ProxyCallPath, HTTPClient: srv.Client()}
	_, _, err := c.Call(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Call_ProxyNonOKHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`err`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL + ProxyCallPath, HTTPClient: srv.Client()}
	body, code, err := c.Call(context.Background(), "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusBadGateway || string(body) != `err` {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestClient_Call_InvalidUnixProxyURL(t *testing.T) {
	c := &Client{
		BaseURL:    "http+unix://%zz" + ProxyCallPath,
		HTTPClient: http.DefaultClient,
	}
	_, _, err := c.Call(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseHTTPUnixEndpoint(t *testing.T) {
	sock := "/tmp/s"
	path := ProxyCallPath
	raw := "http+unix://" + url.PathEscape(sock) + path
	gotSock, gotPath, ok := ParseHTTPUnixEndpoint(raw)
	if !ok || gotSock != sock || gotPath != path {
		t.Errorf("ok=%v sock=%q path=%q", ok, gotSock, gotPath)
	}
	if _, _, ok := ParseHTTPUnixEndpoint("http://127.0.0.1:1" + ToolsCallPath); ok {
		t.Error("expected false for http URL")
	}
	if _, _, ok := ParseHTTPUnixEndpoint("http+unix://nopath"); ok {
		t.Error("expected false without path")
	}
	if _, _, ok := ParseHTTPUnixEndpoint("http+unix://%zz/x"); ok {
		t.Error("expected false for bad escape")
	}
}

func TestLangchainTool_NameDescriptionAndSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ToolsCallPath {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	lt := NewLangchainTool(&Client{BaseURL: srv.URL, HTTPClient: srv.Client()}, "desc", "")
	if lt.Name() != "mcp_call" || lt.Description() != "desc" {
		t.Errorf("name/desc")
	}
	out, err := lt.Call(context.Background(), `{"tool_name":"task.get","arguments" : {"task_id":"x"}}`)
	if err != nil || out != `{"ok":true}` {
		t.Fatalf("success: out=%q err=%v", out, err)
	}
}

func TestNewLangchainTool_DefaultNotConfiguredMsg(t *testing.T) {
	lt := NewLangchainTool(&Client{}, "d", "")
	out, err := lt.Call(context.Background(), `{"tool_name":"x"}`)
	if err != nil || out != "MCP gateway not configured" {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestClient_Call_DirectHTTP_DoError(t *testing.T) {
	c := &Client{
		BaseURL:    "http://127.0.0.1:1",
		HTTPClient: &http.Client{Timeout: 20 * time.Millisecond},
	}
	_, _, err := c.Call(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLangchainTool_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`oops`))
	}))
	defer srv.Close()
	lt := NewLangchainTool(&Client{BaseURL: srv.URL, HTTPClient: srv.Client()}, "d", "")
	out, err := lt.Call(context.Background(), `{"tool_name":"x"}`)
	if err != nil || out != `oops` {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestLangchainTool_Call(t *testing.T) {
	ltEmpty := NewLangchainTool(&Client{}, "d", "")
	if out, err := ltEmpty.Call(context.Background(), "not-json"); err != nil || out == "" {
		t.Errorf("unconfigured: out=%q err=%v", out, err)
	}
	lt := NewLangchainTool(&Client{BaseURL: "http://x"}, "d", "")
	if _, err := lt.Call(context.Background(), "not-json"); err == nil {
		t.Error("expected err for invalid JSON")
	}
	if _, err := lt.Call(context.Background(), `{"arguments":{}}`); err == nil {
		t.Error("expected error for empty tool_name")
	}
	ltNil := NewLangchainTool(nil, "d", "nc")
	if out, err := ltNil.Call(context.Background(), `{}`); err != nil || out != "nc" {
		t.Errorf("nil client: %q %v", out, err)
	}
}
