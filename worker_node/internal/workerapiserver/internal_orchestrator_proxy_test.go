package workerapiserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
)

func TestDeriveMCPToolsBaseURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"not-a-url", "not-a-url"},
		{"http://localhost:12082", "http://localhost:12082"},
		{"http://127.0.0.1:12082/", "http://127.0.0.1:12082"},
		{"http://cynodeai-control-plane:12082", "http://cynodeai-control-plane:12082"},
		{"http://example.com:9999", "http://example.com:9999"},
	}
	for _, c := range cases {
		got := deriveMCPToolsBaseURL(c.in)
		if got != c.want {
			t.Errorf("deriveMCPToolsBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHandleInternalOrchestratorMCPCall_RequiresServiceIdentity(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(`{}`))
	handleInternalOrchestratorMCPCall(rec, req, embedInternalProxyConfig{
		MCPToolsBaseURL: "http://127.0.0.1:12082",
		SecureStore:     nil,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandleInternalOrchestratorMCPCall_WithServiceID_NoToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(`{}`))
	ctx := context.WithValue(req.Context(), CallerServiceIDContextKey, "svc-1")
	req = req.WithContext(ctx)
	handleInternalOrchestratorMCPCall(rec, req, embedInternalProxyConfig{
		MCPToolsBaseURL: "http://127.0.0.1:12082",
		SecureStore:     nil,
	})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterInternalOrchestratorProxyHandlers_NilMuxNoPanic(t *testing.T) {
	registerInternalOrchestratorProxyHandlers(nil, embedInternalProxyConfig{})
}

func TestRegisterInternalOrchestratorProxyHandlers_Routes(t *testing.T) {
	mux := http.NewServeMux()
	registerInternalOrchestratorProxyHandlers(mux, embedInternalProxyConfig{})
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatal("expected handler registered for mcp:call")
	}
	var pd struct {
		Status int `json:"status"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pd)
}

func testSecureStore(t *testing.T) *securestore.Store {
	t.Helper()
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	store, _, err := securestore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return store
}

func TestDecodeManagedProxyRequest_InvalidJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not-json`))
	_, _, _, ok := decodeManagedProxyRequest(rec, req, "/def")
	if ok || rec.Code != http.StatusBadRequest {
		t.Fatalf("ok=%v code=%d body=%s", ok, rec.Code, rec.Body.String())
	}
}

func TestDecodeManagedProxyRequest_InvalidBodyB64(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"path":"/x","body_b64":"@@@"}`))
	_, _, _, ok := decodeManagedProxyRequest(rec, req, "/def")
	if ok || rec.Code != http.StatusBadRequest {
		t.Fatalf("ok=%v code=%d", ok, rec.Code)
	}
}

func TestDecodeManagedProxyRequest_PathResolution(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{}`))
	tests := []struct {
		name        string
		body        string
		defaultPath string
		wantPath    string
	}{
		{
			name:        "usesDefault",
			body:        `{"path":"","body_b64":"` + payload + `"}`,
			defaultPath: "/default",
			wantPath:    "/default",
		},
		{
			name:        "addsLeadingSlash",
			body:        `{"path":"v1/ping","body_b64":"` + payload + `"}`,
			defaultPath: "",
			wantPath:    "/v1/ping",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			_, _, path, ok := decodeManagedProxyRequest(rec, req, tt.defaultPath)
			if !ok || path != tt.wantPath {
				t.Fatalf("path=%q want %q ok=%v", path, tt.wantPath, ok)
			}
		})
	}
}

