package workerapiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeriveMCPGatewayBaseURL(t *testing.T) {
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
		got := deriveMCPGatewayBaseURL(c.in)
		if got != c.want {
			t.Errorf("deriveMCPGatewayBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHandleInternalOrchestratorMCPCall_RequiresServiceIdentity(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", strings.NewReader(`{}`))
	handleInternalOrchestratorMCPCall(rec, req, embedInternalProxyConfig{
		MCPGatewayBaseURL: "http://127.0.0.1:12082",
		SecureStore:       nil,
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
		MCPGatewayBaseURL: "http://127.0.0.1:12082",
		SecureStore:       nil,
	})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
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
