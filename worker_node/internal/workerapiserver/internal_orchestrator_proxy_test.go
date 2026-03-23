package workerapiserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
