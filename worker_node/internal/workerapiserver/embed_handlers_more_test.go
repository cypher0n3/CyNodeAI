package workerapiserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
)

func TestBuildMuxesFromEmbedConfig_ReadyzNotReady(t *testing.T) {
	// Executor with non-direct runtime that is not available so Ready() returns (false, reason).
	exec := executor.New("nonexistent-runtime-xyz", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "token", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz status = %d, want 503", w.Code)
	}
	if body := w.Body.String(); body == "" || len(body) < 20 {
		t.Errorf("readyz body too short: %q", body)
	}
}

func TestBuildMuxesFromEmbedConfig_NilLogger(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, internal := buildMuxesFromEmbedConfig(exec, "token", t.TempDir(), nil, nil, embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	if pub == nil || internal == nil {
		t.Fatal("muxes should be non-nil")
	}
}

func TestApplyManagedServicesSocketByService(t *testing.T) {
	dir := t.TempDir()
	out := &embedProxyConfig{
		InternalProxy: embedInternalProxyConfig{
			SocketByService: map[string]string{},
		},
	}
	services := []nodepayloads.ConfigManagedService{
		{
			ServiceID:    "s1",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		},
		{ServiceID: ""},
		{ServiceID: "s2"},
	}
	applyManagedServicesSocketByService(out, dir, services)
	if len(out.InternalProxy.SocketByService) != 1 {
		t.Errorf("expected 1 entry, got %d", len(out.InternalProxy.SocketByService))
	}
	if _, ok := out.InternalProxy.SocketByService["s1"]; !ok {
		t.Error("expected s1")
	}
}

func TestApplyNodeConfigToEmbedProxyConfig_InvalidJSON(t *testing.T) {
	out := &embedProxyConfig{InternalProxy: embedInternalProxyConfig{SocketByService: map[string]string{}}}
	logger := slog.Default()
	applyNodeConfigToEmbedProxyConfig(out, t.TempDir(), "not json", logger)
	if len(out.InternalProxy.SocketByService) != 0 {
		t.Error("should not populate on invalid JSON")
	}
}

func TestApplyNodeConfigToEmbedProxyConfig_EmptyOrchestratorBaseURL(t *testing.T) {
	out := &embedProxyConfig{
		InternalProxy: embedInternalProxyConfig{
			SocketByService: map[string]string{},
			UpstreamBaseURL: "already-set",
		},
	}
	cfg := nodepayloads.NodeConfigurationPayload{Version: 1}
	raw, _ := json.Marshal(cfg)
	applyNodeConfigToEmbedProxyConfig(out, t.TempDir(), string(raw), slog.Default())
	if out.InternalProxy.UpstreamBaseURL != "already-set" {
		t.Errorf("UpstreamBaseURL should be unchanged: %q", out.InternalProxy.UpstreamBaseURL)
	}
}

func embedTestPubMux(t *testing.T, bearer string, socketByService map[string]string) *http.ServeMux {
	t.Helper()
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		bearer, t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{SocketByService: socketByService},
		},
	)
	return pub
}

func TestManagedServiceProxyHTTPHandler_MethodNotAllowed(t *testing.T) {
	pub := embedTestPubMux(t, "token", map[string]string{"svc1": filepath.Join(t.TempDir(), "svc1", "proxy.sock")})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/managed-services/svc1/proxy:http", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_Unauthorized(t *testing.T) {
	pub := embedTestPubMux(t, "token", map[string]string{"svc1": filepath.Join(t.TempDir(), "svc1", "proxy.sock")})
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
}

func TestManagedServiceProxyHTTPHandler_ServiceNotFound(t *testing.T) {
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{SocketByService: map[string]string{}},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/unknown/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_ServiceSocketEmpty(t *testing.T) {
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": ""},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_PathNormalized(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	var upstreamPath string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	go func() { _ = http.Serve(ln, upstream) }()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "api"} // no leading slash
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if upstreamPath != "/api" {
		t.Errorf("upstream path = %q, want /api", upstreamPath)
	}
}

func TestManagedServiceProxyHTTPHandler_InvalidBody(t *testing.T) {
	dir := t.TempDir()
	proxySock := filepath.Join(dir, "svc1", "proxy.sock")
	if err := os.MkdirAll(filepath.Dir(proxySock), 0o700); err != nil {
		t.Fatal(err)
	}
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader([]byte("not json")))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_InvalidBodyB64(t *testing.T) {
	dir := t.TempDir()
	proxySock := filepath.Join(dir, "svc1", "proxy.sock")
	if err := os.MkdirAll(filepath.Dir(proxySock), 0o700); err != nil {
		t.Fatal(err)
	}
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "POST", Path: "/chat", BodyB64: "not-valid-base64!!"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_Success(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "val")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upstream-body"))
	})
	go func() { _ = http.Serve(ln, upstream) }()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{
		Version: 1,
		Method:  "GET",
		Path:    "/",
		Headers: map[string][]string{"Accept": {"application/json"}},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
	}
	var resp managedProxyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("resp.Status = %d", resp.Status)
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.BodyB64)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "upstream-body" {
		t.Errorf("body = %q", decoded)
	}
	if resp.Headers == nil || resp.Headers["X-Custom"] == nil || resp.Headers["X-Custom"][0] != "val" {
		t.Errorf("Headers = %v", resp.Headers)
	}
}

