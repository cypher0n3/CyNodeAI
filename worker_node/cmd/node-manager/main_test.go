package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestRunMainValidateFails(t *testing.T) {
	_ = os.Setenv("NODE_SLUG", "")
	_ = os.Setenv("ORCHESTRATOR_URL", "http://x")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "psk")
	defer func() {
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
	}()

	ctx := context.Background()
	code := runMain(ctx)
	if code != 1 {
		t.Errorf("runMain should return 1 when config invalid, got %d", code)
	}
}

func TestRunMainContextCancelled(t *testing.T) {
	_ = os.Setenv("NODE_SLUG", "x")
	_ = os.Setenv("ORCHESTRATOR_URL", "http://127.0.0.1:1")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "psk")
	_ = os.Setenv("HTTP_TIMEOUT", "1ms")
	defer func() {
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("HTTP_TIMEOUT")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	code := runMain(ctx)
	if code != 1 {
		t.Errorf("runMain should return 1 when register fails, got %d", code)
	}
}

func TestRunMainSuccess(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/nodes/register" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
				Version:  1,
				IssuedAt: time.Now().UTC().Format(time.RFC3339),
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: srv.URL + "/cap",
						NodeConfigURL: srv.URL + "/cfg",
					},
				},
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	_ = os.Setenv("ORCHESTRATOR_URL", srv.URL)
	_ = os.Setenv("NODE_SLUG", "x")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "psk")
	_ = os.Setenv("CAPABILITY_REPORT_INTERVAL", "1h")
	_ = os.Setenv("HTTP_TIMEOUT", "5s")
	defer func() {
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("CAPABILITY_REPORT_INTERVAL")
		_ = os.Unsetenv("HTTP_TIMEOUT")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	code := runMain(ctx)
	if code != 0 {
		t.Errorf("runMain should return 0 on success, got %d", code)
	}
}
