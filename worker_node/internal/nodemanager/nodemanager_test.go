package nodemanager

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

func TestLoadConfig(t *testing.T) {
	os.Unsetenv("ORCHESTRATOR_URL")
	os.Unsetenv("NODE_SLUG")
	os.Unsetenv("CAPABILITY_REPORT_INTERVAL")
	os.Unsetenv("HTTP_TIMEOUT")
	defer func() {
		os.Unsetenv("ORCHESTRATOR_URL")
		os.Unsetenv("NODE_SLUG")
		os.Unsetenv("CAPABILITY_REPORT_INTERVAL")
		os.Unsetenv("HTTP_TIMEOUT")
	}()

	cfg := LoadConfig()
	if cfg.OrchestratorURL != "http://localhost:8082" || cfg.NodeSlug != "node-01" {
		t.Errorf("defaults: %+v", cfg)
	}
	if cfg.CapabilityReportInterval != 60*time.Second {
		t.Errorf("default interval: %v", cfg.CapabilityReportInterval)
	}

	os.Setenv("ORCHESTRATOR_URL", "http://x")
	os.Setenv("NODE_SLUG", "s")
	os.Setenv("HTTP_TIMEOUT", "2m")
	cfg2 := LoadConfig()
	if cfg2.OrchestratorURL != "http://x" || cfg2.NodeSlug != "s" {
		t.Errorf("env: %+v", cfg2)
	}
	if cfg2.HTTPTimeout != 2*time.Minute {
		t.Errorf("HTTP_TIMEOUT: %v", cfg2.HTTPTimeout)
	}

	os.Setenv("HTTP_TIMEOUT", "invalid")
	cfg3 := LoadConfig()
	if cfg3.HTTPTimeout != 10*time.Second {
		t.Errorf("invalid HTTP_TIMEOUT should use default: %v", cfg3.HTTPTimeout)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		c    Config
		want bool
	}{
		{"valid", Config{OrchestratorURL: "http://x", NodeSlug: "s", RegistrationPSK: "psk"}, false},
		{"missing url", Config{OrchestratorURL: "", NodeSlug: "s", RegistrationPSK: "psk"}, true},
		{"missing slug", Config{OrchestratorURL: "http://x", NodeSlug: "", RegistrationPSK: "psk"}, true},
		{"missing psk", Config{OrchestratorURL: "http://x", NodeSlug: "s", RegistrationPSK: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.want {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.want)
			}
		})
	}
}

func TestValidateBootstrap(t *testing.T) {
	tests := []struct {
		name    string
		b       nodepayloads.BootstrapResponse
		wantErr bool
	}{
		{"valid", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "http://x",
				},
			},
		}, false},
		{"wrong version", nodepayloads.BootstrapResponse{
			Version: 2,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "http://x",
				},
			},
		}, true},
		{"missing node_jwt", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: ""},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "http://x",
				},
			},
		}, true},
		{"missing node_report_url", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "", NodeConfigURL: "http://x",
				},
			},
		}, true},
		{"missing node_config_url", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "",
				},
			},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBootstrap(&tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBootstrap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRun(t *testing.T) {
	reportCalled := false
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
						NodeReportURL: srv.URL + "/v1/nodes/capability",
						NodeConfigURL: srv.URL + "/v1/nodes/config",
					},
				},
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
			return
		}
		if r.URL.Path == "/v1/nodes/capability" {
			reportCalled = true
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "run-test",
		NodeName:                 "Run Test",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: 10 * time.Millisecond,
		HTTPTimeout:              5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := Run(ctx, nil, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reportCalled {
		t.Error("Run should have called reportCapabilities")
	}
}

func TestRunValidateFails(t *testing.T) {
	cfg := &Config{OrchestratorURL: "http://x", NodeSlug: "", RegistrationPSK: "psk"}
	ctx := context.Background()
	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail when config invalid")
	}
}

func TestRunRegisterFails(t *testing.T) {
	cfg := &Config{
		OrchestratorURL: "http://127.0.0.1:1",
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     1 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail when register fails")
	}
}

func TestRunRegisterErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"forbidden"}`))
	}))
	defer server.Close()

	cfg := &Config{
		OrchestratorURL: server.URL,
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     5 * time.Second,
	}
	ctx := context.Background()
	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail on 403")
	}
}

func TestRunRegisterInvalidBootstrap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: ""},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x",
					NodeConfigURL: "http://x",
				},
			},
		})
	}))
	defer server.Close()

	cfg := &Config{
		OrchestratorURL: server.URL,
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     5 * time.Second,
	}
	ctx := context.Background()
	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail when bootstrap missing node_jwt")
	}
}

func TestRunRegisterBadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	cfg := &Config{
		OrchestratorURL: server.URL,
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     5 * time.Second,
	}
	ctx := context.Background()
	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail on invalid JSON")
	}
}

func TestRunReportCapabilitiesErrorBranch(t *testing.T) {
	reportCount := 0
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
						NodeReportURL: srv.URL + "/v1/nodes/capability",
						NodeConfigURL: srv.URL + "/v1/nodes/config",
					},
				},
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
			return
		}
		if r.URL.Path == "/v1/nodes/capability" {
			reportCount++
			if reportCount > 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "x",
		NodeName:                 "x",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: 5 * time.Millisecond,
		HTTPTimeout:              5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = Run(ctx, nil, cfg)
	if reportCount < 2 {
		t.Errorf("expected at least 2 capability reports, got %d", reportCount)
	}
}

func TestRunReportCapabilitiesConnectionFails(t *testing.T) {
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
						NodeReportURL: "http://127.0.0.1:1",
						NodeConfigURL: srv.URL + "/cfg",
					},
				},
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "x",
		NodeName:                 "x",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: 10 * time.Millisecond,
		HTTPTimeout:              1 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := Run(ctx, nil, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunContextCancelledAfterRegister(t *testing.T) {
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
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "j", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "x",
		NodeName:                 "x",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: time.Hour,
		HTTPTimeout:              5 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, nil, cfg)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	err := <-errCh
	if err != nil {
		t.Errorf("Run after cancel should return nil: %v", err)
	}
}