func TestManagedServiceProxyHTTPHandler_SuccessWithPostBody(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("upstream got method %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})
	go func() { _ = http.Serve(ln, upstream) }()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(`{"message":"hello"}`))
	body := managedProxyRequest{
		Version: 1,
		Method:  "POST",
		Path:    "/chat",
		BodyB64: bodyB64,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", w.Code, w.Body.String())
	}
	var resp managedProxyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("resp.Status = %d", resp.Status)
	}
}

func TestManagedServiceProxyHTTPHandler_NoAuthWhenBearerEmpty(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	}()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d (no auth when bearer empty)", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_UpstreamError(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	// No listener on service.sock so client.Do will fail
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502; body %s", w.Code, w.Body.String())
	}
}

// buildTestMux creates a test public mux with a single registered service socket (no listener).
func buildTestMux(t *testing.T, bearerToken, stateDir string) (pub *http.ServeMux, proxySockPath string) {
	t.Helper()
	svcDir := filepath.Join(stateDir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySockPath = filepath.Join(svcDir, "proxy.sock")
	pub, _ = buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		bearerToken, t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySockPath},
			},
		},
	)
	return pub, proxySockPath
}

// buildNoStoreMux creates a test public mux with nil telemetry store and a bearer token of "tok".
func buildNoStoreMux(t *testing.T) *http.ServeMux {
	t.Helper()
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{},
	)
	return pub
}

func TestManagedServiceProxyHTTPHandler_MethodNotAllowed2(t *testing.T) {
	pub, _ := buildTestMux(t, "tok", t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/managed-services/svc1/proxy:http", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_Unauthorized2(t *testing.T) {
	pub, _ := buildTestMux(t, "tok", t.TempDir())
	body := managedProxyRequest{Version: 1, Method: "POST", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_Stream_UpstreamError(t *testing.T) {
	pub, _ := buildTestMux(t, "tok", t.TempDir())
	// body_b64 encodes {"stream":true} to trigger wantsStream
	streamBody := base64.StdEncoding.EncodeToString([]byte(`{"stream":true}`))
	body := managedProxyRequest{Version: 1, Method: "POST", Path: "/chat", BodyB64: streamBody}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	// No socket listener → connection error → 502
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestManagedServiceProxyHTTPHandler_Stream_Success(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	_ = os.MkdirAll(svcDir, 0o700)
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
		}))
	}()
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"tok", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{InternalProxy: embedInternalProxyConfig{SocketByService: map[string]string{"svc1": proxySock}}},
	)
	streamBody := base64.StdEncoding.EncodeToString([]byte(`{"stream":true}`))
	body := managedProxyRequest{Version: 1, Method: "POST", Path: "/chat", BodyB64: streamBody}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestManagedServiceProxyHTTPHandler_Stream_UpstreamErrorStatus(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	_ = os.MkdirAll(svcDir, 0o700)
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	}()
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"tok", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{InternalProxy: embedInternalProxyConfig{SocketByService: map[string]string{"svc1": proxySock}}},
	)
	streamBody := base64.StdEncoding.EncodeToString([]byte(`{"stream":true}`))
	body := managedProxyRequest{Version: 1, Method: "POST", Path: "/chat", BodyB64: streamBody}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 from upstream, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEmbedTelemetryHandlers_NoStore(t *testing.T) {
	pub := buildNoStoreMux(t)
	for _, tc := range []struct {
		name string
		url  string
		auth string
		want int
	}{
		{"container not found with auth", "/v1/worker/telemetry/containers/some-id", "Bearer tok", http.StatusNotFound},
		{"container not found no auth", "/v1/worker/telemetry/containers/some-id", "", http.StatusUnauthorized},
		{"logs missing params", "/v1/worker/telemetry/logs", "Bearer tok", http.StatusBadRequest},
		{"logs with source_kind", "/v1/worker/telemetry/logs?source_kind=container", "Bearer tok", http.StatusOK},
		{"containers list no auth", "/v1/worker/telemetry/containers", "", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, http.NoBody)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			w := httptest.NewRecorder()
			pub.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("expected %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}
