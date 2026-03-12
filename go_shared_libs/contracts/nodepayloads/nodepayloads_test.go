package nodepayloads

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBootstrapResponseJSON(t *testing.T) {
	resp := BootstrapResponse{
		Version:  1,
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
		Orchestrator: BootstrapOrchestrator{
			BaseURL: "https://orchestrator.example.com",
			Endpoints: BootstrapEndpoints{
				WorkerRegistrationURL: "https://orchestrator.example.com/v1/nodes/register",
				NodeReportURL:         "https://orchestrator.example.com/v1/nodes/capability",
				NodeConfigURL:         "https://orchestrator.example.com/v1/nodes/config",
			},
		},
		Auth: BootstrapAuth{
			NodeJWT:   "test-jwt-token",
			ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed BootstrapResponse
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}
	if parsed.Auth.NodeJWT != "test-jwt-token" {
		t.Errorf("expected JWT 'test-jwt-token', got %s", parsed.Auth.NodeJWT)
	}
	if parsed.Orchestrator.BaseURL != "https://orchestrator.example.com" {
		t.Errorf("expected base_url 'https://orchestrator.example.com', got %s", parsed.Orchestrator.BaseURL)
	}
	if parsed.Orchestrator.Endpoints.NodeReportURL != "https://orchestrator.example.com/v1/nodes/capability" {
		t.Errorf("expected node_report_url, got %s", parsed.Orchestrator.Endpoints.NodeReportURL)
	}
	if parsed.Orchestrator.Endpoints.NodeConfigURL != "https://orchestrator.example.com/v1/nodes/config" {
		t.Errorf("expected node_config_url, got %s", parsed.Orchestrator.Endpoints.NodeConfigURL)
	}
}

func TestSupportedBootstrapVersion(t *testing.T) {
	if !SupportedBootstrapVersion(1) {
		t.Error("SupportedBootstrapVersion(1) should be true")
	}
	if SupportedBootstrapVersion(0) || SupportedBootstrapVersion(2) {
		t.Error("SupportedBootstrapVersion(0 or 2) should be false")
	}
}

func TestNodeConfigurationPayloadManagedServicesJSON(t *testing.T) {
	payload := NodeConfigurationPayload{
		Version:       1,
		ConfigVersion: "cfg-1",
		IssuedAt:      time.Now().UTC().Format(time.RFC3339),
		NodeSlug:      "node-01",
		ManagedServices: &ConfigManagedServices{
			Services: []ConfigManagedService{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					Image:       "ghcr.io/cypher0n3/cynode-pma:latest",
					Args:        []string{"--role=project_manager"},
					Healthcheck: &ConfigManagedServiceHealthcheck{
						Path:           "/healthz",
						ExpectedStatus: 200,
					},
					RestartPolicy: "always",
					Role:          "project_manager",
					Inference: &ConfigManagedServiceInference{
						Mode:           "node_local",
						BaseURL:        "http://127.0.0.1:11434",
						DefaultModel:   "qwen3.5:0.8b",
						WarmupRequired: true,
					},
					Orchestrator: &ConfigManagedServiceOrchestrator{
						MCPGatewayProxyURL:    "http://127.0.0.1:12090/v1/worker/internal/orchestrator/mcp:call",
						ReadyCallbackProxyURL: "http://127.0.0.1:12090/v1/worker/internal/orchestrator/agent:ready",
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded NodeConfigurationPayload
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ManagedServices == nil || len(decoded.ManagedServices.Services) != 1 {
		t.Fatalf("expected one managed service, got %#v", decoded.ManagedServices)
	}
	got := decoded.ManagedServices.Services[0]
	if got.ServiceID != "pma-main" || got.ServiceType != "pma" {
		t.Errorf("unexpected service identity: %+v", got)
	}
	if got.Healthcheck == nil || got.Healthcheck.ExpectedStatus != 200 {
		t.Errorf("expected healthcheck expected_status 200, got %+v", got.Healthcheck)
	}
}

func TestCapabilityReportManagedServicesStatusJSON(t *testing.T) {
	report := CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node:       CapabilityNode{NodeSlug: "node-01"},
		Platform:   Platform{OS: "linux", Arch: "amd64"},
		Compute:    Compute{CPUCores: 8, RAMMB: 16384},
		ManagedServices: &ManagedServices{
			Supported: true,
			Features:  []string{"service_containers", "agent_orchestrator_proxy_bidirectional"},
		},
		ManagedServicesStatus: &ManagedServicesStatus{
			Services: []ManagedServiceStatus{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					State:       "ready",
					Endpoints: []string{
						"http://worker.example/v1/worker/managed-services/pma-main/proxy:http",
					},
					ReadyAt:            time.Now().UTC().Format(time.RFC3339),
					ContainerID:        "abc123",
					RestartCount:       2,
					ObservedGeneration: "gen-1",
				},
			},
		},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded CapabilityReport
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ManagedServices == nil || !decoded.ManagedServices.Supported {
		t.Fatalf("managed_services.supported not preserved: %+v", decoded.ManagedServices)
	}
	if decoded.ManagedServicesStatus == nil || len(decoded.ManagedServicesStatus.Services) != 1 {
		t.Fatalf("expected one managed service status: %+v", decoded.ManagedServicesStatus)
	}
	got := decoded.ManagedServicesStatus.Services[0]
	if got.State != "ready" || len(got.Endpoints) != 1 {
		t.Errorf("unexpected service status: %+v", got)
	}
}
