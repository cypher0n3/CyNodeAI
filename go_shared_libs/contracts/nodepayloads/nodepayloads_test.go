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
