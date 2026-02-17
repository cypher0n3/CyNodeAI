package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestValidateBootstrap(t *testing.T) {
	tests := []struct {
		name    string
		b       nodepayloads.BootstrapResponse
		wantErr bool
	}{
		{
			name: "valid",
			b: nodepayloads.BootstrapResponse{
				Version: 1,
				Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: "http://orch/v1/nodes/capability",
						NodeConfigURL: "http://orch/v1/nodes/config",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "wrong version",
			b: nodepayloads.BootstrapResponse{
				Version: 2,
				Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: "http://orch/v1/nodes/capability",
						NodeConfigURL: "http://orch/v1/nodes/config",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing node_jwt",
			b: nodepayloads.BootstrapResponse{
				Version: 1,
				Auth:    nodepayloads.BootstrapAuth{NodeJWT: ""},
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: "http://orch/v1/nodes/capability",
						NodeConfigURL: "http://orch/v1/nodes/config",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing node_report_url",
			b: nodepayloads.BootstrapResponse{
				Version: 1,
				Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: "",
						NodeConfigURL: "http://orch/v1/nodes/config",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing node_config_url",
			b: nodepayloads.BootstrapResponse{
				Version: 1,
				Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: "http://orch/v1/nodes/capability",
						NodeConfigURL: "",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBootstrap(&tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBootstrap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterUsesBootstrapURLs(t *testing.T) {
	// Start a mock server that returns spec-shaped bootstrap with custom node_report_url.
	reportURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/nodes/register" {
			bootstrap := nodepayloads.BootstrapResponse{
				Version:  1,
				IssuedAt: time.Now().UTC().Format(time.RFC3339),
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					BaseURL: "http://custom-orchestrator",
					Endpoints: nodepayloads.BootstrapEndpoints{
						WorkerRegistrationURL: "http://custom-orchestrator/v1/nodes/register",
						NodeReportURL:         "http://custom-orchestrator/v1/nodes/capability",
						NodeConfigURL:         "http://custom-orchestrator/v1/nodes/config",
					},
				},
				Auth: nodepayloads.BootstrapAuth{
					NodeJWT:   "test-jwt",
					ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
				},
			}
			reportURL = bootstrap.Orchestrator.Endpoints.NodeReportURL
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(bootstrap)
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	cfg := config{
		OrchestratorURL:          server.URL,
		NodeSlug:                 "test-node",
		NodeName:                 "Test",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: time.Hour,
		HTTPTimeout:              5 * time.Second,
	}

	bootstrap, err := register(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if bootstrap.NodeReportURL != reportURL {
		t.Errorf("expected NodeReportURL %q from bootstrap, got %q", reportURL, bootstrap.NodeReportURL)
	}
	if bootstrap.NodeReportURL != "http://custom-orchestrator/v1/nodes/capability" {
		t.Errorf("expected node_report_url from bootstrap payload, got %q", bootstrap.NodeReportURL)
	}
}
