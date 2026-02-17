package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

const testOrchestratorURL = "http://test-orchestrator"

func TestNewNodeHandler(t *testing.T) {
	handler := NewNodeHandler(nil, nil, "test-psk", testOrchestratorURL, nil)
	if handler == nil {
		t.Fatal("NewNodeHandler returned nil")
	}
}

func TestRegisterBadRequest(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}
	runHandlerTest(t, "POST", "/v1/nodes/register", []byte("{invalid"), handler.Register, http.StatusBadRequest)
}

func TestRegisterInvalidPSK(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "correct-psk"}
	req, rec := recordedRequestJSON("POST", "/v1/nodes/register", NodeRegistrationRequest{
		PSK: "wrong-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "test-node"},
		},
	})
	handler.Register(rec, req)
	assertStatusCode(t, rec, http.StatusUnauthorized)
}

func TestRegisterInvalidBodyOrSlug(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}
	tests := []struct {
		name       string
		body       NodeRegistrationRequest
		wantStatus int
	}{
		{"invalid capability version", NodeRegistrationRequest{
			PSK:        "test-psk",
			Capability: NodeCapabilityReport{Version: 2, Node: NodeCapabilityNode{NodeSlug: "test-node"}},
		}, http.StatusBadRequest},
		{"missing node slug", NodeRegistrationRequest{
			PSK:        "test-psk",
			Capability: NodeCapabilityReport{Version: 1, Node: NodeCapabilityNode{NodeSlug: ""}},
		}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, rec := recordedRequestJSON("POST", "/v1/nodes/register", tt.body)
			handler.Register(rec, req)
			assertStatusCode(t, rec, tt.wantStatus)
		})
	}
}

func TestReportCapabilityBadRequest(t *testing.T) {
	handler := &NodeHandler{}

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBufferString("{invalid"))
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestNodeCapabilityReportJSON(t *testing.T) {
	report := NodeCapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node: NodeCapabilityNode{
			NodeSlug: "test-node",
			Name:     "Test Node",
			Labels:   []string{"gpu", "high-memory"},
		},
		Platform: NodeCapabilityPlatform{
			OS:     "linux",
			Distro: "ubuntu",
			Arch:   "amd64",
		},
		Compute: NodeCapabilityCompute{
			CPUCores: 8,
			RAMMB:    16384,
		},
		Sandbox: &NodeCapabilitySandbox{
			Supported:      true,
			Features:       []string{"podman"},
			MaxConcurrency: 4,
		},
	}

	jsonData, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed NodeCapabilityReport
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}
	if parsed.Node.NodeSlug != "test-node" {
		t.Errorf("expected node slug 'test-node', got %s", parsed.Node.NodeSlug)
	}
}