func TestDecodeManagedProxyRequest_EmptyPathNoDefault(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{}`))
	body := `{"path":"","body_b64":"` + payload + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	_, _, _, ok := decodeManagedProxyRequest(rec, req, "")
	if ok || rec.Code != http.StatusBadRequest {
		t.Fatalf("expected path required: ok=%v code=%d", ok, rec.Code)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read") }

func TestWriteManagedProxyJSONFromUpstream_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Test": []string{"1"}},
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	if !writeManagedProxyJSONFromUpstream(rec, resp) {
		t.Fatal("expected true")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestWriteManagedProxyJSONFromUpstream_ReadError(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(errReader{}),
	}
	if writeManagedProxyJSONFromUpstream(rec, resp) {
		t.Fatal("expected false")
	}
}

func TestGetAgentTokenForProxy_OK(t *testing.T) {
	store := testSecureStore(t)
	if err := store.PutAgentToken("svc-ok", "secret-tok", ""); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	cfg := embedInternalProxyConfig{SecureStore: store}
	got, ok := getAgentTokenForProxy(cfg, "svc-ok", rec)
	if !ok || got == nil || got.Token != "secret-tok" {
		t.Fatalf("got=%v ok=%v", got, ok)
	}
}

func TestGetAgentTokenForProxy_Expired(t *testing.T) {
	store := testSecureStore(t)
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if err := store.PutAgentToken("svc-exp", "tok", past); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	cfg := embedInternalProxyConfig{SecureStore: store}
	_, ok := getAgentTokenForProxy(cfg, "svc-exp", rec)
	if ok || rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected expired: ok=%v code=%d", ok, rec.Code)
	}
}

func TestHandleInternalOrchestratorMCPCall_ForwardSuccess(t *testing.T) {
	store := testSecureStore(t)
	if err := store.PutAgentToken("svc-fwd", "bearer-tok", ""); err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != toolsCallPath {
			t.Errorf("path %s", r.URL.Path)
		}
		if ah := r.Header.Get("Authorization"); ah != "Bearer bearer-tok" {
			t.Errorf("Authorization: %q", ah)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tool":"ok"}`))
	}))
	t.Cleanup(up.Close)

	oldClient := internalOrchestratorHTTPClient
	internalOrchestratorHTTPClient = up.Client()
	t.Cleanup(func() { internalOrchestratorHTTPClient = oldClient })

	payload := base64.StdEncoding.EncodeToString([]byte(`{}`))
	body := `{"version":1,"method":"POST","path":"","headers":{"Content-Type":["application/json"]},"body_b64":"` + payload + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), CallerServiceIDContextKey, "svc-fwd")
	req = req.WithContext(ctx)

	handleInternalOrchestratorMCPCall(rec, req, embedInternalProxyConfig{
		MCPToolsBaseURL: up.URL,
		SecureStore:     store,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

type roundTripErr struct{}

func (roundTripErr) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("upstream unreachable")
}

func TestHandleInternalOrchestratorMCPCall_UpstreamDoError(t *testing.T) {
	store := testSecureStore(t)
	if err := store.PutAgentToken("svc-doerr", "tok", ""); err != nil {
		t.Fatal(err)
	}
	old := internalOrchestratorHTTPClient
	internalOrchestratorHTTPClient = &http.Client{Transport: roundTripErr{}}
	t.Cleanup(func() { internalOrchestratorHTTPClient = old })

	payload := base64.StdEncoding.EncodeToString([]byte(`{}`))
	body := `{"version":1,"method":"POST","path":"","headers":{"Content-Type":["application/json"]},"body_b64":"` + payload + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), CallerServiceIDContextKey, "svc-doerr")
	req = req.WithContext(ctx)

	handleInternalOrchestratorMCPCall(rec, req, embedInternalProxyConfig{
		MCPToolsBaseURL: "http://127.0.0.1:1",
		SecureStore:     store,
	})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleInternalOrchestratorAgentReady_MethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/internal/orchestrator/agent:ready", http.NoBody)
	handleInternalOrchestratorAgentReady(rec, req, embedInternalProxyConfig{UpstreamBaseURL: "http://x"})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHandleInternalOrchestratorAgentReady_ForwardSuccess(t *testing.T) {
	store := testSecureStore(t)
	if err := store.PutAgentToken("svc-ready", "tok2", ""); err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agents/ready" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ready":true}`))
	}))
	t.Cleanup(up.Close)

	oldClient := internalOrchestratorHTTPClient
	internalOrchestratorHTTPClient = up.Client()
	t.Cleanup(func() { internalOrchestratorHTTPClient = oldClient })

	payload := base64.StdEncoding.EncodeToString([]byte(`{}`))
	body := `{"version":1,"method":"GET","path":"/v1/agents/ready","body_b64":"` + payload + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/agent:ready", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), CallerServiceIDContextKey, "svc-ready")
	req = req.WithContext(ctx)

	handleInternalOrchestratorAgentReady(rec, req, embedInternalProxyConfig{
		UpstreamBaseURL: up.URL,
		SecureStore:     store,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func newMCPCallRoundTripUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != toolsCallPath {
			t.Errorf("upstream path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok-uds" {
			t.Errorf("Authorization: %q", r.Header.Get("Authorization"))
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if !bytes.Contains(b, []byte("help.list")) {
			t.Errorf("expected help.list in upstream body, got %s", b)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"routed":true}`))
	}))
}

func unixSocketHTTPClient(udsPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", udsPath)
			},
		},
	}
}

func startUDSInternalOrchestratorTestServer(t *testing.T, store *securestore.Store, mcpToolsURL, serviceID string) (udsPath string, shutdown func()) {
	t.Helper()
	mux := http.NewServeMux()
	registerInternalOrchestratorProxyHandlers(mux, embedInternalProxyConfig{
		MCPToolsBaseURL: mcpToolsURL,
		SecureStore:     store,
	})
	dir := t.TempDir()
	udsPath = filepath.Join(dir, "run", "managed_agent_proxy", serviceID, "proxy.sock")
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    mux,
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "",
		SocketByService:    map[string]string{serviceID: udsPath},
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready, err := srv.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("server not ready")
	}
	return udsPath, func() {
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}
}

// TestPerServiceUDS_MCPCallRoundTrip exercises the same path PMA uses: http+unix://…/proxy.sock
// plus POST /v1/worker/internal/orchestrator/mcp:call with the managed-service proxy JSON envelope.
func TestPerServiceUDS_MCPCallRoundTrip(t *testing.T) {
	store := testSecureStore(t)
	if err := store.PutAgentToken("svc-uds", "tok-uds", ""); err != nil {
		t.Fatal(err)
	}
	up := newMCPCallRoundTripUpstream(t)
	defer up.Close()

	oldClient := internalOrchestratorHTTPClient
	internalOrchestratorHTTPClient = up.Client()
	t.Cleanup(func() { internalOrchestratorHTTPClient = oldClient })

	udsPath, shutdown := startUDSInternalOrchestratorTestServer(t, store, up.URL, "svc-uds")
	defer shutdown()

	payload := base64.StdEncoding.EncodeToString([]byte(`{"tool_name":"help.list","arguments":{}}`))
	body := `{"version":1,"method":"POST","path":"/v1/mcp/tools/call","headers":{"Content-Type":["application/json"]},"body_b64":"` + payload + `"}`
	req, err := http.NewRequest(http.MethodPost, "http://unix/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := unixSocketHTTPClient(udsPath).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("proxy HTTP status: %d body=%s", resp.StatusCode, b)
	}
	var proxyResp struct {
		Status  int    `json:"status"`
		BodyB64 string `json:"body_b64"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&proxyResp); err != nil {
		t.Fatal(err)
	}
	if proxyResp.Status != http.StatusOK {
		t.Fatalf("upstream status in envelope: %d", proxyResp.Status)
	}
	raw, err := base64.StdEncoding.DecodeString(proxyResp.BodyB64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"routed":true`)) {
		t.Fatalf("decoded body: %s", raw)
	}
}

// TestProxyAuditLog asserts REQ-WORKER-0163: each proxied request emits a structured JSON audit record
// (timestamp, source, destination, method, path, status code, duration).
func TestProxyAuditLog(t *testing.T) {
	var buf bytes.Buffer
	auditLogger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	store := testSecureStore(t)
	if err := store.PutAgentToken("svc-audit", "tok-audit", ""); err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(up.Close)

	oldClient := internalOrchestratorHTTPClient
	internalOrchestratorHTTPClient = up.Client()
	t.Cleanup(func() { internalOrchestratorHTTPClient = oldClient })

	payload := base64.StdEncoding.EncodeToString([]byte(`{}`))
	body := `{"version":1,"method":"POST","path":"","headers":{"Content-Type":["application/json"]},"body_b64":"` + payload + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), CallerServiceIDContextKey, "svc-audit")
	req = req.WithContext(ctx)

	handleInternalOrchestratorMCPCall(rec, req, embedInternalProxyConfig{
		MCPToolsBaseURL:  up.URL,
		SecureStore:      store,
		ProxyAuditLogger: auditLogger,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("audit JSON: %v raw=%q", err, buf.String())
	}
	required := []string{"timestamp", "source", "destination", "method", "path", "status_code", "duration_ms"}
	for _, k := range required {
		if _, ok := line[k]; !ok {
			t.Errorf("missing audit field %q: %v", k, line)
		}
	}
	if line["source"] != internalProxyAuditSource {
		t.Errorf("source = %v want %q", line["source"], internalProxyAuditSource)
	}
	if line["destination"] == "" || line["method"] == "" || line["path"] == "" {
		t.Errorf("empty routing fields: %v", line)
	}
	if sc, ok := line["status_code"].(float64); !ok || int(sc) != http.StatusOK {
		t.Errorf("status_code = %v want 200", line["status_code"])
	}
}