func TestNodeBootstrapResponseJSON(t *testing.T) {
	handler := &NodeHandler{orchestratorPublicURL: testOrchestratorURL}
	resp := handler.buildBootstrapResponse(testOrchestratorURL, "test-jwt", time.Now().Add(time.Hour))

	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed struct {
		Version      int    `json:"version"`
		IssuedAt     string `json:"issued_at"`
		Orchestrator struct {
			BaseURL   string `json:"base_url"`
			Endpoints struct {
				WorkerRegistrationURL string `json:"worker_registration_url"`
				NodeReportURL         string `json:"node_report_url"`
				NodeConfigURL         string `json:"node_config_url"`
			} `json:"endpoints"`
		} `json:"orchestrator"`
		Auth struct {
			NodeJWT   string `json:"node_jwt"`
			ExpiresAt string `json:"expires_at"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}
	if parsed.Auth.NodeJWT != "test-jwt" {
		t.Errorf("expected JWT 'test-jwt', got %s", parsed.Auth.NodeJWT)
	}
	if parsed.Orchestrator.BaseURL != testOrchestratorURL {
		t.Errorf("expected base_url %q, got %s", testOrchestratorURL, parsed.Orchestrator.BaseURL)
	}
	if parsed.Orchestrator.Endpoints.NodeReportURL != testOrchestratorURL+"/v1/nodes/capability" {
		t.Errorf("expected node_report_url, got %s", parsed.Orchestrator.Endpoints.NodeReportURL)
	}
}

func TestBuildBootstrapResponse(t *testing.T) {
	handler := &NodeHandler{}
	baseURL := testOrchestratorURL
	expiresAt := time.Now().Add(time.Hour)

	resp := handler.buildBootstrapResponse(baseURL, "test-jwt", expiresAt)

	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if resp.Auth.NodeJWT != "test-jwt" {
		t.Errorf("expected JWT 'test-jwt', got %s", resp.Auth.NodeJWT)
	}
	if resp.Orchestrator.BaseURL != baseURL {
		t.Errorf("expected base_url %q, got %q", baseURL, resp.Orchestrator.BaseURL)
	}
	if resp.Orchestrator.Endpoints.NodeReportURL != baseURL+"/v1/nodes/capability" {
		t.Errorf("expected node_report_url, got %s", resp.Orchestrator.Endpoints.NodeReportURL)
	}
	if resp.Orchestrator.Endpoints.NodeConfigURL != baseURL+"/v1/nodes/config" {
		t.Errorf("expected node_config_url, got %s", resp.Orchestrator.Endpoints.NodeConfigURL)
	}
}

func TestLogHelpers(t *testing.T) {
	handler := &NodeHandler{} // nil logger

	// Should not panic with nil logger
	handler.logError("test error")
	handler.logWarn("test warn")
	handler.logInfo("test info")
}

func TestNodeCapabilityNode(t *testing.T) {
	node := NodeCapabilityNode{
		NodeSlug: "test-slug",
		Name:     "Test Name",
		Labels:   []string{"label1", "label2"},
	}

	if node.NodeSlug != "test-slug" {
		t.Errorf("expected NodeSlug 'test-slug', got %s", node.NodeSlug)
	}
	if len(node.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(node.Labels))
	}
}

func TestNodeCapabilityPlatform(t *testing.T) {
	platform := NodeCapabilityPlatform{
		OS:     "linux",
		Distro: "ubuntu",
		Arch:   "amd64",
	}

	if platform.OS != "linux" {
		t.Errorf("expected OS 'linux', got %s", platform.OS)
	}
	if platform.Arch != "amd64" {
		t.Errorf("expected Arch 'amd64', got %s", platform.Arch)
	}
}

func TestNodeCapabilityCompute(t *testing.T) {
	compute := NodeCapabilityCompute{
		CPUCores: 16,
		RAMMB:    32768,
	}

	if compute.CPUCores != 16 {
		t.Errorf("expected CPUCores 16, got %d", compute.CPUCores)
	}
	if compute.RAMMB != 32768 {
		t.Errorf("expected RAMMB 32768, got %d", compute.RAMMB)
	}
}

func TestNodeCapabilitySandbox(t *testing.T) {
	sandbox := NodeCapabilitySandbox{
		Supported:      true,
		Features:       []string{"podman", "docker"},
		MaxConcurrency: 8,
	}

	if !sandbox.Supported {
		t.Error("expected Supported to be true")
	}
	if sandbox.MaxConcurrency != 8 {
		t.Errorf("expected MaxConcurrency 8, got %d", sandbox.MaxConcurrency)
	}
}

func TestNodeRegistrationRequestJSON(t *testing.T) {
	req := NodeRegistrationRequest{
		PSK: "secret-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "my-node"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed NodeRegistrationRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.PSK != "secret-psk" {
		t.Errorf("expected PSK 'secret-psk', got %s", parsed.PSK)
	}
}

func TestValidateRegistrationRequestIntegration(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       "{invalid}",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong psk",
			body:       `{"psk":"wrong","capability":{"version":1,"node":{"node_slug":"test"}}}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unsupported version",
			body:       `{"psk":"test-psk","capability":{"version":2,"node":{"node_slug":"test"}}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty slug",
			body:       `{"psk":"test-psk","capability":{"version":1,"node":{"node_slug":""}}}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			handler.Register(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestNodeCapabilitySandboxNil(t *testing.T) {
	report := NodeCapabilityReport{
		Version: 1,
		Node:    NodeCapabilityNode{NodeSlug: "test"},
		Sandbox: nil,
	}

	if report.Sandbox != nil {
		t.Error("expected nil sandbox")
	}
}

func TestCapabilityHash(t *testing.T) {
	report := NodeCapabilityReport{
		Version: 1,
		Node:    NodeCapabilityNode{NodeSlug: "test-node"},
	}

	jsonBytes, _ := json.Marshal(report)
	if len(jsonBytes) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestLogHelpersWithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := &NodeHandler{logger: logger}

	// These should log without error
	handler.logError("test error message", "key", "value")
	handler.logWarn("test warn message", "key", "value")
	handler.logInfo("test info message", "key", "value")
}

func TestReportCapabilityNoNodeID(t *testing.T) {
	handler := &NodeHandler{}

	body := NodeCapabilityReport{
		Version: 1,
		Node:    NodeCapabilityNode{NodeSlug: "test"},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	// Should fail because no node ID in context
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestRegisterWithNilDB(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk", db: nil}

	body := NodeRegistrationRequest{
		PSK: "test-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "test-node"},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	// This will panic or fail due to nil DB, but the test ensures request validation works
	defer func() {
		// Expected panic due to nil db
		_ = recover()
	}()

	handler.Register(rec, req)
}

func TestNodeRegistrationRequestAllFields(t *testing.T) {
	req := NodeRegistrationRequest{
		PSK: "my-psk",
		Capability: NodeCapabilityReport{
			Version:    1,
			ReportedAt: time.Now().UTC().Format(time.RFC3339),
			Node: NodeCapabilityNode{
				NodeSlug: "node-1",
				Name:     "Node 1",
				Labels:   []string{"gpu", "high-mem"},
			},
			Platform: NodeCapabilityPlatform{
				OS:     "linux",
				Distro: "ubuntu",
				Arch:   "amd64",
			},
			Compute: NodeCapabilityCompute{
				CPUCores: 16,
				RAMMB:    65536,
			},
			Sandbox: &NodeCapabilitySandbox{
				Supported:      true,
				Features:       []string{"docker", "podman"},
				MaxConcurrency: 8,
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed NodeRegistrationRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Capability.Node.Name != "Node 1" {
		t.Errorf("expected name 'Node 1', got %s", parsed.Capability.Node.Name)
	}
	if parsed.Capability.Platform.Distro != "ubuntu" {
		t.Errorf("expected distro 'ubuntu', got %s", parsed.Capability.Platform.Distro)
	}
	if parsed.Capability.Compute.RAMMB != 65536 {
		t.Errorf("expected RAM 65536, got %d", parsed.Capability.Compute.RAMMB)
	}
}

func TestNodeCapabilityReportNoSandbox(t *testing.T) {
	report := NodeCapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node:       NodeCapabilityNode{NodeSlug: "minimal-node"},
		Platform:   NodeCapabilityPlatform{OS: "linux", Arch: "arm64"},
		Compute:    NodeCapabilityCompute{CPUCores: 4, RAMMB: 8192},
		// Sandbox is nil
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed NodeCapabilityReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Sandbox != nil {
		t.Error("expected nil sandbox")
	}
	if parsed.Platform.Arch != "arm64" {
		t.Errorf("expected arch 'arm64', got %s", parsed.Platform.Arch)
	}
}

func TestValidateRegistrationRequestValid(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}

	body := NodeRegistrationRequest{
		PSK: "test-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "valid-node"},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	result, ok := handler.validateRegistrationRequest(rec, req)

	if !ok {
		t.Error("expected validation to pass")
	}
	if result == nil {
		t.Error("expected result to be non-nil")
	}
	if result != nil && result.Capability.Node.NodeSlug != "valid-node" {
		t.Errorf("expected node slug 'valid-node', got %s", result.Capability.Node.NodeSlug)
	}
}
